package logx

import (
	"bytes"
	"strings"
	"testing"
)

func TestRedactionInStdlogWriter(t *testing.T) {
	var buf bytes.Buffer
	w := StdlogWriter(LevelDebug, &buf)
	SetMinLevel(LevelDebug)
	SetVerbose(false)
	RegisterSecret("secret123")

	_, err := w.Write([]byte("this contains secret123 and should be redacted\n"))
	if err != nil {
		t.Fatalf("write error: %v", err)
	}
	got := buf.String()
	if strings.Contains(got, "secret123") {
		t.Fatalf("expected secret to be redacted, got: %s", got)
	}
	if !strings.Contains(got, "[REDACTED]") {
		t.Fatalf("expected [REDACTED] marker, got: %s", got)
	}
}

func TestTruncationWhenNotVerbose(t *testing.T) {
	var buf bytes.Buffer
	w := StdlogWriter(LevelDebug, &buf)
	SetMinLevel(LevelDebug)
	SetVerbose(false)

	long := strings.Repeat("a", 6000)
	_, err := w.Write([]byte(long + "\n"))
	if err != nil {
		t.Fatalf("write error: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "truncated") {
		t.Fatalf("expected truncation indicator, got: %s", got)
	}
}

func TestNoTruncationWhenVerbose(t *testing.T) {
	var buf bytes.Buffer
	w := StdlogWriter(LevelDebug, &buf)
	SetMinLevel(LevelDebug)
	SetVerbose(true)

	long := strings.Repeat("b", 4000)
	_, err := w.Write([]byte(long + "\n"))
	if err != nil {
		t.Fatalf("write error: %v", err)
	}
	got := buf.String()
	if strings.Contains(got, "truncated") {
		t.Fatalf("did not expect truncation, got: %s", got)
	}
}
