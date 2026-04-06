package config_test

import (
	"os"
	"testing"

	"github.com/kelsoncm/pdf-builder/internal/config"
)

func TestLoad_Defaults(t *testing.T) {
	cfg, err := config.Load("/nonexistent")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Wkhtmltopdf.TimeoutSeconds != 60 {
		t.Errorf("expected timeout 60, got %d", cfg.Wkhtmltopdf.TimeoutSeconds)
	}
}

func TestLoad_UsersFromEnv(t *testing.T) {
	os.Setenv("PDFSERVICE_USER_TESTUSER", "test-token-123")
	defer os.Unsetenv("PDFSERVICE_USER_TESTUSER")

	cfg, err := config.Load("/nonexistent")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	tokenMap := cfg.TokenMap()
	username, ok := tokenMap["test-token-123"]
	if !ok {
		t.Error("expected token from env to be in token map")
	}
	if username != "testuser" {
		t.Errorf("expected username 'testuser', got %q", username)
	}
}

func TestTokenMap(t *testing.T) {
	cfg := &config.Config{
		Users: []config.User{
			{Username: "alice", Token: "token-alice"},
			{Username: "bob", Token: "token-bob"},
		},
	}
	m := cfg.TokenMap()
	if m["token-alice"] != "alice" {
		t.Error("alice token not mapped correctly")
	}
	if m["token-bob"] != "bob" {
		t.Error("bob token not mapped correctly")
	}
}
