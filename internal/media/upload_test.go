package media

import (
	"testing"
)

func TestStoreKindReportsConcreteBackend(t *testing.T) {
	local, err := NewLocalStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name  string
		store Storage
		want  string
	}{
		{"local", local, "local"},
		{"s3", &S3Storage{}, "s3"},
		{"lsky", &LskyStorage{}, "lsky"},
	}
	for _, tc := range cases {
		sel := NewSelector(local)
		sel.Configure(tc.store, false)
		u := &Uploader{store: sel}
		if got := u.storeKind(); got != tc.want {
			t.Errorf("%s storeKind = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestSanitizeUploadMime(t *testing.T) {
	cases := []struct {
		name     string
		declared string
		detected string
		want     string
	}{
		{"png", "image/png", "image/png", "image/png"},
		{"declared html", "text/html", "text/html", "application/octet-stream"},
		{"disguised html", "image/png", "text/html", "application/octet-stream"},
		{"svg", "image/svg+xml", "image/svg+xml", "application/octet-stream"},
		{"detected fallback", "application/octet-stream", "application/pdf", "application/pdf"},
	}
	for _, tc := range cases {
		if got := sanitizeUploadMime(tc.declared, tc.detected); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestUploaderShaLockCleanup(t *testing.T) {
	u := &Uploader{}
	unlock := u.lockSHA("same")
	if len(u.shaLocks) != 1 {
		t.Fatalf("lock map = %d entries", len(u.shaLocks))
	}
	unlock()
	if len(u.shaLocks) != 0 {
		t.Fatalf("lock map not cleaned: %d entries", len(u.shaLocks))
	}
}
