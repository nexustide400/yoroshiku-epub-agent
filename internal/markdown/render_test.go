package markdown

import (
	"strings"
	"testing"

	"yoroshiku-epub-agent/internal/model"
)

func TestRenderJapaneseNovelFeatures(t *testing.T) {
	source := "# 第一章\n\n｜古書店《こしょてん》で**手紙**を読んだ。\n\n![夜の塔](tower.png)\n\n<script>alert(1)</script>"
	result := Render(source, "第一章", "/book/chapter.md", []model.Illustration{{Path: "extra.jpg", Alt: "机の上の鍵"}}, func(sourcePath, alt string) (string, error) {
		if strings.HasSuffix(sourcePath, "tower.png") {
			return "../images/tower.png", nil
		}
		return "../images/extra.jpg", nil
	})
	for _, expected := range []string{
		"<ruby>古書店<rp>（</rp><rt>こしょてん</rt>",
		"<strong>手紙</strong>",
		"alt=\"夜の塔\"",
		"alt=\"机の上の鍵\"",
		"&lt;script&gt;alert(1)&lt;/script&gt;",
	} {
		if !strings.Contains(result.Body, expected) {
			t.Fatalf("出力に %q がありません:\n%s", expected, result.Body)
		}
	}
	if result.Paragraphs != 2 {
		t.Fatalf("段落数=%d, want 2", result.Paragraphs)
	}
}

func TestRenderAddsHeadingWhenMissing(t *testing.T) {
	result := Render("本文です。", "章題", "/book/chapter.md", nil, func(string, string) (string, error) { return "", nil })
	if !strings.HasPrefix(result.Body, "<h1>章題</h1>") {
		t.Fatalf("h1が補われていません: %s", result.Body)
	}
}
