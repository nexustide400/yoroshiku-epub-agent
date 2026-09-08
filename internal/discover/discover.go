package discover

import (
	"bufio"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"yoroshiku-epub-agent/internal/model"
)

var (
	numberPattern = regexp.MustCompile(`\d+`)
	imageLink     = regexp.MustCompile(`!\[[^\]]*\]\(([^)\s]+)(?:\s+["'][^"']*["'])?\)`)
)

var ignoredDirs = map[string]bool{
	".git": true, ".github": true, ".kdp-work": true, "bin": true, "cmd": true,
	"docs": true, "internal": true, "release": true, "src": true, "templates": true,
	"testdata": true, "vendor": true,
}

var ignoredMarkdown = map[string]bool{
	"agents.md": true, "changelog.md": true, "contributing.md": true, "license.md": true,
	"readme.md": true, "security.md": true,
}

type imageInfo struct {
	path   string
	width  int
	height int
}

func Scan(input string) (model.Config, error) {
	abs, err := filepath.Abs(input)
	if err != nil {
		return model.Config{}, err
	}
	var markdownFiles, infoFiles []string
	var images []imageInfo
	err = filepath.WalkDir(abs, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != abs && ignoredDirs[strings.ToLower(entry.Name())] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(abs, path)
		if err != nil {
			return err
		}
		ext := strings.ToLower(filepath.Ext(path))
		name := strings.ToLower(entry.Name())
		switch ext {
		case ".md", ".markdown":
			if isInfoName(name) {
				infoFiles = append(infoFiles, rel)
			} else if !ignoredMarkdown[name] {
				markdownFiles = append(markdownFiles, rel)
			}
		case ".jpg", ".jpeg", ".png", ".gif":
			w, h := imageSize(path)
			images = append(images, imageInfo{path: rel, width: w, height: h})
		}
		return nil
	})
	if err != nil {
		return model.Config{}, err
	}
	sortNatural(markdownFiles)
	sortNatural(infoFiles)
	sort.SliceStable(images, func(i, j int) bool { return naturalLess(images[i].path, images[j].path) })
	if len(markdownFiles) == 0 {
		return model.Config{}, fmt.Errorf("原稿候補のMarkdownが見つかりません")
	}

	meta := map[string]string{}
	for _, rel := range infoFiles {
		mergeMetadata(meta, filepath.Join(abs, rel))
	}
	coverIndex := chooseCover(images)
	if coverIndex < 0 {
		return model.Config{}, fmt.Errorf("表紙候補のJPEG/PNG/GIF画像が見つかりません")
	}
	cover := images[coverIndex]
	illustrations := append([]imageInfo{}, images[:coverIndex]...)
	illustrations = append(illustrations, images[coverIndex+1:]...)

	cfg := model.Config{Version: 1, InputDir: abs, OutputDir: filepath.Join(abs, "release")}
	cfg.Book.Title = firstNonEmpty(meta["title"], meta["タイトル"], firstHeading(filepath.Join(abs, markdownFiles[0])), strings.TrimSuffix(filepath.Base(abs), filepath.Ext(abs)))
	cfg.Book.Subtitle = firstNonEmpty(meta["subtitle"], meta["サブタイトル"])
	cfg.Book.Creator = firstNonEmpty(meta["author"], meta["creator"], meta["著者"], meta["著者名"])
	cfg.Book.Series = firstNonEmpty(meta["series"], meta["シリーズ"], meta["シリーズ名"])
	cfg.Book.SeriesIndex = firstNonEmpty(meta["seriesindex"], meta["シリーズ番号"])
	cfg.Book.Language = firstNonEmpty(meta["language"], meta["言語"], "ja")
	cfg.Book.Publisher = firstNonEmpty(meta["publisher"], meta["出版社"], meta["出版者名"])
	cfg.Book.Description = firstNonEmpty(meta["description"], meta["商品説明"], meta["内容紹介"])
	cfg.Book.Rights = firstNonEmpty(meta["rights"], meta["権利"], meta["著作権表記"])
	cfg.Cover = model.Cover{Path: filepath.ToSlash(cover.path), Alt: cfg.Book.Title + " 表紙"}
	cfg.Decisions = append(cfg.Decisions, model.Decision{Kind: "cover", Message: fmt.Sprintf("表紙候補として %s を選択（%d×%d）", filepath.ToSlash(cover.path), cover.width, cover.height), Confidence: coverConfidence(cover)})

	linked := map[string]bool{}
	for _, rel := range markdownFiles {
		title := firstHeading(filepath.Join(abs, rel))
		if title == "" {
			title = humanTitle(rel)
		}
		cfg.Sections = append(cfg.Sections, model.Section{Path: filepath.ToSlash(rel), Title: title, Level: 1})
		for _, p := range linkedImages(filepath.Join(abs, rel)) {
			resolved := filepath.Clean(filepath.Join(filepath.Dir(rel), filepath.FromSlash(p)))
			linked[strings.ToLower(resolved)] = true
		}
	}

	var pending []imageInfo
	for _, img := range illustrations {
		if linked[strings.ToLower(filepath.Clean(img.path))] {
			cfg.Decisions = append(cfg.Decisions, model.Decision{Kind: "illustration", Message: filepath.ToSlash(img.path) + " は原稿内リンクを使用", Confidence: "high"})
			continue
		}
		pending = append(pending, img)
	}
	associateImages(&cfg, pending)

	if cfg.Book.Creator == "" {
		cfg.Warnings = append(cfg.Warnings, "著者名を自動特定できませんでした。Agentが確認または補完してください。")
	}
	if len(infoFiles) == 0 {
		cfg.Warnings = append(cfg.Warnings, "作品情報ファイルがないため、原稿とファイル名から仮設定を作成しました。")
	} else if len(meta) == 0 {
		cfg.Warnings = append(cfg.Warnings, "作品情報ファイルはありますが入力値がありません。分かる項目だけ入力するか、Agentが原稿から補完してください。")
	}
	cfg.ApplyDefaults()
	return cfg, nil
}

