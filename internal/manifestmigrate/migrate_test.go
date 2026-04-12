package manifestmigrate

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestMigrate_AlreadyAtCurrent_IsNoOp(t *testing.T) {
	defer resetForTest()
	resetForTest()
	in := []byte(`{"schema_version":"1","tool":{"slug":"x"}}`)
	out, res, err := Migrate(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(in, out) {
		t.Errorf("no-op should return input byte-identical, got %q", out)
	}
	if res.Changed {
		t.Error("Changed=true on no-op")
	}
	if len(res.Applied) != 0 {
		t.Errorf("Applied should be empty, got %v", res.Applied)
	}
}

func TestMigrate_MissingSchemaVersion_TreatedAsOne(t *testing.T) {
	defer resetForTest()
	resetForTest()
	in := []byte(`{"tool":{"slug":"x"}}`)
	_, res, err := Migrate(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.From != "1" {
		t.Errorf("From=%q, want 1", res.From)
	}
}

func TestMigrate_WalkChain(t *testing.T) {
	defer resetForTest()
	resetForTest()
	// Pretend Current is "1". Register fake steps going the other
	// direction to prove the walker works, by overriding via a temp
	// chain that ends at Current.
	//
	// Strategy: register "0" -> "1". Input schema_version "0" should
	// walk one step and end at Current.
	Register("0", "1", func(m map[string]any) error {
		m["added_by_v0_to_v1"] = true
		return nil
	})

	in := []byte(`{"schema_version":"0","tool":{"slug":"x"}}`)
	out, res, err := Migrate(in)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if !res.Changed {
		t.Error("Changed should be true after a migration step")
	}
	if res.From != "0" || res.To != "1" {
		t.Errorf("From/To = %q/%q, want 0/1", res.From, res.To)
	}
	if len(res.Applied) != 1 || res.Applied[0] != [2]string{"0", "1"} {
		t.Errorf("Applied = %v", res.Applied)
	}

	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if got["schema_version"] != "1" {
		t.Errorf("output schema_version = %v, want 1", got["schema_version"])
	}
	if got["added_by_v0_to_v1"] != true {
		t.Error("step function did not mutate")
	}
}

func TestMigrate_UnknownVersion_Errors(t *testing.T) {
	defer resetForTest()
	resetForTest()
	in := []byte(`{"schema_version":"999"}`)
	_, _, err := Migrate(in)
	if !errors.Is(err, ErrUnknownVersion) {
		t.Errorf("expected ErrUnknownVersion, got %v", err)
	}
}

func TestMigrate_StepFuncError_PropagatesWithContext(t *testing.T) {
	defer resetForTest()
	resetForTest()
	Register("0", "1", func(m map[string]any) error {
		return errors.New("step exploded")
	})
	_, _, err := Migrate([]byte(`{"schema_version":"0"}`))
	if err == nil || !strings.Contains(err.Error(), "0 -> 1") {
		t.Errorf("expected wrapped error with edge context, got %v", err)
	}
}

func TestRegister_RejectsDuplicateFrom(t *testing.T) {
	defer resetForTest()
	resetForTest()
	Register("0", "1", func(map[string]any) error { return nil })
	defer func() {
		if recover() == nil {
			t.Error("expected panic on duplicate from=")
		}
	}()
	Register("0", "2", func(map[string]any) error { return nil })
}

func TestRegister_RejectsSameFromAndTo(t *testing.T) {
	defer resetForTest()
	resetForTest()
	defer func() {
		if recover() == nil {
			t.Error("expected panic on from==to")
		}
	}()
	Register("1", "1", func(map[string]any) error { return nil })
}

func TestMigrate_MalformedJSON_Errors(t *testing.T) {
	defer resetForTest()
	resetForTest()
	_, _, err := Migrate([]byte(`{not json`))
	if err == nil {
		t.Error("expected parse error")
	}
}
