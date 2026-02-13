package smtp

import "testing"

func TestPlainAuth(t *testing.T) {
	auth := PlainAuth("", "user", "pass")
	if auth.Name() != "PLAIN" {
		t.Errorf("Name() = %q, want PLAIN", auth.Name())
	}

	resp, err := auth.Start()
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	want := "\x00user\x00pass"
	if string(resp) != want {
		t.Errorf("Start() = %q, want %q", resp, want)
	}

	_, err = auth.Next(nil)
	if err == nil {
		t.Error("Next should fail for PLAIN")
	}
}

func TestPlainAuth_WithIdentity(t *testing.T) {
	auth := PlainAuth("admin", "user", "pass")
	resp, err := auth.Start()
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	want := "admin\x00user\x00pass"
	if string(resp) != want {
		t.Errorf("Start() = %q, want %q", resp, want)
	}
}

func TestLoginAuth(t *testing.T) {
	auth := LoginAuth("user", "pass")
	if auth.Name() != "LOGIN" {
		t.Errorf("Name() = %q, want LOGIN", auth.Name())
	}

	// Start returns nil (no initial response).
	resp, err := auth.Start()
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if resp != nil {
		t.Errorf("Start() = %v, want nil", resp)
	}

	// First challenge: Username.
	resp, err = auth.Next([]byte("Username:"))
	if err != nil {
		t.Fatalf("Next(Username): %v", err)
	}
	if string(resp) != "user" {
		t.Errorf("Next(Username) = %q, want %q", resp, "user")
	}

	// Second challenge: Password.
	resp, err = auth.Next([]byte("Password:"))
	if err != nil {
		t.Fatalf("Next(Password): %v", err)
	}
	if string(resp) != "pass" {
		t.Errorf("Next(Password) = %q, want %q", resp, "pass")
	}

	// Third call should fail.
	_, err = auth.Next(nil)
	if err == nil {
		t.Error("third Next should fail")
	}
}

func TestCramMD5Auth(t *testing.T) {
	auth := CramMD5Auth("user", "secret")
	if auth.Name() != "CRAM-MD5" {
		t.Errorf("Name() = %q, want CRAM-MD5", auth.Name())
	}

	// Start returns nil.
	resp, err := auth.Start()
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if resp != nil {
		t.Errorf("Start() = %v, want nil", resp)
	}

	// Process challenge.
	challenge := []byte("<12345.67890@test.example.com>")
	resp, err = auth.Next(challenge)
	if err != nil {
		t.Fatalf("Next: %v", err)
	}

	// Response should be "user <hex digest>".
	parts := string(resp)
	if !containsString(parts, "user ") {
		t.Errorf("response %q should start with username", parts)
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr
}
