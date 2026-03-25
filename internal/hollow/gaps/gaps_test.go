package gaps

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyzeGoFile_ErrorIgnored(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		wantGap bool
		gapType string
	}{
		{
			name:    "ignored error with underscore",
			code:    "package main\n\nfunc f() {\n\tresult, _ := doSomething(err)\n}\n",
			wantGap: true,
			gapType: "missing_error_handling",
		},
		{
			name:    "properly handled error",
			code:    "package main\n\nfunc f() {\n\tresult, err := doSomething()\n\tif err != nil { return }\n}\n",
			wantGap: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "test.go")
			if err := os.WriteFile(path, []byte(tt.code), 0644); err != nil {
				t.Fatal(err)
			}
			gaps := analyzeGoFile(path, "test.go")
			found := false
			for _, g := range gaps {
				if g.Type == tt.gapType {
					found = true
					break
				}
			}
			if tt.wantGap && !found {
				t.Errorf("expected gap of type %q, got none (gaps: %v)", tt.gapType, gaps)
			}
			if !tt.wantGap && found {
				t.Errorf("did not expect gap of type %q, but found one", tt.gapType)
			}
		})
	}
}

func TestAnalyzeGoFile_SQLWithoutErrorHandling(t *testing.T) {
	code := "package main\n\nfunc f() {\n\tdb.Query(\"SELECT 1\")\n}\n"
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")
	if err := os.WriteFile(path, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}
	gaps := analyzeGoFile(path, "test.go")
	found := false
	for _, g := range gaps {
		if g.Type == "missing_error_handling" && g.Description == "Database operation without visible error handling" {
			found = true
		}
	}
	if !found {
		t.Error("expected SQL gap, got none")
	}
}

func TestAnalyzeGoFile_HTTPClientNoTimeout(t *testing.T) {
	code := "package main\n\nimport \"net/http\"\n\nfunc f() {\n\thttp.Get(\"http://example.com\")\n}\n"
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")
	if err := os.WriteFile(path, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}
	gaps := analyzeGoFile(path, "test.go")
	found := false
	for _, g := range gaps {
		if g.Type == "no_timeout" {
			found = true
		}
	}
	if !found {
		t.Error("expected no_timeout gap for http.Get without timeout")
	}
}

func TestAnalyzeGoFile_HTTPClientWithTimeout(t *testing.T) {
	// If context.WithTimeout is present, no gap should be reported
	code := "package main\n\nimport \"net/http\"\n\nfunc f() {\n\tctx, cancel := context.WithTimeout(ctx, 5*time.Second)\n\tdefer cancel()\n\thttp.Get(\"http://example.com\")\n}\n"
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")
	if err := os.WriteFile(path, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}
	gaps := analyzeGoFile(path, "test.go")
	for _, g := range gaps {
		if g.Type == "no_timeout" {
			t.Error("should not flag no_timeout when context.WithTimeout is present")
		}
	}
}

func TestAnalyzeGoFile_HardcodedSecret(t *testing.T) {
	code := `package main
var x = "password" = "hunter2"
`
	// The regex looks for "password" : "value" or "password" = "value"
	code2 := "package main\nvar cfg = map[string]string{\"password\": \"hunter2\"}\n"
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")
	if err := os.WriteFile(path, []byte(code2), 0644); err != nil {
		t.Fatal(err)
	}
	gaps := analyzeGoFile(path, "test.go")
	found := false
	for _, g := range gaps {
		if g.Type == "missing_auth" && g.Severity == "critical" {
			found = true
			if g.Evidence != "[redacted — contains potential secret]" {
				t.Errorf("secret evidence should be redacted, got %q", g.Evidence)
			}
		}
	}
	if !found {
		t.Error("expected hardcoded secret gap")
	}
	_ = code // unused
}

func TestAnalyzeGoFile_PanicInProduction(t *testing.T) {
	tests := []struct {
		name    string
		file    string // relPath
		code    string
		wantGap bool
	}{
		{
			name:    "panic in production code",
			file:    "server.go",
			code:    "package main\n\nfunc f() {\n\tpanic(\"boom\")\n}\n",
			wantGap: true,
		},
		{
			name:    "panic in test code is OK",
			file:    "server_test.go",
			code:    "package main\n\nfunc f() {\n\tpanic(\"boom\")\n}\n",
			wantGap: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, tt.file)
			if err := os.WriteFile(path, []byte(tt.code), 0644); err != nil {
				t.Fatal(err)
			}
			gaps := analyzeGoFile(path, tt.file)
			found := false
			for _, g := range gaps {
				if g.Type == "missing_error_handling" && g.Description == "Panic in production code — will crash the process" {
					found = true
				}
			}
			if tt.wantGap && !found {
				t.Error("expected panic gap")
			}
			if !tt.wantGap && found {
				t.Error("panic in test file should not be flagged")
			}
		})
	}
}

func TestAnalyzeGoFile_TODO(t *testing.T) {
	code := "package main\n\n// TODO: fix this later\nfunc f() {}\n"
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")
	if err := os.WriteFile(path, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}
	gaps := analyzeGoFile(path, "test.go")
	found := false
	for _, g := range gaps {
		if g.Type == "uncovered_case" && g.Severity == "low" {
			found = true
		}
	}
	if !found {
		t.Error("expected TODO gap")
	}
}

