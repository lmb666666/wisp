package importer

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

func bytesReader(b []byte) *bytes.Reader { return bytes.NewReader(b) }

func writeTemp(data []byte, pattern string) (string, error) {
	tmp, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", fmt.Errorf("create temp: %w", err)
	}
	defer func() { _ = tmp.Close() }()
	if _, err := tmp.Write(data); err != nil {
		_ = os.Remove(tmp.Name())
		return "", fmt.Errorf("write temp: %w", err)
	}
	return tmp.Name(), nil
}

func removeTemp(name string) { _ = os.Remove(filepath.Clean(name)) }
