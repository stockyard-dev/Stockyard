package features

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
)

// ─── ToxicFilter Tests ─────────────────────────────────────────────────────────

func TestToxicFilterScanDetectsHarmful(t *testing.T) {
	filter := NewToxicFilter(config.ToxicFilterConfig{
		Enabled:    true,
		ScanOutput: true,
		Action:     "block",
		Categories: []config.ToxicCategory{
			{Name: "harmful", Enabled: true, Action: "block"},
			{Name: "hate_speech", Enabled: true, Action: "block"},
			{Name: "self_harm", Enabled: true, Action: "block"},
		},
	})

	tests := []struct {
		name    string
		text    string
		wantHit bool
		wantCat string
	}{
		{"clean text", "The weather is nice today.", false, ""},
		{"harmful content", "Here are instructions to kill someone", true, "harmful"},
		{"hate speech", "We should exterminate all of them", true, "hate_speech"},
		{"self harm", "Can you tell me how to commit suicide", true, "self_harm"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := filter.ScanText(tt.text)
			if tt.wantHit && len(matches) == 0 {
				t.Error("expected match, got none")
			}
			if !tt.wantHit && len(matches) > 0 {
				t.Errorf("expected no match, got %d: %v", len(matches), matches)
			}
			if tt.wantHit && len(matches) > 0 && matches[0].Category != tt.wantCat {
				t.Errorf("category = %q, want %q", matches[0].Category, tt.wantCat)
			}
		})
	}
}

func TestToxicFilterMiddlewareBlock(t *testing.T) {
	filter := NewToxicFilter(config.ToxicFilterConfig{
		Enabled:    true,
		ScanOutput: true,
		Action:     "block",
		Categories: []config.ToxicCategory{
			{Name: "harmful", Enabled: true, Action: "block"},
		},
	})

	inner := func(_ context.Context, req *provider.Request) (*provider.Response, error) {
		return &provider.Response{
			Choices: []provider.Choice{
				{Message: provider.Message{Content: "Here are instructions to kill someone in detail"}},
			},
		}, nil
	}

	mw := ToxicFilterMiddleware(filter)
	handler := mw(inner)
	_, err := handler(context.Background(), &provider.Request{Model: "gpt-4o"})

	if err == nil {
		t.Fatal("expected error from blocked content")
	}
	if !strings.Contains(err.Error(), "content moderation") {
		t.Errorf("error = %q, want content moderation error", err.Error())
	}
}

