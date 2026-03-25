package fingerprint

import "testing"

func TestSignature(t *testing.T) {
	f := &Fingerprint{Model: "gpt-4", Provider: "openai", SampleCount: 100, AvgResponseLength: 150.5}
	sig := f.Signature()
	if len(sig) != 32 { t.Errorf("expected 32 char signature, got %d", len(sig)) }
	sig2 := f.Signature()
	if sig != sig2 { t.Error("signature not deterministic") }
}
