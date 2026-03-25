package pipeline

import "testing"

func TestTrace(t *testing.T) {
	comps := Trace("test-trace")
	if len(comps) == 0 { t.Error("expected components") }
}
