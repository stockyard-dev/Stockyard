package parse

import (
	"strings"
	"testing"
)

func TestParseSource_SimpleFunc(t *testing.T) {
	src := []byte(`package example

func Hello(name string) string {
	return "hello " + name
}
`)
	units, err := ParseSource("example.go", src)
	if err != nil {
		t.Fatalf("ParseSource failed: %v", err)
	}
	if len(units) != 1 {
		t.Fatalf("expected 1 unit, got %d", len(units))
	}
	u := units[0]
	if u.Name != "Hello" {
		t.Errorf("name=%q, want Hello", u.Name)
	}
	if u.Kind != KindFunc {
		t.Errorf("kind=%q, want %q", u.Kind, KindFunc)
	}
	if u.Package != "example" {
		t.Errorf("package=%q, want example", u.Package)
	}
	if !strings.Contains(u.Signature, "Hello") {
		t.Errorf("signature %q should contain 'Hello'", u.Signature)
	}
	if !strings.Contains(u.Signature, "name string") {
		t.Errorf("signature %q should contain param", u.Signature)
	}
	if u.BodyHash == "" {
		t.Error("expected non-empty body hash")
	}
}

func TestParseSource_Method(t *testing.T) {
	src := []byte(`package svc

type Server struct{}

func (s *Server) Handle() error {
	return nil
}
`)
	units, err := ParseSource("svc.go", src)
	if err != nil {
		t.Fatalf("ParseSource failed: %v", err)
	}

	// Expect: type Server + method Handle
	var method *Unit
	for i := range units {
		if units[i].Kind == KindMethod {
			method = &units[i]
		}
	}
	if method == nil {
		t.Fatal("expected a method unit")
	}
	if method.Name != "Handle" {
		t.Errorf("method name=%q, want Handle", method.Name)
	}
	if method.Receiver != "*Server" {
		t.Errorf("receiver=%q, want *Server", method.Receiver)
	}
}

func TestParseSource_Interface(t *testing.T) {
	src := []byte(`package iface

type Store interface {
	Get(key string) (string, error)
	Set(key, value string) error
}
`)
	units, err := ParseSource("iface.go", src)
	if err != nil {
		t.Fatalf("ParseSource failed: %v", err)
	}

	var iface *Unit
	for i := range units {
		if units[i].Kind == KindInterface {
			iface = &units[i]
		}
	}
	if iface == nil {
		t.Fatal("expected an interface unit")
	}
	if iface.Name != "Store" {
		t.Errorf("name=%q, want Store", iface.Name)
	}
	if !strings.HasPrefix(iface.Signature, "type Store") {
		t.Errorf("signature=%q, expected to start with 'type Store'", iface.Signature)
	}
}

func TestParseSource_MultipleFuncs(t *testing.T) {
	src := []byte(`package multi

func A() {}
func B() {}
func C() {}
`)
	units, err := ParseSource("multi.go", src)
	if err != nil {
		t.Fatalf("ParseSource failed: %v", err)
	}

	funcCount := 0
	names := map[string]bool{}
	for _, u := range units {
		if u.Kind == KindFunc {
			funcCount++
			names[u.Name] = true
		}
	}
	if funcCount != 3 {
		t.Errorf("expected 3 functions, got %d", funcCount)
	}
	for _, name := range []string{"A", "B", "C"} {
		if !names[name] {
			t.Errorf("missing function %s", name)
		}
	}
}

func TestParseSource_Struct(t *testing.T) {
	src := []byte(`package types

type Config struct {
	Host string
	Port int
}
`)
	units, err := ParseSource("types.go", src)
	if err != nil {
		t.Fatalf("ParseSource failed: %v", err)
	}

	if len(units) != 1 {
		t.Fatalf("expected 1 unit, got %d", len(units))
	}
	if units[0].Kind != KindType {
		t.Errorf("kind=%q, want %q", units[0].Kind, KindType)
	}
	if units[0].Name != "Config" {
		t.Errorf("name=%q, want Config", units[0].Name)
	}
}

func TestParseSource_InvalidSyntax(t *testing.T) {
	src := []byte(`this is not valid go code {{{`)
	_, err := ParseSource("bad.go", src)
	if err == nil {
		t.Error("expected error for invalid Go source")
	}
}

