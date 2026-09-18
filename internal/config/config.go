package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr           string
	DatabaseURL    string
	ReconixBinary  string
	ReconixConfig  string
	ReconixWorkdir string
	APIKey         string
	MaxConcurrent  int
	MaxDuration    time.Duration
	MaxOutput      int64
	PollInterval   time.Duration
	StaleAfter     time.Duration
	TargetMode     string
	AllowedDomains []string
	AllowedCIDRs   []string
}

func Load() Config {
	return Config{
		Addr:           env("API_ADDR", ":8080"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		ReconixBinary:  env("RECONIX_BINARY", "reconix"),
		ReconixConfig:  env("RECONIX_CONFIG", "config/config.yaml"),
		ReconixWorkdir: os.Getenv("RECONIX_WORKDIR"),
		APIKey:         os.Getenv("API_KEY"),
		MaxConcurrent:  envInt("MAX_CONCURRENT_SCANS", 2),
		MaxDuration:    time.Duration(envInt("MAX_SCAN_DURATION_SECONDS", 900)) * time.Second,
		MaxOutput:      int64(envInt("MAX_OUTPUT_SIZE", 16*1024*1024)),
		PollInterval:   time.Duration(envInt("WORKER_POLL_INTERVAL_SECONDS", 1)) * time.Second,
		StaleAfter:     time.Duration(envInt("STALE_JOB_TIMEOUT_SECONDS", 1800)) * time.Second,
		TargetMode:     env("TARGET_POLICY_MODE", "allowlist"),
		AllowedDomains: split(os.Getenv("ALLOWED_DOMAINS")),
		AllowedCIDRs:   split(os.Getenv("ALLOWED_CIDRS")),
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(env(key, strconv.Itoa(fallback)))
	if err != nil || value < 1 {
		return fallback
	}
	return value
}
func split(value string) []string {
	var result []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			result = append(result, strings.ToLower(item))
		}
	}
	return result
}
