package markdown

import (
	"fmt"
	"html"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"yoroshiku-epub-agent/internal/model"
)

type ImageResolver func(sourcePath, alt string) (href string, err error)

type RenderResult struct {
	Body       string
	Paragraphs int
	Warnings   []string
	Errors     []string
}

var (
	headingPattern = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*#*\s*$`)
	unorderedItem  = regexp.MustCompile(`^[-+*]\s+(.+)$`)
	orderedItem    = regexp.MustCompile(`^\d+[.)]\s+(.+)$`)
	imageOnly      = regexp.MustCompile(`^!\[([^\]]*)\]\(([^)\s]+)(?:\s+["'][^"']*["'])?\)$`)
	inlineImage    = regexp.MustCompile(`!\[([^\]]*)\]\(([^)\s]+)(?:\s+["'][^"']*["'])?\)`)
	inlineLink     = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)(?:\s+["'][^"']*["'])?\)`)
	inlineCode     = regexp.MustCompile("`([^`]+)`")
	inlineStrong   = regexp.MustCompile(`\*\*([^*]+)\*\*|__([^_]+)__`)
	inlineEmphasis = regexp.MustCompile(`\*([^*]+)\*|_([^_]+)_`)
	aozoraRuby     = regexp.MustCompile(`(?:｜|\|)([^《\n]+)《([^》\n]+)》`)
	braceRuby      = regexp.MustCompile(`\{([^{}|]+)\|([^{}|]+)\}`)
	htmlRuby       = regexp.MustCompile(`(?i)<ruby>\s*([^<]+?)\s*<rt>\s*([^<]+?)\s*</rt>\s*</ruby>`)
)

func Render(source, title, sourcePath string, insertions []model.Illustration, resolve ImageResolver) RenderResult {
	lines := normalizeLines(source)
	lines = stripFrontMatter(lines)
	byParagraph := map[int][]model.Illustration{}
	var atEnd []model.Illustration
	for _, insertion := range insertions {
		if insertion.AfterParagraph <= 0 {
			atEnd = append(atEnd, insertion)
		} else {
			byParagraph[insertion.AfterParagraph] = append(byParagraph[insertion.AfterParagraph], insertion)
		}
	}

	var out strings.Builder
	result := RenderResult{}
	hasHeading := false
	paragraph := func(parts []string) {
		if len(parts) == 0 {
			return
		}
		joined := joinSoftLines(parts)
		out.WriteString("<p>")
		out.WriteString(renderInline(joined, filepath.Dir(sourcePath), resolve, &result))
		out.WriteString("</p>\n")
		result.Paragraphs++
		writeInsertions(&out, byParagraph[result.Paragraphs], resolve, &result)
	}

	for i := 0; i < len(lines); {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			i++
			continue
		}
		if m := headingPattern.FindStringSubmatch(line); m != nil {
			level := len(m[1])
			if level == 1 {
				hasHeading = true
			}
			out.WriteString(fmt.Sprintf("<h%d>%s</h%d>\n", level, renderInline(m[2], filepath.Dir(sourcePath), resolve, &result), level))
			i++
			continue
		}
		if line == "---" || line == "***" || line == "___" {
			out.WriteString("<hr/>\n")
			i++
			continue
		}
		if m := imageOnly.FindStringSubmatch(line); m != nil {
			writeImage(&out, m[2], m[1], "", filepath.Dir(sourcePath), resolve, &result)
			i++
			continue
		}
		if strings.HasPrefix(line, ">") {
			var quoted []string
			for i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), ">") {
				quoted = append(quoted, strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[i]), ">")))
				i++
			}
			out.WriteString("<blockquote><p>")
			out.WriteString(renderInline(joinSoftLines(quoted), filepath.Dir(sourcePath), resolve, &result))
			out.WriteString("</p></blockquote>\n")
			result.Paragraphs++
			writeInsertions(&out, byParagraph[result.Paragraphs], resolve, &result)
			continue
		}
		if unorderedItem.MatchString(line) {
			out.WriteString("<ul>\n")
			for i < len(lines) {
				m := unorderedItem.FindStringSubmatch(strings.TrimSpace(lines[i]))
				if m == nil {
					break
				}
				out.WriteString("<li>" + renderInline(m[1], filepath.Dir(sourcePath), resolve, &result) + "</li>\n")
				i++
			}
			out.WriteString("</ul>\n")
			continue
		}
		if orderedItem.MatchString(line) {
			out.WriteString("<ol>\n")
			for i < len(lines) {
				m := orderedItem.FindStringSubmatch(strings.TrimSpace(lines[i]))
				if m == nil {
					break
				}
				out.WriteString("<li>" + renderInline(m[1], filepath.Dir(sourcePath), resolve, &result) + "</li>\n")
				i++
			}
			out.WriteString("</ol>\n")
			continue
		}
		var parts []string
		for i < len(lines) {
			candidate := strings.TrimSpace(lines[i])
			if candidate == "" || headingPattern.MatchString(candidate) || candidate == "---" || candidate == "***" || candidate == "___" || strings.HasPrefix(candidate, ">") || unorderedItem.MatchString(candidate) || orderedItem.MatchString(candidate) || imageOnly.MatchString(candidate) {
				break
			}
			parts = append(parts, strings.TrimRight(lines[i], " \t"))
			i++
		}
		paragraph(parts)
	}
	if !hasHeading && strings.TrimSpace(title) != "" {
		result.Body = "<h1>" + html.EscapeString(title) + "</h1>\n" + out.String()
	} else {
		result.Body = out.String()
	}
	var tail strings.Builder
	writeInsertions(&tail, atEnd, resolve, &result)
	result.Body += tail.String()
	for para, items := range byParagraph {
		if para > result.Paragraphs {
			result.Warnings = append(result.Warnings, fmt.Sprintf("指定段落 %d が本文段落数 %d を超えたため、%d点の画像を章末へ移動", para, result.Paragraphs, len(items)))
			var moved strings.Builder
			writeInsertions(&moved, items, resolve, &result)
			result.Body += moved.String()
		}
	}
	return result
}

