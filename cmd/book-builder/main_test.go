package main

import (
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanBuildValidateWorkflow(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manuscript.md"), []byte("# 第一章\n\n本文です。\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "book-info.md"), []byte("タイトル: CLI試験\n著者名: 試験著者\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeCover(t, filepath.Join(dir, "cover.jpg"))
	config := filepath.Join(dir, ".kdp-work", "build-config.json")
	if code := runScan([]string{"--input", dir, "--config", config}); code != 0 {
		t.Fatalf("scan exit=%d", code)
	}
	if code := runBuild([]string{"--config", config}); code != 0 {
		t.Fatalf("build exit=%d", code)
	}
	release := filepath.Join(dir, "release")
	for _, name := range []string{"book.epub", "cover.jpg", "kdp-metadata.md", "description.txt", "keywords.txt", "categories.md", "validation-report.md", "publishing-checklist.md", "build-config.json"} {
		if stat, err := os.Stat(filepath.Join(release, name)); err != nil || stat.Size() == 0 {
			t.Errorf("missing/empty %s: %v", name, err)
		}
	}
	validation, err := os.ReadFile(filepath.Join(release, "validation-report.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, heading := range []string{"正常に処理できた項目", "自動修正した項目", "警告", "ユーザー確認が必要な項目", "エラー"} {
		if !strings.Contains(string(validation), heading) {
			t.Errorf("validation report missing %s", heading)
		}
	}
	if code := runValidate([]string{filepath.Join(release, "book.epub"), "--report", filepath.Join(release, "validation-second-pass.md")}); code != 0 {
		t.Fatalf("validate exit=%d", code)
	}
}

func writeCover(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 100, 160))
	for y := 0; y < 160; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{R: 20, G: 50, B: 80, A: 255})
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
