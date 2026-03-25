package detect

import "testing"

func TestFindLoops(t *testing.T) {
	chains := [][]string{{"a", "b", "c", "a"}, {"x", "y"}}
	loops := FindLoops(chains)
	if len(loops) != 1 { t.Errorf("expected 1 loop, got %d", len(loops)) }
}