func writeInsertions(out *strings.Builder, images []model.Illustration, resolve ImageResolver, result *RenderResult) {
	for _, img := range images {
		writeImage(out, img.Path, img.Alt, img.Caption, "", resolve, result)
	}
}

func writeImage(out *strings.Builder, source, alt, caption, base string, resolve ImageResolver, result *RenderResult) {
	path := source
	if base != "" && !filepath.IsAbs(path) {
		path = filepath.Join(base, filepath.FromSlash(path))
	}
	href, err := resolve(path, alt)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		out.WriteString("<p class=\"missing-image\">[画像を組み込めませんでした: " + html.EscapeString(source) + "]</p>\n")
		return
	}
	out.WriteString("<figure><img src=\"" + html.EscapeString(href) + "\" alt=\"" + html.EscapeString(alt) + "\"/>")
	if caption != "" {
		out.WriteString("<figcaption>" + html.EscapeString(caption) + "</figcaption>")
	}
	out.WriteString("</figure>\n")
}

func renderInline(text, base string, resolve ImageResolver, result *RenderResult) string {
	var placeholders []string
	put := func(value string) string {
		placeholders = append(placeholders, value)
		return fmt.Sprintf("\x00%d\x00", len(placeholders)-1)
	}
	text = htmlRuby.ReplaceAllStringFunc(text, func(value string) string {
		m := htmlRuby.FindStringSubmatch(value)
		return put(ruby(m[1], m[2]))
	})
	text = aozoraRuby.ReplaceAllStringFunc(text, func(value string) string {
		m := aozoraRuby.FindStringSubmatch(value)
		return put(ruby(m[1], m[2]))
	})
	text = braceRuby.ReplaceAllStringFunc(text, func(value string) string {
		m := braceRuby.FindStringSubmatch(value)
		return put(ruby(m[1], m[2]))
	})
	text = inlineImage.ReplaceAllStringFunc(text, func(value string) string {
		m := inlineImage.FindStringSubmatch(value)
		path := m[2]
		if base != "" && !filepath.IsAbs(path) {
			path = filepath.Join(base, filepath.FromSlash(path))
		}
		href, err := resolve(path, m[1])
		if err != nil {
			result.Errors = append(result.Errors, err.Error())
			return put("[画像欠損: " + html.EscapeString(m[2]) + "]")
		}
		return put("<img class=\"inline-image\" src=\"" + html.EscapeString(href) + "\" alt=\"" + html.EscapeString(m[1]) + "\"/>")
	})
	text = inlineCode.ReplaceAllStringFunc(text, func(value string) string {
		m := inlineCode.FindStringSubmatch(value)
		return put("<code>" + html.EscapeString(m[1]) + "</code>")
	})
	text = inlineLink.ReplaceAllStringFunc(text, func(value string) string {
		m := inlineLink.FindStringSubmatch(value)
		return put("<a href=\"" + html.EscapeString(m[2]) + "\">" + html.EscapeString(m[1]) + "</a>")
	})
	text = inlineStrong.ReplaceAllStringFunc(text, func(value string) string {
		m := inlineStrong.FindStringSubmatch(value)
		content := m[1]
		if content == "" {
			content = m[2]
		}
		return put("<strong>" + html.EscapeString(content) + "</strong>")
	})
	text = inlineEmphasis.ReplaceAllStringFunc(text, func(value string) string {
		m := inlineEmphasis.FindStringSubmatch(value)
		content := m[1]
		if content == "" {
			content = m[2]
		}
		return put("<em>" + html.EscapeString(content) + "</em>")
	})
	text = strings.ReplaceAll(text, "  \n", "\x00BR\x00")
	text = html.EscapeString(text)
	text = strings.ReplaceAll(text, "\x00BR\x00", "<br/>")
	for i, value := range placeholders {
		text = strings.ReplaceAll(text, fmt.Sprintf("\x00%d\x00", i), value)
	}
	return text
}

