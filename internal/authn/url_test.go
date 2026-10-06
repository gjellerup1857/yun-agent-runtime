package authn

import "testing"

func TestValidateSecureURLAcceptsHTTPS(t *testing.T) {
	if _, err := ValidateSecureURL("https://auth.example.com/oauth2/introspect"); err != nil {
		t.Fatalf("HTTPS URL rejected: %v", err)
	}
}

func TestValidateSecureURLAllowsLoopbackHTTP(t *testing.T) {
	if _, err := ValidateSecureURL("http://127.0.0.1:8081/introspect"); err != nil {
		t.Fatalf("loopback HTTP URL rejected: %v", err)
	}
}

func TestValidateSecureURLRejectsRemoteHTTP(t *testing.T) {
	if _, err := ValidateSecureURL("http://auth.example.com/introspect"); err == nil {
		t.Fatal("expected remote HTTP URL to be rejected")
	}
}

func TestValidateSecureURLRejectsUserInfo(t *testing.T) {
	if _, err := ValidateSecureURL("https://user:secret@auth.example.com/introspect"); err == nil {
		t.Fatal("expected URL with user info to be rejected")
	}
}
