package characterization

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	ch "github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
)

var uuidRE = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

var scenarios = []ch.Scenario[Driver]{
	{Name: "create public session returns presigned url and 30 minute ttl", Run: func(ctx context.Context, t ch.T, d Driver) {
		s := ch.Must(t, ch.Ret(d.CreateUploadSession(ctx, ch.Buyer, UploadInput{
			Filename: "photo-" + ch.Nonce() + ".jpg", ContentType: "image/jpeg", FileSize: "1024",
		})))
		if d.Variant() == Monolith {
			if s.ID == "" {
				t.Errorf("staged token 是空的")
			}
		} else if !uuidRE.MatchString(s.ID) {
			t.Errorf("sessionId = %q，預期 UUID", s.ID)
		}
		if s.UploadURL == "" {
			t.Errorf("公開上傳的 uploadUrl 是空的")
		}
		if d.Variant() == Monolith {
			secs, err := strconv.Atoi(s.ExpiresIn)
			ttl := int(media.UploadTTL / time.Second)
			if err != nil || secs > ttl || secs <= 0 {
				t.Errorf("expiresIn = %q，預期 1 到 %d 之間", s.ExpiresIn, ttl)
			}
		} else if s.ExpiresIn != "1800" {
			t.Errorf("expiresIn = %q，預期 1800", s.ExpiresIn)
		}
		if s.IsSecure {
			t.Errorf("isSecure = true，預期 false")
		}
	}},
	{Name: "create secure session has no upload url and allows pdf", Run: func(ctx context.Context, t ch.T, d Driver) {
		s := ch.Must(t, ch.Ret(d.CreateUploadSession(ctx, ch.Buyer, UploadInput{
			Filename: "doc-" + ch.Nonce() + ".pdf", ContentType: "application/pdf", FileSize: "2048", IsSecure: true,
		})))
		if s.UploadURL != "" {
			t.Errorf("機敏上傳不該有 uploadUrl：%q", s.UploadURL)
		}
		if !s.IsSecure {
			t.Errorf("isSecure = false，預期 true")
		}
		if d.Variant() == Monolith {
			if s.ID == "" {
				t.Errorf("staged token 是空的")
			}
		} else if !uuidRE.MatchString(s.ID) {
			t.Errorf("sessionId = %q，預期 UUID", s.ID)
		}
	}},
	{Name: "create rejects pdf for public upload", Run: func(ctx context.Context, t ch.T, d Driver) {
		_, err := d.CreateUploadSession(ctx, ch.Buyer, UploadInput{Filename: "a.pdf", ContentType: "application/pdf", FileSize: "100"})
		ch.WantKind(t, err, ch.ErrInvalid)
	}},
	{Name: "create rejects unsupported mime", Run: func(ctx context.Context, t ch.T, d Driver) {
		_, err := d.CreateUploadSession(ctx, ch.Buyer, UploadInput{Filename: "a.svg", ContentType: "image/svg+xml", FileSize: "100"})
		ch.WantKind(t, err, ch.ErrInvalid)
	}},
	{Name: "create rejects declared size over limit", Run: func(ctx context.Context, t ch.T, d Driver) {
		_, err := d.CreateUploadSession(ctx, ch.Buyer, UploadInput{Filename: "a.jpg", ContentType: "image/jpeg", FileSize: "10485761"})
		ch.WantKind(t, err, ch.ErrInvalid)
	}},
	{Name: "create rejects non positive size", Run: func(ctx context.Context, t ch.T, d Driver) {
		_, err := d.CreateUploadSession(ctx, ch.Buyer, UploadInput{Filename: "a.jpg", ContentType: "image/jpeg", FileSize: "0"})
		ch.WantKind(t, err, ch.ErrInvalid)
	}},
	{Name: "complete returns staged token and preview url", Run: func(ctx context.Context, t ch.T, d Driver) {
		s := ch.Must(t, ch.Ret(d.CreateUploadSession(ctx, ch.Buyer, UploadInput{
			Filename: "photo-" + ch.Nonce() + ".png", ContentType: "image/png", FileSize: "4096",
		})))
		c := ch.Must(t, ch.Ret(d.CompleteUploadSession(ctx, ch.Buyer, s.ID)))
		if d.Variant() == Monolith {
			if c.StagedToken != s.ID || c.State != "UPLOADED" {
				t.Errorf("完成後 token = %q（預期 %q）、state = %q（預期 UPLOADED）", c.StagedToken, s.ID, c.State)
			}
			return
		}
		if strings.Count(c.StagedToken, ".") != 2 {
			t.Errorf("stagedToken = %q，預期 JWT 三段", c.StagedToken)
		}
		if c.PreviewURL == "" {
			t.Errorf("previewUrl 是空的")
		}
	}},
	{Name: "complete twice fails", Run: func(ctx context.Context, t ch.T, d Driver) {
		s := ch.Must(t, ch.Ret(d.CreateUploadSession(ctx, ch.Buyer, UploadInput{
			Filename: "photo-" + ch.Nonce() + ".webp", ContentType: "image/webp", FileSize: "512",
		})))
		ch.Must(t, ch.Ret(d.CompleteUploadSession(ctx, ch.Buyer, s.ID)))
		_, err := d.CompleteUploadSession(ctx, ch.Buyer, s.ID)
		if d.Variant() == Monolith {
			if err != nil {
				t.Fatalf("重送完成應該成功，實際：%v", err)
			}
			return
		}
		if !errors.Is(err, ch.ErrRejected) {
			t.Fatalf("第二次完成同一個會話應該被服務以業務層 envelope 拒絕，實際：%v", err)
		}
	}},
	{Name: "complete rejects secure session", Run: func(ctx context.Context, t ch.T, d Driver) {
		s := ch.Must(t, ch.Ret(d.CreateUploadSession(ctx, ch.Buyer, UploadInput{
			Filename: "doc-" + ch.Nonce() + ".pdf", ContentType: "application/pdf", FileSize: "2048", IsSecure: true,
		})))
		_, err := d.CompleteUploadSession(ctx, ch.Buyer, s.ID)
		if d.Variant() == Monolith {
			if err != nil {
				t.Fatalf("機敏 token 的 CompleteUpload 現況是成功且無作用，實際：%v", err)
			}
			return
		}
		ch.WantKind(t, err, ch.ErrInvalid)
	}},
	{Name: "complete of another accounts session is not found", Run: func(ctx context.Context, t ch.T, d Driver) {
		s := ch.Must(t, ch.Ret(d.CreateUploadSession(ctx, ch.Buyer, UploadInput{
			Filename: "photo-" + ch.Nonce() + ".gif", ContentType: "image/gif", FileSize: "256",
		})))
		_, err := d.CompleteUploadSession(ctx, ch.Buyer2, s.ID)
		ch.WantKind(t, err, ch.ErrNotFound)
	}},
	{Name: "complete rejects malformed session id", Run: func(ctx context.Context, t ch.T, d Driver) {
		_, err := d.CompleteUploadSession(ctx, ch.Buyer, "not-a-uuid")
		if d.Variant() == Monolith {
			ch.WantKind(t, err, ch.ErrNotFound)
			return
		}
		ch.WantKind(t, err, ch.ErrInvalid)
	}},
}

// 情境清單本身的檢查（不需要服務）：名稱不可重複，否則 CI 的 job 紀錄分不出是哪一個。本機與 lint 的預設建置
// 沒有 gateway tag，也靠它讓 scenarios 在預設建置裡有使用。
func TestScenarioNamesAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range scenarios {
		if s.Name == "" || seen[s.Name] {
			t.Errorf("情境名稱空白或重複：%q", s.Name)
		}
		seen[s.Name] = true
	}
	for _, s := range monolithScenarios {
		if s.Name == "" || seen[s.Name] {
			t.Errorf("情境名稱空白或重複：%q", s.Name)
		}
		seen[s.Name] = true
	}
}
