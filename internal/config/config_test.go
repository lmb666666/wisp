package config

import "testing"

func TestLoadValidatesPublicURL(t *testing.T) {
	t.Setenv("WISP_PUBLIC_URL", "https://wisp.example.com")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("valid URL rejected: %v", err)
	}
	if cfg.PublicURL != "https://wisp.example.com" {
		t.Fatalf("PublicURL = %q", cfg.PublicURL)
	}

	for _, bad := range []string{"wisp.example.com", "ftp://wisp.example.com", "http://"} {
		t.Setenv("WISP_PUBLIC_URL", bad)
		if _, err := Load(); err == nil {
			t.Errorf("invalid URL %q accepted", bad)
		}
	}
}
