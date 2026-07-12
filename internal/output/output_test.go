package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteBodyJSON(t *testing.T) {
	var buf bytes.Buffer
	o := Options{Writer: &buf, Format: FormatJSON}
	if err := o.WriteBody([]byte(`{"a":1,"b":2}`)); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "\n  \"a\": 1") {
		t.Errorf("expected indented JSON, got:\n%s", got)
	}
}

func TestWriteBodyCompact(t *testing.T) {
	var buf bytes.Buffer
	o := Options{Writer: &buf, Format: FormatCompact}
	if err := o.WriteBody([]byte("{\n  \"a\": 1\n}")); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(buf.String()); got != `{"a":1}` {
		t.Errorf("compact = %q", got)
	}
}

func TestWriteBodyRaw(t *testing.T) {
	var buf bytes.Buffer
	o := Options{Writer: &buf, Format: FormatRaw}
	if err := o.WriteBody([]byte(`{"a":1}`)); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "{\"a\":1}\n" {
		t.Errorf("raw = %q", got)
	}
}

func TestWriteBodyEmpty(t *testing.T) {
	var buf bytes.Buffer
	o := Options{Writer: &buf, Format: FormatJSON}
	if err := o.WriteBody(nil); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output for empty body, got %q", buf.String())
	}
}

func TestParseFormat(t *testing.T) {
	for _, s := range []string{"json", "compact", "raw", ""} {
		if _, err := ParseFormat(s); err != nil {
			t.Errorf("ParseFormat(%q) unexpected error: %v", s, err)
		}
	}
	if _, err := ParseFormat("yaml"); err == nil {
		t.Error("expected error for invalid format")
	}
}
