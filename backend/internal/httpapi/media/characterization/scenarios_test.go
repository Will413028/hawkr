package characterization

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	ch "github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
)

var uuidRE = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// scenarios 是舊的 media-service 要通過的情境（B1-3 起單體的 Connect driver 也跑同一份）：只涵蓋不需要物件儲存
// 的兩個端點，來源是 UploadSessionService.createSession／completeSession 與 MediaUploadValidator.validateDeclared。
// 各情境的期望值附 Java 的 file:line（services/java/media-service/src/main/java/.../media 底下）。
var scenarios = []ch.Scenario[Driver]{
	{Name: "create public session returns presigned url and 30 minute ttl", Run: func(ctx context.Context, t ch.T, d Driver) {
		// UploadSessionService.java:99-106、application.yml media.upload.session-ttl-minutes=30
		s := ch.Must(t, ch.Ret(d.CreateUploadSession(ctx, ch.Buyer, UploadInput{
			Filename: "photo-" + ch.Nonce() + ".jpg", ContentType: "image/jpeg", FileSize: "1024",
		})))
		if !uuidRE.MatchString(s.ID) {
			t.Errorf("sessionId = %q，預期 UUID", s.ID)
		}
		if s.UploadURL == "" {
			t.Errorf("公開上傳的 uploadUrl 是空的")
		}
		if s.ExpiresIn != "1800" {
			t.Errorf("expiresIn = %q，預期 1800", s.ExpiresIn)
		}
		if s.IsSecure {
			t.Errorf("isSecure = true，預期 false")
		}
	}},
	{Name: "create secure session has no upload url and allows pdf", Run: func(ctx context.Context, t ch.T, d Driver) {
		// UploadSessionService.java:98-106（機敏不回 URL）、MediaUploadValidator.java:128-133（PDF 僅機敏）
		s := ch.Must(t, ch.Ret(d.CreateUploadSession(ctx, ch.Buyer, UploadInput{
			Filename: "doc-" + ch.Nonce() + ".pdf", ContentType: "application/pdf", FileSize: "2048", IsSecure: true,
		})))
		if s.UploadURL != "" {
			t.Errorf("機敏上傳不該有 uploadUrl：%q", s.UploadURL)
		}
		if !s.IsSecure {
			t.Errorf("isSecure = false，預期 true")
		}
		if !uuidRE.MatchString(s.ID) {
			t.Errorf("sessionId = %q，預期 UUID", s.ID)
		}
	}},
	{Name: "create rejects pdf for public upload", Run: func(ctx context.Context, t ch.T, d Driver) {
		// MediaUploadValidator.java:56-58
		_, err := d.CreateUploadSession(ctx, ch.Buyer, UploadInput{Filename: "a.pdf", ContentType: "application/pdf", FileSize: "100"})
		ch.WantKind(t, err, ch.ErrInvalid)
	}},
	{Name: "create rejects unsupported mime", Run: func(ctx context.Context, t ch.T, d Driver) {
		// MediaUploadValidator.java:56-58、:30-31 白名單
		_, err := d.CreateUploadSession(ctx, ch.Buyer, UploadInput{Filename: "a.svg", ContentType: "image/svg+xml", FileSize: "100"})
		ch.WantKind(t, err, ch.ErrInvalid)
	}},
	{Name: "create rejects declared size over limit", Run: func(ctx context.Context, t ch.T, d Driver) {
		// MediaUploadValidator.java:63-66，一般圖片上限 10485760（application.yml hawkr.media.upload.max-image-size-bytes）
		_, err := d.CreateUploadSession(ctx, ch.Buyer, UploadInput{Filename: "a.jpg", ContentType: "image/jpeg", FileSize: "10485761"})
		ch.WantKind(t, err, ch.ErrInvalid)
	}},
	{Name: "create rejects non positive size", Run: func(ctx context.Context, t ch.T, d Driver) {
		// MediaGrpcService.java:407-414
		_, err := d.CreateUploadSession(ctx, ch.Buyer, UploadInput{Filename: "a.jpg", ContentType: "image/jpeg", FileSize: "0"})
		ch.WantKind(t, err, ch.ErrInvalid)
	}},
	{Name: "complete returns staged token and preview url", Run: func(ctx context.Context, t ch.T, d Driver) {
		// UploadSessionService.java:159-179（不回查物件；R2 未配置時 previewUrl 是模擬 URL，R2StorageService.java:151-154）
		s := ch.Must(t, ch.Ret(d.CreateUploadSession(ctx, ch.Buyer, UploadInput{
			Filename: "photo-" + ch.Nonce() + ".png", ContentType: "image/png", FileSize: "4096",
		})))
		c := ch.Must(t, ch.Ret(d.CompleteUploadSession(ctx, ch.Buyer, s.ID)))
		if strings.Count(c.StagedToken, ".") != 2 {
			t.Errorf("stagedToken = %q，預期 JWT 三段", c.StagedToken)
		}
		if c.PreviewURL == "" {
			t.Errorf("previewUrl 是空的")
		}
	}},
	{Name: "complete twice fails", Run: func(ctx context.Context, t ch.T, d Driver) {
		// UploadSessionService.java:131-135：第二次狀態不是 INIT。FAILED_PRECONDITION 在舊 gateway 塌成 code 500
		// （GrpcServiceHelper.mapStatusCode 的 default），沒有分類，所以只確認失敗；單體的分類是與舊實作的差異，寫進 CARRYOVER。
		s := ch.Must(t, ch.Ret(d.CreateUploadSession(ctx, ch.Buyer, UploadInput{
			Filename: "photo-" + ch.Nonce() + ".webp", ContentType: "image/webp", FileSize: "512",
		})))
		ch.Must(t, ch.Ret(d.CompleteUploadSession(ctx, ch.Buyer, s.ID)))
		_, err := d.CompleteUploadSession(ctx, ch.Buyer, s.ID)
		if !errors.Is(err, ch.ErrRejected) {
			t.Fatalf("第二次完成同一個會話應該被服務以業務層 envelope 拒絕，實際：%v", err)
		}
	}},
	{Name: "complete rejects secure session", Run: func(ctx context.Context, t ch.T, d Driver) {
		// UploadSessionService.java:142-146
		s := ch.Must(t, ch.Ret(d.CreateUploadSession(ctx, ch.Buyer, UploadInput{
			Filename: "doc-" + ch.Nonce() + ".pdf", ContentType: "application/pdf", FileSize: "2048", IsSecure: true,
		})))
		_, err := d.CompleteUploadSession(ctx, ch.Buyer, s.ID)
		ch.WantKind(t, err, ch.ErrInvalid)
	}},
	{Name: "complete of another accounts session is not found", Run: func(ctx context.Context, t ch.T, d Driver) {
		// UploadSessionService.java:126-129（selectByIdAndAccountId）
		s := ch.Must(t, ch.Ret(d.CreateUploadSession(ctx, ch.Buyer, UploadInput{
			Filename: "photo-" + ch.Nonce() + ".gif", ContentType: "image/gif", FileSize: "256",
		})))
		_, err := d.CompleteUploadSession(ctx, ch.Buyer2, s.ID)
		ch.WantKind(t, err, ch.ErrNotFound)
	}},
	{Name: "complete rejects malformed session id", Run: func(ctx context.Context, t ch.T, d Driver) {
		// MediaGrpcService.java:493-504
		_, err := d.CompleteUploadSession(ctx, ch.Buyer, "not-a-uuid")
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
}
