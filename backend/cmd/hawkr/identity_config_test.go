package main

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/config"
)

func TestClerkOriginDefaultDoesNotEnableClerk(t *testing.T) {
	env := config.FromMap(map[string]string{"CLERK_AUTHORIZED_PARTIES": "http://localhost:3000"})
	if got := readClerk(env, "CLERK_PUBLISHABLE_KEY", "CLERK_SECRET_KEY", "CLERK_AUTHORIZED_PARTIES"); got != nil || env.Err() != nil {
		t.Fatalf("dev-only configuration: %v %v", got, env.Err())
	}
}

func TestAppleConfigIsAllOrNothing(t *testing.T) {
	env := config.FromMap(map[string]string{})
	if c := readIdentityConfig(context.Background(), env); c.providers.Apple != nil || env.Err() != nil {
		t.Fatalf("沒有設定（本機開發）：%v %v", c.providers.Apple, env.Err())
	}
	env = config.FromMap(map[string]string{"APPLE_TEAM_ID": "TEAM"})
	readIdentityConfig(context.Background(), env)
	if env.Err() == nil {
		t.Fatal("只設一部分 Apple 設定必須失敗，不能悄悄停用")
	}
}