func ruby(base, reading string) string {
	return "<ruby>" + html.EscapeString(strings.TrimSpace(base)) + "<rp>（</rp><rt>" + html.EscapeString(strings.TrimSpace(reading)) + "</rt><rp>）</rp></ruby>"
}

func normalizeLines(source string) []string {
	source = strings.TrimPrefix(source, "\ufeff")
	source = strings.ReplaceAll(source, "\r\n", "\n")
	source = strings.ReplaceAll(source, "\r", "\n")
	return strings.Split(source, "\n")
}

func stripFrontMatter(lines []string) []string {
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return lines
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return lines[i+1:]
		}
	}
	return lines
}

func joinSoftLines(lines []string) string {
	var out strings.Builder
	for i, line := range lines {
		if i > 0 {
			previous := strings.TrimSpace(lines[i-1])
			current := strings.TrimSpace(line)
			if needsSpace(previous, current) {
				out.WriteByte(' ')
			}
		}
		if strings.HasSuffix(line, "  ") {
			out.WriteString(strings.TrimSuffix(line, "  "))
			out.WriteString("  \n")
		} else {
			out.WriteString(strings.TrimSpace(line))
		}
	}
	return out.String()
}

func needsSpace(left, right string) bool {
	if left == "" || right == "" || strings.HasSuffix(left, "  ") {
		return false
	}
	lr, _ := utf8.DecodeLastRuneInString(left)
	rr, _ := utf8.DecodeRuneInString(right)
	return (unicode.IsLetter(lr) || unicode.IsDigit(lr)) && (unicode.IsLetter(rr) || unicode.IsDigit(rr)) && lr < unicode.MaxASCII && rr < unicode.MaxASCII
}

func SortedWarnings(result RenderResult) []string {
	out := append([]string(nil), result.Warnings...)
	sort.Strings(out)
	return out
}