func chooseCover(images []imageInfo) int {
	best, score := -1, -1
	for i, img := range images {
		name := strings.ToLower(filepath.Base(img.path))
		s := 0
		if strings.Contains(name, "cover") || strings.Contains(name, "表紙") || strings.Contains(name, "hyoshi") {
			s += 100
		}
		if img.height > img.width && img.width > 0 {
			ratio := float64(img.height) / float64(img.width)
			if ratio >= 1.4 && ratio <= 1.8 {
				s += 20
			}
		}
		if img.width >= 625 && img.height >= 1000 {
			s += 10
		}
		if s > score {
			best, score = i, s
		}
	}
	return best
}

func coverConfidence(img imageInfo) string {
	name := strings.ToLower(filepath.Base(img.path))
	if strings.Contains(name, "cover") || strings.Contains(name, "表紙") || strings.Contains(name, "hyoshi") {
		return "high"
	}
	return "medium"
}

func associateImages(cfg *model.Config, images []imageInfo) {
	used := make([]bool, len(images))
	for sectionIndex := range cfg.Sections {
		sectionNumber := firstNumber(cfg.Sections[sectionIndex].Path)
		if sectionNumber == "" {
			continue
		}
		for imageIndex, img := range images {
			if !used[imageIndex] && firstNumber(img.path) == sectionNumber {
				cfg.Sections[sectionIndex].Images = append(cfg.Sections[sectionIndex].Images, illustration(img))
				used[imageIndex] = true
				cfg.Decisions = append(cfg.Decisions, model.Decision{Kind: "illustration", Message: fmt.Sprintf("番号一致により %s → %s", filepath.ToSlash(img.path), cfg.Sections[sectionIndex].Path), Confidence: "high"})
			}
		}
	}
	var remaining []int
	for i := range images {
		if !used[i] {
			remaining = append(remaining, i)
		}
	}
	if len(cfg.Sections) == 1 {
		for _, i := range remaining {
			cfg.Sections[0].Images = append(cfg.Sections[0].Images, illustration(images[i]))
			cfg.Decisions = append(cfg.Decisions, model.Decision{Kind: "illustration", Message: fmt.Sprintf("単一原稿のため %s → %s", filepath.ToSlash(images[i].path), cfg.Sections[0].Path), Confidence: "medium"})
		}
		return
	}
	if len(remaining) == len(cfg.Sections) {
		for sectionIndex, imageIndex := range remaining {
			cfg.Sections[sectionIndex].Images = append(cfg.Sections[sectionIndex].Images, illustration(images[imageIndex]))
			cfg.Decisions = append(cfg.Decisions, model.Decision{Kind: "illustration", Message: fmt.Sprintf("並び順により %s → %s", filepath.ToSlash(images[imageIndex].path), cfg.Sections[sectionIndex].Path), Confidence: "medium"})
		}
		return
	}
	for _, i := range remaining {
		cfg.Warnings = append(cfg.Warnings, filepath.ToSlash(images[i].path)+" の挿入先を安全に特定できませんでした。")
	}
}

