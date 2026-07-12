// Package output handles rendering API results to a writer.
package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// Format selects how responses are rendered.
type Format string

const (
	// FormatJSON pretty-prints JSON (default).
	FormatJSON Format = "json"
	// FormatCompact prints single-line JSON.
	FormatCompact Format = "compact"
	// FormatRaw prints the response body exactly as received.
	FormatRaw Format = "raw"
)

// Options controls output rendering.
type Options struct {
	Writer io.Writer
	Format Format
}

// WriteBody renders a raw JSON response body according to the configured format.
func (o Options) WriteBody(body []byte) error {
	w := o.Writer
	switch o.Format {
	case FormatRaw:
		if _, err := w.Write(body); err != nil {
			return err
		}
		if len(body) > 0 && body[len(body)-1] != '\n' {
			_, err := io.WriteString(w, "\n")
			return err
		}
		return nil
	case FormatCompact:
		if len(bytes.TrimSpace(body)) == 0 {
			return nil
		}
		var buf bytes.Buffer
		if err := json.Compact(&buf, body); err != nil {
			// Not valid JSON; emit as-is.
			return o.writeRawFallback(body)
		}
		buf.WriteByte('\n')
		_, err := w.Write(buf.Bytes())
		return err
	default: // FormatJSON
		if len(bytes.TrimSpace(body)) == 0 {
			return nil
		}
		var buf bytes.Buffer
		if err := json.Indent(&buf, body, "", "  "); err != nil {
			return o.writeRawFallback(body)
		}
		buf.WriteByte('\n')
		_, err := w.Write(buf.Bytes())
		return err
	}
}

func (o Options) writeRawFallback(body []byte) error {
	_, err := o.Writer.Write(body)
	return err
}

// WriteValue marshals an arbitrary value as JSON using the configured format.
func (o Options) WriteValue(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal output: %w", err)
	}
	return o.WriteBody(data)
}

// ParseFormat validates and converts a format string.
func ParseFormat(s string) (Format, error) {
	switch Format(s) {
	case FormatJSON, FormatCompact, FormatRaw:
		return Format(s), nil
	case "":
		return FormatJSON, nil
	default:
		return "", fmt.Errorf("invalid output format %q (want json, compact, or raw)", s)
	}
}
