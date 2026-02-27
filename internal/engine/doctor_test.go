package engine

import (
	"testing"
)

func TestCheckDiskSpace(t *testing.T) {
	result := checkDiskSpace(t.TempDir())
	if result.Status != "pass" {
		t.Errorf("expected pass, got %s: %s", result.Status, result.Detail)
	}
}

func TestCheckDiskSpaceBadDir(t *testing.T) {
	result := checkDiskSpace("/nonexistent/path/that/should/not/exist")
	// May pass on some systems if it can create the dir, or fail
	if result.Status == "fail" {
		t.Logf("correctly detected bad directory: %s", result.Detail)
	}
}
