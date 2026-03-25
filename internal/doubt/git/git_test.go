package git

import (
	"strings"
	"testing"
	"time"

	"github.com/stockyard-dev/stockyard/internal/doubt/scanner"
)

func TestParseGitLog_Empty(t *testing.T) {
	entries := parseGitLog("")
	if len(entries) != 0 {
		t.Errorf("parseGitLog empty = %d entries, want 0", len(entries))
	}
}

func TestParseGitLog_SingleEntry(t *testing.T) {
	output := `abc1234567890|John Doe|2025-01-15T10:30:00Z
diff --git a/file.go b/file.go
--- a/file.go
+++ b/file.go
@@ -10,5 +10,6 @@ func handleHealth(
+    new line
`
	entries := parseGitLog(output)
	if len(entries) != 1 {
		t.Fatalf("parseGitLog = %d entries, want 1", len(entries))
	}

	e := entries[0]
	if e.Hash != "abc1234567890" {
		t.Errorf("Hash = %q, want %q", e.Hash, "abc1234567890")
	}
	if e.Author != "John Doe" {
		t.Errorf("Author = %q, want %q", e.Author, "John Doe")
	}
	if e.Date.IsZero() {
		t.Error("Date should not be zero")
	}
	if e.Patch == "" {
		t.Error("Patch should not be empty")
	}
}

func TestParseGitLog_MultipleEntries(t *testing.T) {
	output := `abc1234567890|Alice|2025-01-15T10:30:00Z
diff line 1
diff line 2
def7890123456|Bob|2025-01-14T09:00:00Z
diff line 3
`
	entries := parseGitLog(output)
	if len(entries) != 2 {
		t.Fatalf("parseGitLog = %d entries, want 2", len(entries))
	}

	if entries[0].Author != "Alice" {
		t.Errorf("entries[0].Author = %q, want %q", entries[0].Author, "Alice")
	}
	if entries[1].Author != "Bob" {
		t.Errorf("entries[1].Author = %q, want %q", entries[1].Author, "Bob")
	}
}

func TestPatchTouchesUnit_EmptyPatch(t *testing.T) {
	unit := scanner.Unit{Name: "Hello"}
	if patchTouchesUnit("", unit) {
		t.Error("empty patch should not touch unit")
	}
}

func TestPatchTouchesUnit_HunkHeader(t *testing.T) {
	unit := scanner.Unit{Name: "handleHealth"}
	patch := `@@ -10,5 +10,6 @@ func handleHealth(
+    new line
-    old line`

	if !patchTouchesUnit(patch, unit) {
		t.Error("patch with matching hunk header should touch unit")
	}
}

func TestPatchTouchesUnit_MethodName(t *testing.T) {
	// For methods like "*Server.handleHealth", should match just "handleHealth"
	unit := scanner.Unit{Name: "*Server.handleHealth"}
	patch := `@@ -10,5 +10,6 @@ func handleHealth(
+    new line`

	if !patchTouchesUnit(patch, unit) {
		t.Error("patch should match method name after dot")
	}
}

func TestPatchTouchesUnit_FuncDeclaration(t *testing.T) {
	unit := scanner.Unit{Name: "ProcessData"}
	patch := `some context
+func ProcessData(input string) {
some more context`

	if !patchTouchesUnit(patch, unit) {
		t.Error("patch with func declaration should touch unit")
	}
}

func TestPatchTouchesUnit_NoMatch(t *testing.T) {
	unit := scanner.Unit{Name: "UnrelatedFunc"}
	patch := `@@ -10,5 +10,6 @@ func handleHealth(
+    new line
-    old line`

	if patchTouchesUnit(patch, unit) {
		t.Error("patch should not touch unrelated unit")
	}
}

func TestPatchTouchesUnit_RemovedFunc(t *testing.T) {
	unit := scanner.Unit{Name: "OldFunc"}
	patch := `some context
-func OldFunc() {
some more context`

	if !patchTouchesUnit(patch, unit) {
		t.Error("removed func line should touch unit")
	}
}

func TestStabilityScore_NoData(t *testing.T) {
	info := StabilityInfo{ChangeCount: 0}
	score := StabilityScore(info)
	if score != 0.5 {
		t.Errorf("StabilityScore (no data) = %v, want 0.5", score)
	}
}

func TestStabilityScore_OldStableCode(t *testing.T) {
	info := StabilityInfo{
		ChangeCount: 5,
		AgeDays:     180, // well past 90 day saturation
		ChurnRate:   0.5, // 0.5 changes per 30 days -> low churn
		Authors:     []string{"Alice"},
	}
	score := StabilityScore(info)

	// ageFactor = 1.0 (saturated), churnFactor = 1/(1+(0.5-1)*0.3) = 1/0.85 -> capped at 1
	// authorFactor = 1.0
	// score = 1.0*0.4 + churn*0.4 + 1.0*0.2
	if score < 0.8 {
		t.Errorf("StabilityScore (old stable) = %v, want >= 0.8", score)
	}
}

