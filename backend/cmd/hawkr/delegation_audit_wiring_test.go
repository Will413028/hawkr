package main

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/delegatedactor"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

type noSeller struct{}

func (noSeller) Store(context.Context, int64) (seller.Store, error) { return seller.Store{}, nil }

type noSocial struct{ delegation.Social }

// events.PublishRequired（指名的 handler 不在就失敗關閉），不在這裡；serve、worker 與 CLI 的組裝（newDelegation、newDelegationOffline）都經過它，
// 所以這個測試確認正式組裝真的接了這些事件，兩條路徑各一次。
func TestNewDelegationSubscribesEveryPostAndProductWriteEvent(t *testing.T) {
	bus := events.NewBus()
	newDelegation(bus, delegation.Deps{Seller: noSeller{}, Social: noSocial{}, NewID: uuid.NewV7})
	assertAuditHandlers(t, bus)
}

// serve、worker 與 CLI 的離線組裝各自建 social，每個建 social 的檔案也要經 newDelegation 接稽核 handler（社交寫入的 PublishRequired 才不會在那條路徑失敗關閉、
// 或更糟：被別的訂閱者蒙混）。檔案層級的計數：建 social 的呼叫數不超過 newDelegation 的呼叫數；social.go 是 newSocial 的定義處，不算。
func TestEveryEntrypointThatBuildsSocialAssemblesDelegation(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	buildsSocial := regexp.MustCompile(`\bnewSocial\(|\bsocial\.New\(`)
	assemblesDelegation := regexp.MustCompile(`\bnewDelegation\(`)
	found := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") || f == "social.go" {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		builds, assembles := len(buildsSocial.FindAll(src, -1)), len(assemblesDelegation.FindAll(src, -1))
		found += builds
		if builds > assembles {
			t.Errorf("%s 建了 %d 次 social，卻只有 %d 次 newDelegation", f, builds, assembles)
		}
	}
	if found < 3 {
		t.Errorf("只找到 %d 個建 social 的入口，預期 serve、worker、離線 CLI 三個", found)
	}
}

func assertAuditHandlers(t *testing.T, bus *events.Bus) {
	t.Helper()
	for name, ok := range map[string]bool{
		"PostCreated":              events.HasNamed[social.PostCreated](bus, delegatedactor.AuditHandler),
		"PostUpdated":              events.HasNamed[social.PostUpdated](bus, delegatedactor.AuditHandler),
		"PostPublished":            events.HasNamed[social.PostPublished](bus, delegatedactor.AuditHandler),
		"PostUnpublished":          events.HasNamed[social.PostUnpublished](bus, delegatedactor.AuditHandler),
		"PostScheduled":            events.HasNamed[social.PostScheduled](bus, delegatedactor.AuditHandler),
		"PostPublishRepeated":      events.HasNamed[social.PostPublishRepeated](bus, delegatedactor.AuditHandler),
		"PostUnpublishRepeated":    events.HasNamed[social.PostUnpublishRepeated](bus, delegatedactor.AuditHandler),
		"ProductCreated":           events.HasNamed[catalog.ProductCreated](bus, delegatedactor.AuditHandler),
		"ProductUpdated":           events.HasNamed[catalog.ProductUpdated](bus, delegatedactor.AuditHandler),
		"ProductPublished":         events.HasNamed[catalog.ProductPublished](bus, delegatedactor.AuditHandler),
		"ProductUnpublished":       events.HasNamed[catalog.ProductUnpublished](bus, delegatedactor.AuditHandler),
		"ProductPublishRepeated":   events.HasNamed[catalog.ProductPublishRepeated](bus, delegatedactor.AuditHandler),
		"ProductUnpublishRepeated": events.HasNamed[catalog.ProductUnpublishRepeated](bus, delegatedactor.AuditHandler),
		"ProductsUnpublished":      events.HasNamed[catalog.ProductsUnpublished](bus, delegatedactor.AuditHandler),
	} {
		if !ok {
			t.Errorf("%s 沒有訂閱者", name)
		}
	}
}
