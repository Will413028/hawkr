package database

import (
	"net/url"
	"strings"

	pkgconfig "github.com/Will413028/hawkr/go-pkg/config"
)

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
	TimeZone string
}

const poolerSafeExecModeParam = "default_query_exec_mode=exec"

func resolveSSLMode(s string) string {
	if s == "" {
		return "disable"
	}
	return s
}

func BuildDSN(cfg DBConfig) string {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.User, cfg.Password),
		Host:   cfg.Host + ":" + cfg.Port,
		Path:   "/" + cfg.Name,
	}
	q := u.Query()
	q.Set("sslmode", resolveSSLMode(cfg.SSLMode))
	if cfg.TimeZone != "" {
		q.Set("TimeZone", cfg.TimeZone)
	}
	k, v, _ := strings.Cut(poolerSafeExecModeParam, "=")
	q.Set(k, v)
	u.RawQuery = q.Encode()
	return u.String()
}

func BuildDSNKeyword(cfg DBConfig) string {
	parts := []string{
		"host=" + cfg.Host,
		"port=" + cfg.Port,
		"user=" + cfg.User,
		"password=" + cfg.Password,
		"dbname=" + cfg.Name,
		"sslmode=" + resolveSSLMode(cfg.SSLMode),
	}
	if cfg.TimeZone != "" {
		parts = append(parts, "TimeZone="+cfg.TimeZone)
	}
	parts = append(parts, poolerSafeExecModeParam)
	return strings.Join(parts, " ")
}

func LoadDBConfigFromEnv() DBConfig {
	return DBConfig{
		Host:     pkgconfig.GetEnvOrDefault("DB_HOST", "localhost"),
		Port:     ResolvePoolPort(),
		User:     pkgconfig.GetEnvOrDefault("DB_USER", "dbadmin"),
		Password: pkgconfig.GetEnvOrDefault("DB_PASSWORD", ""),
		Name:     pkgconfig.GetEnvOrDefault("DB_NAME", ""),
		SSLMode:  pkgconfig.GetEnvOrDefault("DB_SSLMODE", "disable"),
		TimeZone: pkgconfig.GetEnvOrDefault("DB_TIMEZONE", ""),
	}
}

func ResolvePoolPort() string {
	if p := pkgconfig.GetEnvOrDefault("DB_POOL_PORT", ""); p != "" {
		return p
	}
	return pkgconfig.GetEnvOrDefault("DB_PORT", "5432")
}
