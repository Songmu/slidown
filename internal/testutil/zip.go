// Package testutil provides fixtures and ZIP primitives for presentation tests.
// It must only be imported by tests.
package testutil

import (
	"archive/zip"
	"bytes"
	"io"
	"slices"
	"testing"
)

// ReadZIP returns byte parts and their archive order.
func ReadZIP(t testing.TB, data []byte) (map[string][]byte, []string) {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("read ZIP: %v", err)
	}
	parts := make(map[string][]byte, len(zr.File))
	order := make([]string, 0, len(zr.File))
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open ZIP part %s: %v", f.Name, err)
		}
		b, readErr := io.ReadAll(rc)
		closeErr := rc.Close()
		if readErr != nil {
			t.Fatalf("read ZIP part %s: %v", f.Name, readErr)
		}
		if closeErr != nil {
			t.Fatalf("close ZIP part %s: %v", f.Name, closeErr)
		}
		parts[f.Name] = b
		order = append(order, f.Name)
	}
	return parts, order
}

// ReadZIPStrings reads parts for tests that assert on XML text.
func ReadZIPStrings(t testing.TB, data []byte) map[string]string {
	t.Helper()
	raw, _ := ReadZIP(t, data)
	parts := make(map[string]string, len(raw))
	for name, b := range raw {
		parts[name] = string(b)
	}
	return parts
}

// WriteZIP preserves the supplied order, omits deleted parts, and appends new
// parts sorted by name. A nil order creates a new archive sorted by name.
func WriteZIP(t testing.TB, parts map[string][]byte, order []string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	written := make(map[string]bool, len(parts))
	write := func(name string) {
		t.Helper()
		data, ok := parts[name]
		if !ok || written[name] {
			return
		}
		fw, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create ZIP part %s: %v", name, err)
		}
		if _, err := fw.Write(data); err != nil {
			t.Fatalf("write ZIP part %s: %v", name, err)
		}
		written[name] = true
	}
	for _, name := range order {
		write(name)
	}
	var added []string
	for name := range parts {
		if !written[name] {
			added = append(added, name)
		}
	}
	slices.Sort(added)
	for _, name := range added {
		write(name)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close ZIP: %v", err)
	}
	return buf.Bytes()
}
