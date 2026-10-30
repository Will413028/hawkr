package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/config"
)

// writeDevKey 在 t 的暫存目錄寫一把 PKCS#8 的 RSA 私鑰，回傳路徑。
func writeDevKey(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "dev-auth.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// 顧客登入已經不由 Clerk 處理：設定表裡沒有 CLERK_ 的鍵，設了也不啟用任何東西、不算設定錯誤，更不影響 DEV_AUTH_KEY_FILE。
func TestClerkEnvironmentIsNotRead(t *testing.T) {
	keyFile := writeDevKey(t)
	clerkKeys := map[string]string{
		"CLERK_PUBLISHABLE_KEY":    "pk_live_x",
		"CLERK_SECRET_KEY":         "sk_live_x",
		"CLERK_JWT_KEY":            "jwt",
		"CLERK_AUTHORIZED_PARTIES": "http://localhost:3000",
	}
	cases := map[string]map[string]string{"全部": clerkKeys}
	for k, v := range clerkKeys {
		cases["只有 "+k] = map[string]string{k: v}
	}
	for name, clerk := range cases {
		t.Run(name, func(t *testing.T) {
			m := map[string]string{"DEV_AUTH_KEY_FILE": keyFile}
			for k, v := range clerk {
				m[k] = v
			}
			env := config.FromMap(m)
			c := readIdentityConfig(context.Background(), env)
			if err := env.Err(); err != nil {
				t.Fatalf("CLERK_ 的環境變數不該影響設定：%v", err)
			}
			if c.providers.Admin == nil {
				t.Error("DEV_AUTH_KEY_FILE 的後台 dev 登入被 CLERK_ 的設定關掉")
			}
			if c.providers.Google != nil || len(c.webLogin) != 0 || len(c.googleAppClients) != 0 {
				t.Errorf("CLERK_ 的設定啟用了其他登入：%+v", c)
			}
		})
	}
}

// DEV_AUTH_KEY_FILE 與 Cloudflare Access 同時設定是設定錯誤；兩者各自單獨設定都可以。
func TestDevAuthKeyFileExcludesAccess(t *testing.T) {
	keyFile := writeDevKey(t)
	access := map[string]string{"CF_ACCESS_TEAM_DOMAIN": "https://team.cloudflareaccess.com", "CF_ACCESS_ADMIN_AUD": "aud"}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	env := config.FromMap(map[string]string{"DEV_AUTH_KEY_FILE": keyFile})
	if readIdentityConfig(ctx, env); env.Err() != nil {
		t.Fatalf("只有 DEV_AUTH_KEY_FILE：%v", env.Err())
	}
	env = config.FromMap(access)
	if c := readIdentityConfig(ctx, env); env.Err() != nil || c.providers.Admin == nil {
		t.Fatalf("只有 Access：%v %v", c.providers.Admin, env.Err())
	}
	both := map[string]string{"DEV_AUTH_KEY_FILE": keyFile}
	for k, v := range access {
		both[k] = v
	}
	env = config.FromMap(both)
	readIdentityConfig(ctx, env)
	if err := env.Err(); err == nil || !strings.Contains(err.Error(), "DEV_AUTH_KEY_FILE 不能與 Cloudflare Access 設定同時使用") {
		t.Fatalf("DEV_AUTH_KEY_FILE 與 Access 同時設定必須被拒：%v", err)
	}
}

// 不提供 Sign in with Apple（計畫 2026-10-30-self-hosted-auth 前提段）：設定表裡沒有 APPLE_ 與 IDENTITY_TOKEN_KEYS 的鍵，設了也不啟用
// 任何東西、不算設定錯誤（舊的 .env 還留著也能啟動），更不影響 DEV_AUTH_KEY_FILE。
func TestAppleEnvironmentIsNotRead(t *testing.T) {
	keyFile := writeDevKey(t)
	appleEnv := map[string]string{ //nolint:gosec // G101：假的設定值，不是憑證
		"APPLE_CLIENT_IDS":    "com.example.buyer",
		"APPLE_TEAM_ID":       "TEAM",
		"APPLE_KEY_ID":        "KEY",
		"APPLE_PRIVATE_KEY":   "not a pem",
		"APPLE_REDIRECT_URIS": "web=https://example.com/cb",
		"IDENTITY_TOKEN_KEYS": "k1:not-base64",
	}
	cases := map[string]map[string]string{"全部": appleEnv}
	for k, v := range appleEnv {
		cases["只有 "+k] = map[string]string{k: v}
	}
	for name, apple := range cases {
		t.Run(name, func(t *testing.T) {
			m := map[string]string{"DEV_AUTH_KEY_FILE": keyFile}
			for k, v := range apple {
				m[k] = v
			}
			env := config.FromMap(m)
			c := readIdentityConfig(context.Background(), env)
			if err := env.Err(); err != nil {
				t.Fatalf("APPLE_ 的環境變數不該影響設定：%v", err)
			}
			if c.providers.Admin == nil {
				t.Error("DEV_AUTH_KEY_FILE 的後台 dev 登入被 APPLE_ 的設定關掉")
			}
			if c.providers.Google != nil || len(c.webLogin) != 0 || len(c.googleAppClients) != 0 {
				t.Errorf("APPLE_ 的設定啟用了其他登入：%+v", c)
			}
		})
	}
}