func TestAnalyzeGoFile_JSONDecodeWithoutValidation(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		wantGap bool
	}{
		{
			name:    "json decode without validation",
			code:    "package main\n\nfunc f() {\n\tjson.Unmarshal(data, &v)\n\treturn v\n}\n",
			wantGap: true,
		},
		{
			name:    "json decode with validation nearby",
			code:    "package main\n\nfunc f() {\n\tjson.Unmarshal(data, &v)\n\tif v.Name == \"\" {\n\t\treturn err\n\t}\n}\n",
			wantGap: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "test.go")
			if err := os.WriteFile(path, []byte(tt.code), 0644); err != nil {
				t.Fatal(err)
			}
			gaps := analyzeGoFile(path, "test.go")
			found := false
			for _, g := range gaps {
				if g.Type == "no_validation" {
					found = true
				}
			}
			if tt.wantGap && !found {
				t.Errorf("expected no_validation gap")
			}
			if !tt.wantGap && found {
				t.Errorf("did not expect no_validation gap when validation exists nearby")
			}
		})
	}
}

func TestAnalyzeGoFile_HTTPHandlerNoRateLimit(t *testing.T) {
	code := "package main\n\nimport \"net/http\"\n\nfunc handler(w http.ResponseWriter, r *http.Request) {\n\tw.Write([]byte(\"ok\"))\n}\n"
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")
	if err := os.WriteFile(path, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}
	gaps := analyzeGoFile(path, "test.go")
	found := false
	for _, g := range gaps {
		if g.Type == "no_rate_limit" {
			found = true
		}
	}
	if !found {
		t.Error("expected no_rate_limit gap for HTTP handler without rate limiting")
	}
}

func TestAnalyzePythonFile(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		wantGap string // expected gap type substring in Description or Type
	}{
		{
			name:    "bare except",
			code:    "try:\n    pass\nexcept:\n    pass\n",
			wantGap: "missing_error_handling",
		},
		{
			name:    "silent except with pass",
			code:    "try:\n    x()\nexcept ValueError:\n    pass\n",
			wantGap: "missing_error_handling",
		},
		{
			name:    "python TODO",
			code:    "# TODO: implement this\ndef f():\n    pass\n",
			wantGap: "uncovered_case",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "test.py")
			if err := os.WriteFile(path, []byte(tt.code), 0644); err != nil {
				t.Fatal(err)
			}
			gaps := analyzePythonFile(path, "test.py")
			found := false
			for _, g := range gaps {
				if g.Type == tt.wantGap {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected gap type %q, got gaps: %v", tt.wantGap, gaps)
			}
		})
	}
}

func TestAnalyzeTSFile(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		wantGap string
	}{
		{
			name:    "empty catch block",
			code:    "try {\n  doSomething();\n} catch(e) {\n}\n",
			wantGap: "missing_error_handling",
		},
		{
			name:    "console.log in production",
			code:    "function f() {\n  console.log(\"debug\");\n}\n",
			wantGap: "uncovered_case",
		},
		{
			name:    "ts TODO",
			code:    "// TODO: fix this\nconst x = 1;\n",
			wantGap: "uncovered_case",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "test.ts")
			if err := os.WriteFile(path, []byte(tt.code), 0644); err != nil {
				t.Fatal(err)
			}
			gaps := analyzeTSFile(path, "test.ts")
			found := false
			for _, g := range gaps {
				if g.Type == tt.wantGap {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected gap type %q, got gaps: %v", tt.wantGap, gaps)
			}
		})
	}
}

func TestAnalyze_Integration(t *testing.T) {
	dir := t.TempDir()

	// Create a Go file with issues
	goCode := "package main\n\nfunc f() {\n\tpanic(\"crash\")\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(goCode), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a Python file with issues
	pyCode := "try:\n    x()\nexcept:\n    pass\n"
	if err := os.WriteFile(filepath.Join(dir, "app.py"), []byte(pyCode), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := Analyze(dir)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if result.Files != 2 {
		t.Errorf("expected 2 files scanned, got %d", result.Files)
	}
	if len(result.Gaps) == 0 {
		t.Error("expected gaps to be found")
	}

	// Check that BySeverity/ByCategory/ByType are populated
	totalBySeverity := 0
	for _, c := range result.BySeverity {
		totalBySeverity += c
	}
	if totalBySeverity != len(result.Gaps) {
		t.Errorf("BySeverity total %d != gap count %d", totalBySeverity, len(result.Gaps))
	}

	totalByCategory := 0
	for _, c := range result.ByCategory {
		totalByCategory += c
	}
	if totalByCategory != len(result.Gaps) {
		t.Errorf("ByCategory total %d != gap count %d", totalByCategory, len(result.Gaps))
	}
}

func TestAnalyze_SkipsVendorAndGit(t *testing.T) {
	dir := t.TempDir()

	// Create vendor directory with a Go file
	vendorDir := filepath.Join(dir, "vendor")
	if err := os.MkdirAll(vendorDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vendorDir, "lib.go"), []byte("package lib\nfunc f() { panic(\"x\") }\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create .git directory with a file
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "config.go"), []byte("package git\nfunc f() { panic(\"x\") }\n"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := Analyze(dir)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if result.Files != 0 {
		t.Errorf("expected 0 files scanned (vendor/.git skipped), got %d", result.Files)
	}
}

func TestAnalyze_ListenAndServeGracefulShutdown(t *testing.T) {
	code := "package main\n\nimport \"net/http\"\n\nfunc main() {\n\thttp.ListenAndServe(\":8080\", nil)\n}\n"
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(code), 0644); err != nil {
		t.Fatal(err)
	}
	gaps := analyzeGoFile(filepath.Join(dir, "main.go"), "main.go")
	found := false
	for _, g := range gaps {
		if g.Type == "no_timeout" && g.Description == "HTTP server without graceful shutdown" {
			found = true
		}
	}
	if !found {
		t.Error("expected graceful shutdown gap for http.ListenAndServe")
	}
}
