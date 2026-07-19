package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	DatabaseURL        string
	JWTSecret          string
	AllowedEmailDomain string
	Port               string

	// SMTP is optional. When SMTPHost is empty, features that would send
	// email (e.g. emailing a defect report to a supplier) record the
	// action without actually sending, instead of failing.
	SMTPHost     string
	SMTPPort     string
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string
}

// loadDotEnv reads KEY=VALUE pairs from a .env file in the current
// directory, if one exists, and applies them via os.Setenv for any key not
// already set in the real environment (which always takes precedence).
func loadDotEnv() {
	f, err := os.Open(".env")
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if _, set := os.LookupEnv(key); !set {
			os.Setenv(key, value)
		}
	}
}

func Load() (Config, error) {
	loadDotEnv()

	cfg := Config{
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		JWTSecret:          os.Getenv("JWT_SECRET"),
		AllowedEmailDomain: os.Getenv("ALLOWED_EMAIL_DOMAIN"),
		Port:               os.Getenv("PORT"),
		SMTPHost:           os.Getenv("SMTP_HOST"),
		SMTPPort:           os.Getenv("SMTP_PORT"),
		SMTPUsername:       os.Getenv("SMTP_USERNAME"),
		SMTPPassword:       os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:           os.Getenv("SMTP_FROM"),
	}
	if cfg.Port == "" {
		cfg.Port = "4083"
	}
	if cfg.AllowedEmailDomain == "" {
		cfg.AllowedEmailDomain = "sbs.com.pg"
	}
	if cfg.DatabaseURL == "" {
		return cfg, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return cfg, fmt.Errorf("JWT_SECRET is required")
	}
	return cfg, nil
}
