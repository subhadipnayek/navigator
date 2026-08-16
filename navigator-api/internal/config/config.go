package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port           string
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

	// Support standard connection strings from Docker or environment
	dbURL := getEnv("DATABASE_URL", "")
	if dbURL == "" {
		// Support ASP.NET style ConnectionStrings__DefaultConnection
		aspnetConn := getEnv("ConnectionStrings__DefaultConnection", "")
		if aspnetConn != "" {
			dbURL = parseAspnetConnectionString(aspnetConn)
		} else {
			host := getEnv("DB_HOST", "localhost")
			dbPort := getEnv("DB_PORT", "5432")
			user := getEnv("DB_USER", "devuser")
			password := getEnv("DB_PASSWORD", "devpassword")
			dbname := getEnv("DB_NAME", "NAVigatorDb_QA")
			dbURL = "host=" + host + " port=" + dbPort + " user=" + user + " password=" + password + " dbname=" + dbname + " sslmode=disable"
		}
	}

	jwtSecret := getEnv("JWT_KEY", "dev-only-change-this-secret-key-before-production-use")
	jwtIssuer := getEnv("JWT_ISSUER", "NAVigator")
	jwtAudience := getEnv("JWT_AUDIENCE", "NAVigator")
	jwtExpiryHours, _ := strconv.Atoi(getEnv("JWT_EXPIRES_HOURS", "8"))
	if jwtExpiryHours <= 0 {
		jwtExpiryHours = 8
	}

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

func parseAspnetConnectionString(conn string) string {
	// Example: Host=db;Port=5432;Database=NAVigatorDb_QA;Username=devuser;Password=devpassword;
	parts := strings.Split(conn, ";")
	var host, port, dbname, user, password string
	for _, part := range parts {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		k := strings.ToLower(strings.TrimSpace(kv[0]))
		v := strings.TrimSpace(kv[1])
		switch k {
		case "host", "server":
			host = v
		case "port":
			port = v
		case "database", "initial catalog":
			dbname = v
		case "username", "user id", "user":
			user = v
		case "password":
			password = v
		}
	}

	if host == "" {
		host = "localhost"
	}
	if port == "" {
		port = "5432"
	}
	if dbname == "" {
		dbname = "NAVigatorDb_QA"
	}
	if user == "" {
		user = "devuser"
	}
	if password == "" {
		password = "devpassword"
	}

	return "host=" + host + " port=" + port + " user=" + user + " password=" + password + " dbname=" + dbname + " sslmode=disable"
}