func TestStabilityScore_ManyAuthors(t *testing.T) {
	info := StabilityInfo{
		ChangeCount: 10,
		AgeDays:     90,
		ChurnRate:   1.0,
		Authors:     []string{"Alice", "Bob", "Charlie", "Dave"}, // >3 authors
	}
	score := StabilityScore(info)

	// authorFactor = 0.9 for >3 authors
	// This should be slightly lower than with single author
	singleAuthor := StabilityInfo{
		ChangeCount: 10,
		AgeDays:     90,
		ChurnRate:   1.0,
		Authors:     []string{"Alice"},
	}
	singleScore := StabilityScore(singleAuthor)

	if score >= singleScore {
		t.Errorf("Many authors score (%v) should be less than single author (%v)", score, singleScore)
	}
}

func TestStabilityScore_HighChurn(t *testing.T) {
	info := StabilityInfo{
		ChangeCount: 30,
		AgeDays:     90,
		ChurnRate:   10.0, // very high churn
		Authors:     []string{"Alice"},
	}
	score := StabilityScore(info)

	low := StabilityInfo{
		ChangeCount: 3,
		AgeDays:     90,
		ChurnRate:   1.0,
		Authors:     []string{"Alice"},
	}
	lowScore := StabilityScore(low)

	if score >= lowScore {
		t.Errorf("High churn score (%v) should be less than low churn (%v)", score, lowScore)
	}
}

func TestFreshnessScore_ZeroTime(t *testing.T) {
	info := StabilityInfo{}
	score := FreshnessScore(info)
	if score != 0.5 {
		t.Errorf("FreshnessScore (zero time) = %v, want 0.5", score)
	}
}

func TestFreshnessScore_RecentlyModified(t *testing.T) {
	info := StabilityInfo{
		LastModified: time.Now().Add(-1 * time.Hour), // 1 hour ago
	}
	score := FreshnessScore(info)
	if score < 0.95 {
		t.Errorf("FreshnessScore (1h ago) = %v, want > 0.95", score)
	}
}

func TestFreshnessScore_OldCode(t *testing.T) {
	info := StabilityInfo{
		LastModified: time.Now().Add(-100 * 24 * time.Hour), // 100 days ago
	}
	score := FreshnessScore(info)
	if score != 0.1 {
		t.Errorf("FreshnessScore (100 days) = %v, want 0.1", score)
	}
}

func TestAnalyzeUnit_NoFileLog(t *testing.T) {
	unit := scanner.Unit{ID: "test:unit", Name: "TestFunc"}
	info := analyzeUnit(unit, nil, "test.go", "/repo")
	if info.ChangeCount != 0 {
		t.Errorf("ChangeCount = %d, want 0 for empty log", info.ChangeCount)
	}
}

func TestAnalyzeUnit_WithMatches(t *testing.T) {
	now := time.Now()
	fileLog := []logEntry{
		{
			Hash:   "abc123",
			Author: "Alice",
			Date:   now.Add(-24 * time.Hour),
			Patch:  "@@ -10,5 +10,6 @@ func MyFunc(\n+    new line",
		},
		{
			Hash:   "def456",
			Author: "Bob",
			Date:   now.Add(-48 * time.Hour),
			Patch:  "@@ -20,5 +20,6 @@ func OtherFunc(\n+    change",
		},
	}

	unit := scanner.Unit{ID: "test:MyFunc", Name: "MyFunc"}
	info := analyzeUnit(unit, fileLog, "test.go", "/repo")

	if info.ChangeCount != 1 {
		t.Errorf("ChangeCount = %d, want 1", info.ChangeCount)
	}
	if len(info.Authors) != 1 || info.Authors[0] != "Alice" {
		t.Errorf("Authors = %v, want [Alice]", info.Authors)
	}
}

func TestAnalyzeUnit_NoTouches_FallbackToFile(t *testing.T) {
	now := time.Now()
	fileLog := []logEntry{
		{
			Hash:   "abc123",
			Author: "Alice",
			Date:   now.Add(-24 * time.Hour),
			Patch:  "some unrelated patch",
		},
	}

	unit := scanner.Unit{ID: "test:MyFunc", Name: "MyFunc"}
	info := analyzeUnit(unit, fileLog, "test.go", "/repo")

	// Falls back to file-level data
	if info.ChangeCount != 1 {
		t.Errorf("ChangeCount = %d, want 1 (file fallback)", info.ChangeCount)
	}
	if len(info.Authors) != 1 {
		t.Errorf("Authors count = %d, want 1 (file fallback)", len(info.Authors))
	}
}

func TestIsGitRepo(t *testing.T) {
	// The Stockyard repo itself should be a git repo
	if !isGitRepo("/home/user/Stockyard") {
		t.Skip("Stockyard is not a git repo in this environment")
	}

	// A temp dir should not be
	dir := t.TempDir()
	if isGitRepo(dir) {
		t.Errorf("%s should not be a git repo", dir)
	}
}

func TestParseGitLog_HandlesPipeInContent(t *testing.T) {
	// Lines that have pipes but don't match the hash|author|date format should be treated as patch
	output := `abc1234567890|Alice|2025-01-15T10:30:00Z
this line has | pipes | in it
+    added | pipe line
`
	entries := parseGitLog(output)
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if !strings.Contains(entries[0].Patch, "pipes") {
		t.Error("pipe lines should be included in patch")
	}
}
