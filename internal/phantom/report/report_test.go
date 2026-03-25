package report

import "testing"

func TestDetect(t *testing.T) {
	r := Detect("hello", "goodbye")
	if r == nil { t.Fatal("expected anomaly") }
	if r.Severity != "warning" { t.Errorf("got severity %s", r.Severity) }
	if Detect("same", "same") != nil { t.Error("should not detect anomaly for matching") }
}