func illustration(img imageInfo) model.Illustration {
	return model.Illustration{Path: filepath.ToSlash(img.path), Alt: humanTitle(img.path), AfterParagraph: 0}
}

func isInfoName(name string) bool {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	return strings.Contains(base, "book-info") || strings.Contains(base, "book_info") || strings.Contains(base, "metadata") || strings.Contains(base, "作品情報") || strings.Contains(base, "書籍情報")
}

func imageSize(path string) (int, int) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	c, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0
	}
	return c.Width, c.Height
}

func mergeMetadata(meta map[string]string, path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(s.Text(), "-"))
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			parts = strings.SplitN(line, "：", 2)
		}
		if len(parts) == 2 {
			key := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(parts[0]), " ", ""))
			value := strings.TrimSpace(parts[1])
			if value != "" {
				meta[key] = value
			}
		}
	}
}

func firstHeading(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if strings.HasPrefix(line, "#") {
			return strings.TrimSpace(strings.TrimLeft(line, "#"))
		}
	}
	return ""
}

func linkedImages(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []string
	for _, m := range imageLink.FindAllStringSubmatch(string(data), -1) {
		if len(m) > 1 && !strings.Contains(m[1], "://") {
			out = append(out, strings.Trim(m[1], "<>"))
		}
	}
	return out
}

func firstNumber(value string) string {
	m := numberPattern.FindString(value)
	if m == "" {
		return ""
	}
	n, err := strconv.Atoi(m)
	if err != nil {
		return m
	}
	return strconv.Itoa(n)
}

func humanTitle(path string) string {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	name = strings.NewReplacer("_", " ", "-", " ").Replace(name)
	return strings.TrimSpace(name)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func sortNatural(values []string) {
	sort.SliceStable(values, func(i, j int) bool { return naturalLess(values[i], values[j]) })
}

func naturalLess(a, b string) bool {
	aa, bb := strings.ToLower(filepath.ToSlash(a)), strings.ToLower(filepath.ToSlash(b))
	ra, rb := []rune(aa), []rune(bb)
	for i, j := 0, 0; i < len(ra) && j < len(rb); {
		if unicode.IsDigit(ra[i]) && unicode.IsDigit(rb[j]) {
			ii, jj := i, j
			for ii < len(ra) && unicode.IsDigit(ra[ii]) {
				ii++
			}
			for jj < len(rb) && unicode.IsDigit(rb[jj]) {
				jj++
			}
			na, _ := strconv.Atoi(string(ra[i:ii]))
			nb, _ := strconv.Atoi(string(rb[j:jj]))
			if na != nb {
				return na < nb
			}
			i, j = ii, jj
			continue
		}
		if ra[i] != rb[j] {
			return ra[i] < rb[j]
		}
		i++
		j++
	}
	return len(ra) < len(rb)
}
