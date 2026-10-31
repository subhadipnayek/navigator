package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port           string
	DatabaseType   string // "postgres" or "sqlite"
	DatabaseURL    string
	JwtSecret      string
	JwtIssuer      string
	JwtAudience    string
	JwtExpiryHours int
	CorsOrigins    []string
}

func Load() *Config {
	port := getEnv("PORT", "8080")
	if port == "" {
		port = "8080"
	}

	// 1. Resolve Database Engine & Connection
	dbType := strings.ToLower(strings.TrimSpace(getEnv("DB_TYPE", getEnv("DB_DRIVER", ""))))
	rawURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	dbPath := strings.TrimSpace(getEnv("DATABASE_PATH", getEnv("DB_PATH", "")))

	if dbType == "" {
		if dbPath != "" || strings.HasSuffix(rawURL, ".db") || strings.HasSuffix(rawURL, ".sqlite") || strings.HasPrefix(rawURL, "sqlite:") || strings.HasPrefix(rawURL, "file:") {
			dbType = "sqlite"
		} else if strings.HasPrefix(rawURL, "postgres://") || strings.HasPrefix(rawURL, "postgresql://") || strings.Contains(rawURL, "host=") {
			dbType = "postgres"
		} else if os.Getenv("DB_HOST") != "" {
			dbType = "postgres"
		} else {
			// Zero-config default: embedded SQLite for zero-setup local run
			dbType = "sqlite"
		}
	}

	var dbURL string
	if dbType == "sqlite" {
		if dbPath != "" {
			dbURL = dbPath
		} else if rawURL != "" {
			dbURL = strings.TrimPrefix(rawURL, "sqlite:")
		} else {
			dbURL = "navigator.db"
		}
	} else {
		dbType = "postgres"
		dbURL = rawURL
		if dbURL == "" {
			host := getEnv("DB_HOST", "localhost")
			dbPort := getEnv("DB_PORT", "5432")
			user := getEnv("DB_USER", "devuser")
			password := getEnv("DB_PASSWORD", "devpassword")
			dbname := getEnv("DB_NAME", "NAVigatorDb_QA")
			sslmode := getEnv("DB_SSLMODE", "disable")

			dbURL = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
				host, dbPort, user, password, dbname, sslmode)
		}
	}

	// 2. JWT Configuration
	jwtSecret := getEnv("JWT_KEY", "dev-only-change-this-secret-key-before-production-use")
	jwtIssuer := getEnv("JWT_ISSUER", "NAVigator")
	jwtAudience := getEnv("JWT_AUDIENCE", "NAVigator")
	jwtExpiryHours, _ := strconv.Atoi(getEnv("JWT_EXPIRES_HOURS", "8"))
	if jwtExpiryHours <= 0 {
		jwtExpiryHours = 8
	}

	// 3. CORS Allowed Origins
	corsStr := getEnv("CORS_ORIGINS", "http://localhost:4000,http://localhost:4200,http://localhost:5038,http://localhost:8080")
	var origins []string
	for _, o := range strings.Split(corsStr, ",") {
		trimmed := strings.TrimSpace(o)
		if trimmed != "" {
			origins = append(origins, trimmed)
		}
	}

	return &Config{
		Port:           port,
		DatabaseType:   dbType,
		DatabaseURL:    dbURL,
		JwtSecret:      jwtSecret,
		JwtIssuer:      jwtIssuer,
		JwtAudience:    jwtAudience,
		JwtExpiryHours: jwtExpiryHours,
		CorsOrigins:    origins,
	}
}

func (c *Config) JwtExpiryDuration() time.Duration {
	return time.Duration(c.JwtExpiryHours) * time.Hour
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return fallback
}