func TestParseSource_EmptyFile(t *testing.T) {
	src := []byte(`package empty
`)
	units, err := ParseSource("empty.go", src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(units) != 0 {
		t.Errorf("expected 0 units for empty file, got %d", len(units))
	}
}

func TestUnit_ID(t *testing.T) {
	tests := []struct {
		name     string
		unit     Unit
		wantID   string
	}{
		{
			name:   "plain function",
			unit:   Unit{Package: "pkg", Name: "Foo"},
			wantID: "pkg.Foo",
		},
		{
			name:   "method with receiver",
			unit:   Unit{Package: "pkg", Name: "Bar", Receiver: "*Server"},
			wantID: "pkg.*Server.Bar",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.unit.ID()
			if got != tt.wantID {
				t.Errorf("ID()=%q, want %q", got, tt.wantID)
			}
		})
	}
}

func TestParseSource_FuncSignature(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		wantSig string
	}{
		{
			name:    "no params no return",
			src:     `package p; func F() {}`,
			wantSig: "func F()",
		},
		{
			name:    "with params and return",
			src:     `package p; func Add(a int, b int) int { return a + b }`,
			wantSig: "func Add(a int, b int) int",
		},
		{
			name:    "multiple returns",
			src:     `package p; func Get(k string) (string, error) { return "", nil }`,
			wantSig: "func Get(k string) (string, error)",
		},
		{
			name: "method receiver",
			src:  `package p; type S struct{}; func (s *S) Do() {}`,
			wantSig: "func (*S) Do()",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			units, err := ParseSource("sig.go", []byte(tt.src))
			if err != nil {
				t.Fatalf("ParseSource: %v", err)
			}
			// Find the function/method unit
			var funcUnit *Unit
			for i := range units {
				if units[i].Kind == KindFunc || units[i].Kind == KindMethod {
					funcUnit = &units[i]
					break
				}
			}
			if funcUnit == nil {
				t.Fatal("no function unit found")
			}
			if funcUnit.Signature != tt.wantSig {
				t.Errorf("signature=%q, want %q", funcUnit.Signature, tt.wantSig)
			}
		})
	}
}

func TestParseSource_LineNumbers(t *testing.T) {
	src := []byte(`package lines

// Comment
func First() {}

func Second() {
	x := 1
	_ = x
}
`)
	units, err := ParseSource("lines.go", src)
	if err != nil {
		t.Fatalf("ParseSource: %v", err)
	}

	funcUnits := map[string]Unit{}
	for _, u := range units {
		if u.Kind == KindFunc {
			funcUnits[u.Name] = u
		}
	}

	first, ok := funcUnits["First"]
	if !ok {
		t.Fatal("First function not found")
	}
	if first.StartLine != 4 {
		t.Errorf("First.StartLine=%d, want 4", first.StartLine)
	}

	second, ok := funcUnits["Second"]
	if !ok {
		t.Fatal("Second function not found")
	}
	if second.StartLine != 6 {
		t.Errorf("Second.StartLine=%d, want 6", second.StartLine)
	}
	if second.EndLine < second.StartLine {
		t.Errorf("Second.EndLine=%d < StartLine=%d", second.EndLine, second.StartLine)
	}
}

func TestHashContent_Deterministic(t *testing.T) {
	h1 := hashContent("func Foo() {}")
	h2 := hashContent("func Foo() {}")
	if h1 != h2 {
		t.Error("same content should produce same hash")
	}

	h3 := hashContent("func Bar() {}")
	if h1 == h3 {
		t.Error("different content should produce different hash")
	}
}

func TestHashContent_NormalizesWhitespace(t *testing.T) {
	h1 := hashContent("  func Foo() {}  ")
	h2 := hashContent("func Foo() {}")
	if h1 != h2 {
		t.Error("hashContent should normalize leading/trailing whitespace")
	}
}

func TestExtractLines(t *testing.T) {
	lines := []string{"line1", "line2", "line3", "line4", "line5"}

	tests := []struct {
		start, end int
		want       string
	}{
		{1, 3, "line1\nline2\nline3"},
		{2, 4, "line2\nline3\nline4"},
		{1, 1, "line1"},
		{0, 2, "line1\nline2"}, // start < 1 gets clamped
		{4, 10, "line4\nline5"}, // end > len gets clamped
	}

	for _, tt := range tests {
		got := extractLines(lines, tt.start, tt.end)
		if got != tt.want {
			t.Errorf("extractLines(%d,%d)=%q, want %q", tt.start, tt.end, got, tt.want)
		}
	}
}
