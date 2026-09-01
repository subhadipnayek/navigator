package services_test

import (
	"testing"

	"navigator-api/internal/config"
	"navigator-api/internal/models"
	"navigator-api/internal/services"
)

func TestAuthService_HashAndVerifyPassword(t *testing.T) {
	cfg := &config.Config{
		JwtSecret:         "test-secret-key-that-is-long-enough-32-chars",
		JwtIssuer:         "NAVigator-Test",
		JwtAudience:       "NAVigator-Clients",
		JwtExpiryHours:   24,
	}
	svc := services.NewAuthService(nil, cfg)

	password := "SecretPassword@123"

	hash, err := svc.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if hash == "" || hash == password {
		t.Fatalf("Expected hashed output, got '%s'", hash)
	}

	// Correct password verification
	if !svc.VerifyPassword(password, hash) {
		t.Errorf("VerifyPassword should succeed for correct password")
	}

	// Incorrect password verification
	if svc.VerifyPassword("WrongPassword", hash) {
		t.Errorf("VerifyPassword should fail for wrong password")
	}

	// Corrupted hash
	if svc.VerifyPassword(password, "corrupted.hash") {
		t.Errorf("VerifyPassword should fail for malformed hash")
	}
}

func TestAuthService_GenerateToken(t *testing.T) {
	cfg := &config.Config{
		JwtSecret:        "test-secret-key-that-is-long-enough-32-chars",
		JwtIssuer:        "NAVigator-Test",
		JwtAudience:      "NAVigator-Clients",
		JwtExpiryHours:   24,
	}
	svc := services.NewAuthService(nil, cfg)

	user := &models.AppUser{
		Id:       42,
		Username: "subhadip",
		Email:    "subhadip@example.com",
		Role:     "Admin",
	}

	token, expiresAt, err := svc.GenerateToken(user)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	if token == "" {
		t.Errorf("Expected valid JWT token string, got empty")
	}

	if expiresAt.IsZero() {
		t.Errorf("Expected valid expiration timestamp")
	}
}
