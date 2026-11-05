package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHardenFilesSetsPrivateMode(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "wisp.db")
	if err := os.WriteFile(dbPath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	wal := dbPath + "-wal"
	shm := dbPath + "-shm"
	if err := os.WriteFile(wal, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(shm, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := HardenFiles(dbPath); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{dbPath, wal, shm} {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Errorf("%s mode = %o, want 600", p, got)
		}
	}
}