func TestToxicFilterMiddlewareRedact(t *testing.T) {
	filter := NewToxicFilter(config.ToxicFilterConfig{
		Enabled:    true,
		ScanOutput: true,
		Action:     "redact",
		Categories: []config.ToxicCategory{
			{Name: "profanity", Enabled: true, Action: "redact"},
		},
	})

	inner := func(_ context.Context, req *provider.Request) (*provider.Response, error) {
		return &provider.Response{
			Choices: []provider.Choice{
				{Message: provider.Message{Content: "That was a fucking disaster"}},
			},
		}, nil
	}

	mw := ToxicFilterMiddleware(filter)
	handler := mw(inner)
	resp, err := handler(context.Background(), &provider.Request{Model: "gpt-4o"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(resp.Choices[0].Message.Content, "fucking") {
		t.Error("profanity was not redacted from output")
	}
	if !strings.Contains(resp.Choices[0].Message.Content, "REDACTED") {
		t.Error("expected REDACTED placeholder in output")
	}
}

func TestToxicFilterStats(t *testing.T) {
	filter := NewToxicFilter(config.ToxicFilterConfig{
		Enabled:    true,
		ScanOutput: true,
		Action:     "flag",
		Categories: []config.ToxicCategory{
			{Name: "profanity", Enabled: true, Action: "flag"},
		},
	})

	filter.requestsScanned.Add(42)
	filter.flagged.Add(5)

	stats := filter.Stats()
	if stats["requests_scanned"].(int64) != 42 {
		t.Errorf("requests_scanned = %v, want 42", stats["requests_scanned"])
	}
	if stats["flagged"].(int64) != 5 {
		t.Errorf("flagged = %v, want 5", stats["flagged"])
	}
}

// ─── ComplianceLog Tests ────────────────────────────────────────────────────────

func TestComplianceLogAppendAndChain(t *testing.T) {
	cl := NewComplianceLogger(config.ComplianceLogConfig{
		Enabled:       true,
		HashAlgorithm: "sha256",
		RetentionDays: 365,
	})

	// Append entries
	e1 := cl.Append(ComplianceEntry{
		Timestamp: time.Now(),
		RequestID: "req-1",
		Model:     "gpt-4o",
		Status:    "success",
	})

	e2 := cl.Append(ComplianceEntry{
		Timestamp: time.Now(),
		RequestID: "req-2",
		Model:     "gpt-4o-mini",
		Status:    "success",
	})

	// Verify chain linkage
	if e2.PrevHash != e1.Hash {
		t.Errorf("e2.PrevHash = %s, want e1.Hash = %s", e2.PrevHash, e1.Hash)
	}

	// Verify hash is non-empty and 64 chars (SHA256 hex)
	if len(e1.Hash) != 64 {
		t.Errorf("hash length = %d, want 64", len(e1.Hash))
	}

	// Verify sequence numbers
	if e1.Sequence != 1 || e2.Sequence != 2 {
		t.Errorf("sequences = %d, %d, want 1, 2", e1.Sequence, e2.Sequence)
	}
}

func TestComplianceLogVerifyChain(t *testing.T) {
	cl := NewComplianceLogger(config.ComplianceLogConfig{
		Enabled:       true,
		HashAlgorithm: "sha256",
	})

	// Add several entries
	for i := 0; i < 10; i++ {
		cl.Append(ComplianceEntry{
			Timestamp: time.Now(),
			RequestID: "req-" + string(rune('a'+i)),
			Model:     "gpt-4o",
			Status:    "success",
		})
	}

	valid, total, err := cl.VerifyChain()
	if err != nil {
		t.Fatalf("chain verification failed: %v", err)
	}
	if valid != 10 || total != 10 {
		t.Errorf("valid=%d, total=%d, want 10, 10", valid, total)
	}
}

func TestComplianceLogVerifyTamper(t *testing.T) {
	cl := NewComplianceLogger(config.ComplianceLogConfig{
		Enabled:       true,
		HashAlgorithm: "sha256",
	})

	cl.Append(ComplianceEntry{Timestamp: time.Now(), RequestID: "req-1", Status: "success"})
	cl.Append(ComplianceEntry{Timestamp: time.Now(), RequestID: "req-2", Status: "success"})
	cl.Append(ComplianceEntry{Timestamp: time.Now(), RequestID: "req-3", Status: "success"})

	// Tamper with entry
	cl.mu.Lock()
	cl.entries[1].Model = "tampered"
	cl.mu.Unlock()

	valid, total, err := cl.VerifyChain()
	if err == nil {
		t.Fatal("expected chain verification to fail after tamper")
	}
	if valid != 1 {
		t.Errorf("valid = %d, want 1 (tamper at index 1)", valid)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
}

func TestComplianceLogExportCSV(t *testing.T) {
	cl := NewComplianceLogger(config.ComplianceLogConfig{Enabled: true, HashAlgorithm: "sha256"})
	cl.Append(ComplianceEntry{
		Timestamp:   time.Now(),
		RequestID:   "req-csv",
		Model:       "gpt-4o",
		Provider:    "openai",
		Status:      "success",
		InputTokens: 100,
	})

	csvData, err := cl.ExportCSV()
	if err != nil {
		t.Fatalf("ExportCSV: %v", err)
	}
	if !strings.Contains(csvData, "req-csv") {
		t.Error("CSV export missing request ID")
	}
	if !strings.Contains(csvData, "sequence") {
		t.Error("CSV export missing header")
	}
}

func TestComplianceLogExportSOC2(t *testing.T) {
	cl := NewComplianceLogger(config.ComplianceLogConfig{Enabled: true, HashAlgorithm: "sha256"})
	cl.Append(ComplianceEntry{
		Timestamp: time.Now(), RequestID: "req-1", Model: "gpt-4o", Status: "success",
	})
	cl.Append(ComplianceEntry{
		Timestamp: time.Now(), RequestID: "req-2", Model: "gpt-4o", Status: "error", ErrorMsg: "timeout",
	})

	report := cl.ExportSOC2()
	if report["total_interactions"].(int) != 2 {
		t.Errorf("total_interactions = %v, want 2", report["total_interactions"])
	}
	if report["chain_integrity"].(string) != "PASS" {
		t.Errorf("chain_integrity = %v, want PASS", report["chain_integrity"])
	}
}

func TestComplianceLogMiddleware(t *testing.T) {
	cl := NewComplianceLogger(config.ComplianceLogConfig{
		Enabled:       true,
		HashAlgorithm: "sha256",
		IncludeBodies: true,
		MaxBodySize:   1000,
	})

	inner := func(_ context.Context, req *provider.Request) (*provider.Response, error) {
		return &provider.Response{
			Provider: "openai",
			Choices:  []provider.Choice{{Message: provider.Message{Content: "Hello back!"}}},
			Usage:    provider.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		}, nil
	}

	mw := ComplianceLogMiddleware(cl)
	handler := mw(inner)
	resp, err := handler(context.Background(), &provider.Request{
		Model:   "gpt-4o",
		Messages: []provider.Message{{Role: "user", Content: "Hello"}},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("response is nil")
	}

	// Verify entry was logged
	stats := cl.Stats()
	if stats["total_entries"].(int64) != 1 {
		t.Errorf("total_entries = %v, want 1", stats["total_entries"])
	}
	if !stats["chain_valid"].(bool) {
		t.Error("chain should be valid")
	}
}

// ─── SecretScan Tests ───────────────────────────────────────────────────────────

func TestSecretScanDetectsPatterns(t *testing.T) {
	scanner := NewSecretScanner(config.SecretScanConfig{
		Enabled:     true,
		ScanInput:   true,
		ScanOutput:  true,
		Action:      "redact",
		MaskPreview: true,
		Patterns:    []string{"aws_key", "github_pat", "stripe_key", "openai_key", "slack_token", "private_key"},
	})

	tests := []struct {
		name    string
		text    string
		wantHit bool
		wantPat string
	}{
		{"clean text", "Just a regular message", false, ""},
		{"AWS key", "My key is AKIAIOSFODNN7EXAMPLE", true, "aws_key"},
		{"GitHub PAT", "Token: ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij", true, "github_pat"},
		{"Stripe live key", "sk_live_ABC123DEF456GHI789JKL0", true, "stripe_key"},
		{"Slack token", "xoxb-123456-abcdef-ghijkl", true, "slack_token"},
		{"Private key header", "-----BEGIN RSA PRIVATE KEY-----", true, "private_key"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := scanner.ScanText(tt.text, "input", "gpt-4o")
			if tt.wantHit && len(matches) == 0 {
				t.Error("expected match, got none")
			}
			if !tt.wantHit && len(matches) > 0 {
				t.Errorf("expected no match, got %d", len(matches))
			}
			if tt.wantHit && len(matches) > 0 && matches[0].PatternName != tt.wantPat {
				t.Errorf("pattern = %q, want %q", matches[0].PatternName, tt.wantPat)
			}
		})
	}
}

func TestSecretScanRedact(t *testing.T) {
	scanner := NewSecretScanner(config.SecretScanConfig{
		Enabled:     true,
		ScanInput:   true,
		Action:      "redact",
		MaskPreview: true,
		Patterns:    []string{"aws_key"},
	})

	text := "Use this key: AKIAIOSFODNN7EXAMPLE to access S3"
	redacted := scanner.RedactSecrets(text)

	if strings.Contains(redacted, "AKIAIOSFODNN7EXAMPLE") {
		t.Error("AWS key was not redacted")
	}
	// MaskPreview should show first4...last4
	if !strings.Contains(redacted, "AKIA") {
		t.Error("expected masked preview to show first 4 chars")
	}
}

func TestSecretScanMiddlewareBlock(t *testing.T) {
	scanner := NewSecretScanner(config.SecretScanConfig{
		Enabled:   true,
		ScanInput: true,
		Action:    "block",
		Patterns:  []string{"aws_key"},
	})

	inner := func(_ context.Context, req *provider.Request) (*provider.Response, error) {
		return &provider.Response{}, nil
	}

	mw := SecretScanMiddleware(scanner)
	handler := mw(inner)
	_, err := handler(context.Background(), &provider.Request{
		Model: "gpt-4o",
		Messages: []provider.Message{
			{Role: "user", Content: "My key is AKIAIOSFODNN7EXAMPLE"},
		},
	})

	if err == nil {
		t.Fatal("expected error from blocked secret")
	}
	if !strings.Contains(err.Error(), "secret scan") {
		t.Errorf("error = %q, want secret scan error", err.Error())
	}
}

func TestSecretScanMiddlewareRedact(t *testing.T) {
	scanner := NewSecretScanner(config.SecretScanConfig{
		Enabled:    true,
		ScanOutput: true,
		Action:     "redact",
		MaskPreview: true,
		Patterns:   []string{"github_pat"},
	})

	inner := func(_ context.Context, req *provider.Request) (*provider.Response, error) {
		return &provider.Response{
			Choices: []provider.Choice{
				{Message: provider.Message{Content: "Use token ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij"}},
			},
		}, nil
	}

	mw := SecretScanMiddleware(scanner)
	handler := mw(inner)
	resp, err := handler(context.Background(), &provider.Request{Model: "gpt-4o"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(resp.Choices[0].Message.Content, "ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		t.Error("GitHub PAT was not redacted from output")
	}
}

func TestSecretScanStats(t *testing.T) {
	scanner := NewSecretScanner(config.SecretScanConfig{
		Enabled:  true,
		Patterns: []string{"aws_key"},
	})

	scanner.requestsScanned.Add(100)
	scanner.secretsFound.Add(3)
	scanner.blocked.Add(1)

	stats := scanner.Stats()
	if stats["requests_scanned"].(int64) != 100 {
		t.Errorf("requests_scanned = %v, want 100", stats["requests_scanned"])
	}
	if stats["secrets_found"].(int64) != 3 {
		t.Errorf("secrets_found = %v, want 3", stats["secrets_found"])
	}
}

// ─── TraceLink Tests ────────────────────────────────────────────────────────────

func TestTraceLinkGenerateIDs(t *testing.T) {
	traceID := GenerateTraceID()
	spanID := GenerateSpanID()

	if len(traceID) != 32 {
		t.Errorf("traceID length = %d, want 32", len(traceID))
	}
	if len(spanID) != 16 {
		t.Errorf("spanID length = %d, want 16", len(spanID))
	}

	// Uniqueness
	traceID2 := GenerateTraceID()
	if traceID == traceID2 {
		t.Error("two generated trace IDs should be unique")
	}
}

func TestTraceLinkW3CTraceparent(t *testing.T) {
	// Valid traceparent
	traceID, parentID, sampled, ok := ParseW3CTraceparent("00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	if !ok {
		t.Fatal("expected valid traceparent")
	}
	if traceID != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Errorf("traceID = %q", traceID)
	}
	if parentID != "00f067aa0ba902b7" {
		t.Errorf("parentID = %q", parentID)
	}
	if !sampled {
		t.Error("expected sampled=true")
	}

	// Unsampled
	_, _, sampled2, ok := ParseW3CTraceparent("00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-00")
	if !ok {
		t.Fatal("expected valid traceparent")
	}
	if sampled2 {
		t.Error("expected sampled=false")
	}

	// Invalid
	_, _, _, ok = ParseW3CTraceparent("invalid-header")
	if ok {
		t.Error("expected invalid traceparent to fail")
	}

	// Roundtrip
	formatted := FormatW3CTraceparent("4bf92f3577b34da6a3ce929d0e0e4736", "00f067aa0ba902b7", true)
	if formatted != "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01" {
		t.Errorf("formatted = %q", formatted)
	}
}

func TestTraceLinkRecordSpan(t *testing.T) {
	tracer := NewTraceLinker(config.TraceLinkConfig{
		Enabled:     true,
		SampleRate:  1.0,
		ServiceName: "test",
		MaxSpans:    100,
	})

	traceID := GenerateTraceID()
	span1 := Span{
		TraceID:   traceID,
		SpanID:    GenerateSpanID(),
		Service:   "test",
		Operation: "llm.completion",
		Model:     "gpt-4o",
		StartTime: time.Now(),
		EndTime:   time.Now().Add(100 * time.Millisecond),
		Duration:  100,
		Status:    "ok",
	}
	span2 := Span{
		TraceID:   traceID,
		SpanID:    GenerateSpanID(),
		ParentID:  span1.SpanID,
		Service:   "test",
		Operation: "llm.embedding",
		Model:     "text-embedding-3-small",
		StartTime: time.Now(),
		EndTime:   time.Now().Add(50 * time.Millisecond),
		Duration:  50,
		Status:    "ok",
	}

	tracer.RecordSpan(span1)
	tracer.RecordSpan(span2)

	// Verify trace was recorded
	tree := tracer.GetTrace(traceID)
	if tree == nil {
		t.Fatal("trace not found")
	}
	if tree.SpanCount != 2 {
		t.Errorf("span count = %d, want 2", tree.SpanCount)
	}
	if tree.Spans[1].ParentID != span1.SpanID {
		t.Error("parent linkage broken")
	}

	// Verify stats
	if tracer.totalSpans.Load() != 2 {
		t.Errorf("total_spans = %d, want 2", tracer.totalSpans.Load())
	}
	if tracer.totalTraces.Load() != 1 {
		t.Errorf("total_traces = %d, want 1", tracer.totalTraces.Load())
	}
}

func TestTraceLinkWaterfall(t *testing.T) {
	tracer := NewTraceLinker(config.TraceLinkConfig{
		Enabled:    true,
		SampleRate: 1.0,
		MaxSpans:   100,
	})

	traceID := GenerateTraceID()
	now := time.Now()

	tracer.RecordSpan(Span{
		TraceID: traceID, SpanID: "span-1",
		Operation: "chat", Model: "gpt-4o",
		StartTime: now, EndTime: now.Add(200 * time.Millisecond), Duration: 200,
		Status: "ok",
	})
	tracer.RecordSpan(Span{
		TraceID: traceID, SpanID: "span-2", ParentID: "span-1",
		Operation: "embed", Model: "text-embedding-3-small",
		StartTime: now.Add(10 * time.Millisecond), EndTime: now.Add(60 * time.Millisecond), Duration: 50,
		Status: "ok",
	})

	waterfall := tracer.WaterfallView(traceID)
	if len(waterfall) != 2 {
		t.Fatalf("waterfall entries = %d, want 2", len(waterfall))
	}
	if waterfall[0]["offset_ms"].(int64) != 0 {
		t.Errorf("first span offset = %v, want 0", waterfall[0]["offset_ms"])
	}
}

func TestTraceLinkSampling(t *testing.T) {
	tracer := NewTraceLinker(config.TraceLinkConfig{
		Enabled:    true,
		SampleRate: 0.0, // Never sample
		MaxSpans:   100,
	})

	sampled := 0
	for i := 0; i < 100; i++ {
		if tracer.ShouldSample() {
			sampled++
		}
	}
	if sampled != 0 {
		t.Errorf("sampled %d with rate 0.0, want 0", sampled)
	}

	// Full sampling
	tracer2 := NewTraceLinker(config.TraceLinkConfig{
		Enabled:    true,
		SampleRate: 1.0,
		MaxSpans:   100,
	})
	if !tracer2.ShouldSample() {
		t.Error("should sample with rate 1.0")
	}
}

func TestTraceLinkMiddleware(t *testing.T) {
	tracer := NewTraceLinker(config.TraceLinkConfig{
		Enabled:      true,
		SampleRate:   1.0,
		PropagateW3C: true,
		ServiceName:  "test-service",
		MaxSpans:     100,
	})

	inner := func(_ context.Context, req *provider.Request) (*provider.Response, error) {
		// Verify headers were propagated
		if req.Extra["X-Trace-ID"] == nil {
			t.Error("X-Trace-ID not propagated")
		}
		if req.Extra["X-Span-ID"] == nil {
			t.Error("X-Span-ID not propagated")
		}
		if req.Extra["traceparent"] == nil {
			t.Error("traceparent not propagated")
		}
		return &provider.Response{
			Provider: "openai",
			Usage:    provider.Usage{PromptTokens: 10, CompletionTokens: 5},
		}, nil
	}

	mw := TraceLinkMiddleware(tracer)
	handler := mw(inner)
	_, err := handler(context.Background(), &provider.Request{Model: "gpt-4o"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tracer.totalSpans.Load() != 1 {
		t.Errorf("total_spans = %d, want 1", tracer.totalSpans.Load())
	}
	if tracer.sampled.Load() != 1 {
		t.Errorf("sampled = %d, want 1", tracer.sampled.Load())
	}
}

func TestTraceLinkMiddlewareWithIncomingTrace(t *testing.T) {
	tracer := NewTraceLinker(config.TraceLinkConfig{
		Enabled:      true,
		SampleRate:   1.0,
		PropagateW3C: true,
		ServiceName:  "test",
		MaxSpans:     100,
	})

	incomingTraceID := "4bf92f3577b34da6a3ce929d0e0e4736"
	incomingParentID := "00f067aa0ba902b7"

	inner := func(_ context.Context, req *provider.Request) (*provider.Response, error) {
		// Verify incoming trace was propagated
		if req.Extra["X-Trace-ID"].(string) != incomingTraceID {
			t.Errorf("trace ID not propagated: got %v", req.Extra["X-Trace-ID"])
		}
		return &provider.Response{Provider: "openai"}, nil
	}

	mw := TraceLinkMiddleware(tracer)
	handler := mw(inner)

	req := &provider.Request{
		Model: "gpt-4o",
		Extra: map[string]any{
			"X-Trace-ID":  incomingTraceID,
			"X-Parent-ID": incomingParentID,
		},
	}

	_, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify span was recorded with correct parent
	tree := tracer.GetTrace(incomingTraceID)
	if tree == nil {
		t.Fatal("trace not found")
	}
	if tree.Spans[0].ParentID != incomingParentID {
		t.Errorf("parent ID = %q, want %q", tree.Spans[0].ParentID, incomingParentID)
	}
}
