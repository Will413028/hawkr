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

// scenarios 是舊的 media-service 要通過的情境，單體的 Connect driver 跑同一份（B1-3d）：只涵蓋不需要物件儲存
// 的兩個端點，來源是 UploadSessionService.createSession／completeSession 與 MediaUploadValidator.validateDeclared。
// 各情境的期望值附 Java 的 file:line（services/java/media-service/src/main/java/.../media 底下）。
// 與舊實作刻意不同的地方，在 d.Variant() == Monolith 的分支寫決策代號與新行為（Connect driver 把 REST 的建立會話對應到
// BeginUpload、機敏的對應到 BeginSecureUpload，session ID 就是 staged token）。
var scenarios = []ch.Scenario[Driver]{
	{Name: "create public session returns presigned url and 30 minute ttl", Run: func(ctx context.Context, t ch.T, d Driver) {
		// UploadSessionService.java:99-106、application.yml media.upload.session-ttl-minutes=30
		s := ch.Must(t, ch.Ret(d.CreateUploadSession(ctx, ch.Buyer, UploadInput{
			Filename: "photo-" + ch.Nonce() + ".jpg", ContentType: "image/jpeg", FileSize: "1024",
		})))
		if d.Variant() == Monolith {
			// B1-D2：session ID 是不透明的 staged token，不再是 UUID。
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
			// B1-D1：單一從 BeginUpload 起算的期限 media.UploadTTL（1 小時，CARRYOVER 沿用盤點「暫存 TTL」列），取代 Java 的 30 分鐘。
			// driver 以回應的到期時間扣掉現在，只斷言落在 (0, ttl]，不依賴請求往返的耗時。
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
		if d.Variant() == Monolith {
			// B1-D2：不透明 token。B1-D17：BeginSecureUpload 不收宣告的型別，PDF 或圖片在第 0 段依 magic bytes 辨認（見單體情境
			// "declared pdf that is arbitrary bytes is rejected"、"real pdf bytes are accepted"）。
			if s.ID == "" {
				t.Errorf("staged token 是空的")
			}
		} else if !uuidRE.MatchString(s.ID) {
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
		if d.Variant() == Monolith {
			// B1-D2：staged token 就是建立時回的不透明 token（不再是完成時簽的 JWT）；CompleteUpload 不回預覽網址，client 以
			// GetUpload 輪詢，完成後狀態是 UPLOADED（等處理 job），READY 才交給擁有者轉正（B1-D1）。
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
		// UploadSessionService.java:131-135：第二次狀態不是 INIT。FAILED_PRECONDITION 在舊 gateway 塌成 code 500
		// （GrpcServiceHelper.mapStatusCode 的 default），沒有分類，所以只確認失敗；單體的分類是與舊實作的差異，寫進 CARRYOVER。
		s := ch.Must(t, ch.Ret(d.CreateUploadSession(ctx, ch.Buyer, UploadInput{
			Filename: "photo-" + ch.Nonce() + ".webp", ContentType: "image/webp", FileSize: "512",
		})))
		ch.Must(t, ch.Ret(d.CompleteUploadSession(ctx, ch.Buyer, s.ID)))
		_, err := d.CompleteUploadSession(ctx, ch.Buyer, s.ID)
		if d.Variant() == Monolith {
			// B1-D1 補充（token 與暫存列一對一、重入冪等）與 proto 的 CompleteUpload：已完成的重送一律成功，不重複排處理 job。
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
		// UploadSessionService.java:142-146
		s := ch.Must(t, ch.Ret(d.CreateUploadSession(ctx, ch.Buyer, UploadInput{
			Filename: "doc-" + ch.Nonce() + ".pdf", ContentType: "application/pdf", FileSize: "2048", IsSecure: true,
		})))
		_, err := d.CompleteUploadSession(ctx, ch.Buyer, s.ID)
		if d.Variant() == Monolith {
			// B1-D17：機敏檔走分段端點，會話在最後一段推進時轉 READY；對機敏 token 呼叫 CompleteUpload 沒有任何轉換（只有 PENDING 的
			// 公開會話會轉 UPLOADED），回成功而不是被拒。這是讀 Upload.Complete 與 MarkUploadUploaded 得到的現況，計畫沒有決定要不要拒絕。
			if err != nil {
				t.Fatalf("機敏 token 的 CompleteUpload 現況是成功且無作用，實際：%v", err)
			}
			return
		}
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
		if d.Variant() == Monolith {
			// B1-D2：token 是不透明字串，形狀不對的與不存在的同一個 not_found，不洩漏 token 是否存在（沒有 UUID 格式可驗）。
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
