package database

import (
	"strings"
	"testing"
)

func TestBuildDSN_RequireSSL(t *testing.T) {
	cfg := DBConfig{
		Host: "rds.example", Port: "5432",
		User: "u", Password: "p", Name: "db",
		SSLMode: "require", TimeZone: "UTC",
	}
	got := BuildDSN(cfg)
	want := "postgres://u:p@rds.example:5432/db?TimeZone=UTC&default_query_exec_mode=exec&sslmode=require"
	if got != want {
		t.Fatalf("BuildDSN() = %q, want %q", got, want)
	}
}

func TestBuildDSNKeyword_Disable(t *testing.T) {
	cfg := DBConfig{
		Host: "localhost", Port: "5432",
		User: "u", Password: "p", Name: "db",
		SSLMode: "disable",
	}
	got := BuildDSNKeyword(cfg)
	want := "host=localhost port=5432 user=u password=p dbname=db sslmode=disable default_query_exec_mode=exec"
	if got != want {
		t.Fatalf("BuildDSNKeyword() = %q, want %q", got, want)
	}
}

func TestBuildDSNKeyword_WithTimeZone(t *testing.T) {
	cfg := DBConfig{
		Host: "h", Port: "5432", User: "u", Password: "p",
		Name: "db", SSLMode: "require", TimeZone: "Asia/Taipei",
	}
	got := BuildDSNKeyword(cfg)
	want := "host=h port=5432 user=u password=p dbname=db sslmode=require TimeZone=Asia/Taipei default_query_exec_mode=exec"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestLoadDBConfigFromEnv_DefaultsSSLDisable(t *testing.T) {
	t.Setenv("DB_HOST", "h")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_USER", "u")
	t.Setenv("DB_PASSWORD", "p")
	t.Setenv("DB_NAME", "db")
	t.Setenv("DB_SSLMODE", "")
	cfg := LoadDBConfigFromEnv()
	if cfg.SSLMode != "disable" {
		t.Fatalf("default SSLMode = %q, want disable", cfg.SSLMode)
	}
}

func TestLoadDBConfigFromEnv_RespectsSSLMode(t *testing.T) {
	t.Setenv("DB_SSLMODE", "require")
	cfg := LoadDBConfigFromEnv()
	if cfg.SSLMode != "require" {
		t.Fatalf("SSLMode = %q, want require", cfg.SSLMode)
	}
}

func TestBuildDSN_EmptySSLModeDefaultsToDisable(t *testing.T) {
	cfg := DBConfig{
		Host: "h", Port: "5432", User: "u", Password: "p", Name: "db",
	}
	got := BuildDSN(cfg)
	if !strings.Contains(got, "sslmode=disable") {
		t.Fatalf("expected sslmode=disable in DSN, got %q", got)
	}
}

func TestBuildDSNKeyword_EmptySSLModeDefaultsToDisable(t *testing.T) {
	cfg := DBConfig{
		Host: "h", Port: "5432", User: "u", Password: "p", Name: "db",
	}
	got := BuildDSNKeyword(cfg)
	if !strings.Contains(got, "sslmode=disable") {
		t.Fatalf("expected sslmode=disable in keyword DSN, got %q", got)
	}
}

func TestResolvePoolPort_PrefersPoolPort(t *testing.T) {
	t.Setenv("DB_POOL_PORT", "6432")
	t.Setenv("DB_PORT", "5432")
	if got := ResolvePoolPort(); got != "6432" {
		t.Fatalf("ResolvePoolPort() = %q, want 6432", got)
	}
}

func TestResolvePoolPort_FallsBackToDirectPort(t *testing.T) {
	t.Setenv("DB_POOL_PORT", "")
	t.Setenv("DB_PORT", "5432")
	if got := ResolvePoolPort(); got != "5432" {
		t.Fatalf("ResolvePoolPort() = %q, want 5432", got)
	}
}

func TestResolvePoolPort_DefaultsWhenNeitherSet(t *testing.T) {
	t.Setenv("DB_POOL_PORT", "")
	t.Setenv("DB_PORT", "")
	if got := ResolvePoolPort(); got != "5432" {
		t.Fatalf("ResolvePoolPort() = %q, want default 5432", got)
	}
}
