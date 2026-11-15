package social_test

import (
	"context"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/social"
)

type noBlocks struct{}

func (noBlocks) StoreBlocksBuyer(context.Context, int64, int64) (bool, error) { return false, nil }

// 缺少任何一種發起者的授權實作時，social.New 在啟動時失敗（不是到點才發現）：每一種 ScheduleKind，除了 social 自己提供的 OWNER，都要在 Deps 給。
func TestNewRequiresAnAuthorizerForEveryScheduleKind(t *testing.T) {
	t.Parallel()
	allow := social.ScheduleAuthorizerFunc(func(context.Context, social.ScheduledPublish) error { return nil })
	for _, kind := range social.ScheduleKindValues() {
		if kind == social.ScheduleByOwner {
			continue
		}
		func() {
			defer func() {
				r := recover()
				if r == nil {
					t.Fatalf("缺 %s 的授權實作要失敗", kind)
				}
				if err, ok := r.(error); !ok || !strings.Contains(err.Error(), string(kind)) {
					t.Fatalf("錯誤要指出缺哪一種：%v", r)
				}
			}()
			social.New(social.Deps{StoreBlocks: noBlocks{}, NewID: uuid.NewV7, Now: time.Now, ScheduleAuthorizers: map[social.ScheduleKind]social.ScheduleAuthorizer{social.ScheduleByOwner: allow}})
		}()
	}
}
