package discover

import (
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

func TestScanFindsFlexibleInputs(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "chapter01.md"), "# はじまり\n\n本文。\n")
	mustWrite(t, filepath.Join(dir, "book-info.md"), `# 作品情報

- タイトル: 星の便り
- サブタイトル: 夜空の物語
- シリーズ名: 星めぐり
- シリーズ番号: 2
- 著者名: 著者A
- 言語: ja
`)
	writeJPEG(t, filepath.Join(dir, "my-cover.jpg"), 100, 160)
	writeJPEG(t, filepath.Join(dir, "illustration-01.jpg"), 100, 80)
	cfg, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Book.Title != "星の便り" || cfg.Book.Subtitle != "夜空の物語" || cfg.Book.Creator != "著者A" || cfg.Book.Series != "星めぐり" || cfg.Book.SeriesIndex != "2" || cfg.Book.Language != "ja" {
		t.Fatalf("metadata=%+v", cfg.Book)
	}
	if cfg.KDP.Description != "" || len(cfg.KDP.Keywords) != 0 || len(cfg.KDP.Categories) != 0 || cfg.KDP.RightsConfirmed != nil || cfg.KDP.AIGeneratedText != "" {
		t.Fatalf("scan should not populate KDP form data: %+v", cfg.KDP)
	}
	if cfg.Cover.Path != "my-cover.jpg" {
		t.Fatalf("cover=%s", cfg.Cover.Path)
	}
	if len(cfg.Sections) != 1 || len(cfg.Sections[0].Images) != 1 {
		t.Fatalf("sections=%+v", cfg.Sections)
	}
}

func TestScanIgnoresStarterReadmes(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "manuscript", "README.md"), "# 案内\n")
	mustWrite(t, filepath.Join(dir, "manuscript", "001.md"), "# 第一章\n\n本文。\n")
	mustWrite(t, filepath.Join(dir, "images", "README.md"), "# 案内\n")
	mustWrite(t, filepath.Join(dir, "book-info.md"), "タイトル: 作品名\n著者名: 著者名\n")
	writeJPEG(t, filepath.Join(dir, "images", "cover.jpg"), 100, 160)
	cfg, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Sections) != 1 || cfg.Sections[0].Path != "manuscript/001.md" {
		t.Fatalf("starter README was treated as manuscript: %+v", cfg.Sections)
	}
}

func mustWrite(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeJPEG(t *testing.T, path string, width, height int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 100, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 80}); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}
