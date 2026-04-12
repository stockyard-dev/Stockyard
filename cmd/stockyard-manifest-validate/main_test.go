package main

import (
	"os"
	"path/filepath"
	"strings"
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
	emit(f, true, false, true)
	for _, x := range f {
		if x.Severity == "warning" {
			t.Errorf("strict did not promote warning: %s", x.Code)
		}
	}
}

func TestEscapeGitHubCmd(t *testing.T) {
	cases := map[string]string{
		"plain":              "plain",
		"with %":             "with %25",
		"line1\nline2":       "line1%0Aline2",
		"cr\rhere":           "cr%0Dhere",
		"100% done\nand ok":  "100%25 done%0Aand ok",
	}
	for in, want := range cases {
		if got := escapeGitHubCmd(in); got != want {
			t.Errorf("escapeGitHubCmd(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCollectManifests(t *testing.T) {
	dir := t.TempDir()
	// Layout:
	//   dir/stockyard.json                (direct file)
	//   dir/tool-a/stockyard.json         (one level down)
	//   dir/tool-b/stockyard.json
	//   dir/not-a-tool/other.json         (ignored)
	mustWrite := func(p, body string) {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite(filepath.Join(dir, "stockyard.json"), `{"schema_version":"1"}`)
	mustWrite(filepath.Join(dir, "tool-a", "stockyard.json"), `{"schema_version":"1"}`)
	mustWrite(filepath.Join(dir, "tool-b", "stockyard.json"), `{"schema_version":"1"}`)
	mustWrite(filepath.Join(dir, "not-a-tool", "other.json"), `{}`)

	got, err := collectManifests([]string{dir})
	if err != nil {
		t.Fatalf("collectManifests: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 manifests, got %d: %v", len(got), got)
	}
	// Results are sorted; assert the set of basenames-of-parent.
	seen := map[string]bool{}
	for _, p := range got {
		seen[filepath.Base(filepath.Dir(p))] = true
	}
	if !seen[filepath.Base(dir)] || !seen["tool-a"] || !seen["tool-b"] {
		t.Errorf("missing expected manifests, got: %v", got)
	}

	// Explicit file path should also work.
	got2, err := collectManifests([]string{filepath.Join(dir, "tool-a", "stockyard.json")})
	if err != nil || len(got2) != 1 {
		t.Fatalf("explicit path: err=%v got=%v", err, got2)
	}
}

func TestEmitJSON(t *testing.T) {
	// Capture stdout.
	orig := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	findings := []Finding{
		{Code: "E001", Severity: "warning", Path: "x.json", Message: "hi"},
	}
	// strict=true should promote the warning to error before JSON emit.
	hasErr := emit(findings, true, false, true)

	w.Close()
	os.Stdout = orig
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	out := string(buf[:n])

	if !hasErr {
		t.Error("strict promotion did not flip hasErr")
	}
	if !strings.Contains(out, `"code": "E001"`) || !strings.Contains(out, `"severity": "error"`) {
		t.Errorf("unexpected JSON output: %s", out)
	}
}
