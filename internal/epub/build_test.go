package epub

import (
	"archive/zip"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yoroshiku-epub-agent/internal/model"
)

func TestBuildAndValidate(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "chapter.md"), []byte("# 第一章\n\n｜星《ほし》を見た。\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeTestJPEG(t, filepath.Join(dir, "cover.jpg"), 100, 160)
	cfg := model.Config{
		Version:   1,
		InputDir:  dir,
		OutputDir: filepath.Join(dir, "release"),
		Book:      model.Book{Title: "試験書籍", Creator: "試験著者", Language: "ja", Modified: "2026-09-02T00:00:00Z"},
		Cover:     model.Cover{Path: "cover.jpg", Alt: "試験書籍 表紙"},
		Sections:  []model.Section{{Path: "chapter.md", Title: "第一章"}},
	}
	path, result, err := Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Valid() {
		t.Fatalf("validation errors: %+v", result.Errors)
	}
	coverData, err := os.ReadFile(filepath.Join(dir, "release", "cover.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	if len(coverData) < 18 || coverData[13] != 1 || coverData[14] != 0x01 || coverData[15] != 0x2c || coverData[16] != 0x01 || coverData[17] != 0x2c {
		t.Fatal("cover.jpg does not record 300 PPI JFIF density")
	}
	firstBuild, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	secondPath, secondResult, err := Build(cfg)
	if err != nil || !secondResult.Valid() {
		t.Fatalf("second build: %v %+v", err, secondResult.Errors)
	}
	secondBuild, err := os.ReadFile(secondPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstBuild) != string(secondBuild) {
		t.Fatal("same config and inputs did not produce identical EPUB bytes")
	}
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	if len(zr.File) == 0 || zr.File[0].Name != "mimetype" || zr.File[0].Method != zip.Store {
		t.Fatal("mimetype is not first and stored")
	}
	for _, required := range []string{"META-INF/container.xml", "EPUB/package.opf", "EPUB/nav.xhtml", "EPUB/text/section-001.xhtml", "EPUB/images/cover.jpg"} {
		found := false
		for _, f := range zr.File {
			if f.Name == required {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing %s", required)
		}
	}
}

func TestGeneratedOPFWithMainTitleOnly(t *testing.T) {
	opf, result := buildTitleFixture(t, "")
	expected := "<dc:title id=\"title\">主タイトル</dc:title>\n    <meta refines=\"#title\" property=\"title-type\">main</meta>"
	if !strings.Contains(opf, expected) {
		t.Fatalf("主タイトルのOPF指定が不正です:\n%s", opf)
	}
	if strings.Contains(opf, "id=\"subtitle\"") {
		t.Fatalf("サブタイトル未指定なのにsubtitleが生成されました:\n%s", opf)
	}
	assertPassedFinding(t, result, "OPF-TITLE-MAIN")
}

func TestGeneratedOPFWithMainTitleAndSubtitle(t *testing.T) {
	opf, result := buildTitleFixture(t, "副題")
	mainExpected := "<dc:title id=\"title\">主タイトル</dc:title>\n    <meta refines=\"#title\" property=\"title-type\">main</meta>"
	subtitleExpected := "<dc:title id=\"subtitle\">副題</dc:title>\n    <meta refines=\"#subtitle\" property=\"title-type\">subtitle</meta>"
	if !strings.Contains(opf, mainExpected) || !strings.Contains(opf, subtitleExpected) {
		t.Fatalf("主タイトル／サブタイトルのOPF指定が不正です:\n%s", opf)
	}
	assertPassedFinding(t, result, "OPF-TITLE-MAIN")
}

func TestValidateMainTitleRejectsMissingDuplicateAndBrokenReferences(t *testing.T) {
	tests := []struct {
		name   string
		titles []opfTitle
		metas  []opfMeta
	}{
		{name: "missing main refinement", titles: []opfTitle{{ID: "title", Value: "作品"}}},
		{name: "duplicate main refinements", titles: []opfTitle{{ID: "title", Value: "作品"}}, metas: []opfMeta{{Refines: "#title", Property: "title-type", Value: "main"}, {Refines: "#title", Property: "title-type", Value: "main"}}},
		{name: "broken reference", titles: []opfTitle{{ID: "title", Value: "作品"}}, metas: []opfMeta{{Refines: "#missing", Property: "title-type", Value: "main"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := model.Result{}
			validateMainTitle(packageInfo{Titles: tt.titles, Metas: tt.metas}, &result)
			if result.Valid() {
				t.Fatal("不正な主タイトル指定をValidatorが受理しました")
			}
		})
	}
}

func TestBuildReportsMissingReferencedImageAsError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "chapter.md"), []byte("# 第一章\n\n![存在しない挿絵](missing.jpg)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeTestJPEG(t, filepath.Join(dir, "cover.jpg"), 100, 160)
	cfg := model.Config{
		Version: 1, InputDir: dir, OutputDir: filepath.Join(dir, "release"),
		Book:     model.Book{Title: "画像試験", Creator: "試験著者", Language: "ja", Modified: "2026-09-02T00:00:00Z"},
		Cover:    model.Cover{Path: "cover.jpg", Alt: "表紙"},
		Sections: []model.Section{{Path: "chapter.md", Title: "第一章"}},
	}
	_, result, err := Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if result.Valid() {
		t.Fatal("missing referenced image was not reported as an error")
	}
	found := false
	for _, finding := range result.Errors {
		if finding.Code == "MARKDOWN-IMAGE-MISSING" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing image error not found: %+v", result.Errors)
	}
}

func writeTestJPEG(t *testing.T, path string, width, height int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: 30, G: 60, B: 90, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(f, img, nil); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func buildTitleFixture(t *testing.T, subtitle string) (string, model.Result) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "chapter.md"), []byte("# 第一章\n\n本文。\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeTestJPEG(t, filepath.Join(dir, "cover.jpg"), 100, 160)
	cfg := model.Config{
		Version:   1,
		InputDir:  dir,
		OutputDir: filepath.Join(dir, "release"),
		Book: model.Book{
			Title: "主タイトル", Subtitle: subtitle, Creator: "試験著者", Language: "ja", Modified: "2026-09-03T00:00:00Z",
		},
		Cover:    model.Cover{Path: "cover.jpg", Alt: "主タイトル 表紙"},
		Sections: []model.Section{{Path: "chapter.md", Title: "第一章"}},
	}
	epubPath, result, err := Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Valid() {
		t.Fatalf("生成EPUBがValidatorエラーになりました: %+v", result.Errors)
	}
	return readEPUBEntry(t, epubPath, "EPUB/package.opf"), result
}

func readEPUBEntry(t *testing.T, epubPath, entryName string) string {
	t.Helper()
	zr, err := zip.OpenReader(epubPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	for _, entry := range zr.File {
		if entry.Name != entryName {
			continue
		}
		reader, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(reader)
		_ = reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	t.Fatalf("EPUB entry not found: %s", entryName)
	return ""
}

func assertPassedFinding(t *testing.T, result model.Result, code string) {
	t.Helper()
	for _, finding := range result.Passed {
		if finding.Code == code {
			return
		}
	}
	t.Fatalf("成功項目%sがありません: %+v", code, result.Passed)
}
