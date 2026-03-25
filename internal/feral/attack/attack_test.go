package attack

import "testing"

func TestGenerate(t *testing.T) {
	p := Generate("injection")
	if p == "" { t.Error("empty payload") }
}

func TestMutate(t *testing.T) {
	result, muts := Mutate("test", 2)
	if result == "test" { t.Error("no mutation applied") }
	if len(muts) != 2 { t.Errorf("expected 2 mutations, got %d", len(muts)) }
}
