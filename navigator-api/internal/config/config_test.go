package config

import (
	"os"
	"testing"
)

func TestConfig_DatabaseTypeResolution(t *testing.T) {
	tests := []struct {
		name         string
		envVars      map[string]string
		expectedType string
		expectedURL  string
	}{
		{
			name: "Explicit SQLite via DB_TYPE",
			envVars: map[string]string{
				"DB_TYPE":      "sqlite",
				"DATABASE_URL": "custom_navigator.db",
			},
			expectedType: "sqlite",
			expectedURL:  "custom_navigator.db",
		},
		{
			name: "Auto-detect SQLite via .db extension in DATABASE_URL",
			envVars: map[string]string{
				"DB_TYPE":      "",
				"DATABASE_URL": "./data/prod.db",
			},
			expectedType: "sqlite",
			expectedURL:  "./data/prod.db",
		},
		{
			name: "Auto-detect SQLite via sqlite: prefix",
			envVars: map[string]string{
				"DB_TYPE":      "",
				"DATABASE_URL": "sqlite:test.db",
			},
			expectedType: "sqlite",
			expectedURL:  "test.db",
		},
		{
			name: "Explicit SQLite via DATABASE_PATH",
			envVars: map[string]string{
				"DB_TYPE":       "",
				"DATABASE_PATH": "/app/data/app.db",
			},
			expectedType: "sqlite",
			expectedURL:  "/app/data/app.db",
		},
		{
			name: "Explicit PostgreSQL via DB_TYPE",
			envVars: map[string]string{
				"DB_TYPE":      "postgres",
				"DATABASE_URL": "postgres://user:pass@localhost:5432/mydb",
			},
			expectedType: "postgres",
			expectedURL:  "postgres://user:pass@localhost:5432/mydb",
		},
		{
			name: "PostgreSQL via DB_HOST environment variable",
			envVars: map[string]string{
				"DB_TYPE": "",
				"DB_HOST": "postgres-server",
			},
			expectedType: "postgres",
		},
		{
			name: "Zero-config default falls back to embedded SQLite for zero-setup local execution",
			envVars: map[string]string{
				"DB_TYPE":       "",
				"DATABASE_URL":  "",
				"DATABASE_PATH": "",
				"DB_PATH":       "",
				"DB_HOST":       "",
			},
			expectedType: "sqlite",
			expectedURL:  "navigator.db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear env vars
			os.Unsetenv("DB_TYPE")
			os.Unsetenv("DB_DRIVER")
			os.Unsetenv("DATABASE_URL")
			os.Unsetenv("DATABASE_PATH")
			os.Unsetenv("DB_PATH")
			os.Unsetenv("DB_HOST")

			for k, v := range tt.envVars {
				if v != "" {
					os.Setenv(k, v)
				}
			}

			cfg := Load()

			if cfg.DatabaseType != tt.expectedType {
				t.Errorf("expected DatabaseType '%s', got '%s'", tt.expectedType, cfg.DatabaseType)
			}
			if tt.expectedURL != "" && cfg.DatabaseURL != tt.expectedURL {
				t.Errorf("expected DatabaseURL '%s', got '%s'", tt.expectedURL, cfg.DatabaseURL)
			}
		})
	}
}
