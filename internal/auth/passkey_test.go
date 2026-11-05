package auth

import (
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
)

func TestLoadSessionRejectsExpiredCeremony(t *testing.T) {
	s := &WebAuthnService{}
	s.sessions.Store("expired", &ceremonyEntry{
		data: &webauthn.SessionData{},
		exp:  time.Now().Add(-time.Minute),
	})
	if _, err := s.loadSession("expired"); err == nil {
		t.Fatal("expired ceremony was accepted")
	}
}

func TestLoadSessionAcceptsFreshCeremony(t *testing.T) {
	s := &WebAuthnService{}
	s.sessions.Store("fresh", &ceremonyEntry{
		data: &webauthn.SessionData{},
		exp:  time.Now().Add(time.Minute),
	})
	if _, err := s.loadSession("fresh"); err != nil {
		t.Fatalf("fresh ceremony rejected: %v", err)
	}
}
