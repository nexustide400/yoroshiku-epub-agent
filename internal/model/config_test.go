package model

import "testing"

func TestApplyDefaultsIdentifierIsStable(t *testing.T) {
	a := Config{Book: Book{Title: "作品", Creator: "著者"}}
	b := Config{Book: Book{Title: "作品", Creator: "著者"}}
	a.ApplyDefaults()
	b.ApplyDefaults()
	if a.Book.Identifier == "" || a.Book.Identifier != b.Book.Identifier {
		t.Fatalf("identifier not stable: %q %q", a.Book.Identifier, b.Book.Identifier)
	}
	if a.Book.Language != "ja" || a.Book.WritingMode != "horizontal-tb" {
		t.Fatalf("defaults=%+v", a.Book)
	}
}
