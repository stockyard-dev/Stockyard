package main

import (
	"testing"
)

func codes(f []Finding) map[string]bool {
	out := map[string]bool{}
	for _, x := range f {
		out[x.Code] = true
	}
	return out
}

func TestValidManifestPasses(t *testing.T) {
	f := validateFile("testdata/valid.json", nil, false)
	for _, x := range f {
		if x.Severity == "error" {
			t.Errorf("unexpected error on valid fixture: %s %s", x.Code, x.Message)
		}
	}
}

func TestBadManifestFires(t *testing.T) {
	f := validateFile("testdata/bad.json", nil, false)
	got := codes(f)
	want := []string{"E002", "E004", "E010", "E011", "E012", "E013", "W020"}
	for _, w := range want {
		if !got[w] {
			t.Errorf("expected finding %s, missing. got: %v", w, got)
		}
	}
}

func TestCatalogEquality(t *testing.T) {
	cat, err := loadCatalog("testdata/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	// Valid fixture should pass catalog check.
	f := validateFile("testdata/valid.json", cat, true)
	for _, x := range f {
		if x.Severity == "error" {
			t.Errorf("valid fixture failed catalog check: %s %s", x.Code, x.Message)
		}
	}
}

func TestCatalogMissingSlug(t *testing.T) {
	cat, _ := loadCatalog("testdata/catalog.json")
	f := validateFile("testdata/bad.json", cat, true)
	if !codes(f)["E030"] {
		t.Errorf("expected E030 for slug not in catalog, got: %v", codes(f))
	}
}

func TestStrictPromotesWarnings(t *testing.T) {
	f := validateFile("testdata/bad.json", nil, false)
	hasWarn := false
	for _, x := range f {
		if x.Severity == "warning" {
			hasWarn = true
		}
	}
	if !hasWarn {
		t.Fatal("expected at least one warning in bad fixture")
	}
	// Simulate --strict: emit() mutates severity.
	emit(f, true, true)
	for _, x := range f {
		if x.Severity == "warning" {
			t.Errorf("strict did not promote warning: %s", x.Code)
		}
	}
}
