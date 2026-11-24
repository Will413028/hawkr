package characterization

import (
	"bytes"
	"context"
	"net/http"
	"strconv"

	ch "github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
)

// monolithScenarios 是只有單體能跑的情境：機敏分段上傳（B1-D17）。CI 的 Java 沒有物件儲存（B1-D12），這些情境對它測不到，
// 規格來源是 test-map.md 第二節的 U 規則（附 Java 的 file:line）與計畫的 B1-3 驗收，各情境標出對應的規則或決策。
var monolithScenarios = []ch.Scenario[MonolithDriver]{
	{Name: "declared pdf that is arbitrary bytes is rejected", Run: func(ctx context.Context, t ch.T, d MonolithDriver) {
		// 宣告 PDF 的機敏會話，第 0 段卻是任意位元組：第 0 段依 magic bytes 辨認，辨認不出回 400、會話轉 REJECTED（B1-3 驗收；
		// Java 只信宣告的 Content-Type，MediaUploadValidator.java:128-133，沒有內容檢查，這是補強）。
		body := bytes.Repeat([]byte("not a pdf, just arbitrary bytes. "), 4)
		s := ch.Must(t, ch.Ret(d.CreateUploadSession(ctx, ch.Buyer, UploadInput{
			Filename: "doc-" + ch.Nonce() + ".pdf", ContentType: "application/pdf", FileSize: strconv.Itoa(len(body)), IsSecure: true,
		})))
		wantStatus(ctx, t, d, s.ID, 0, body, http.StatusBadRequest)
		// 被拒的會話不能再續傳：換成真的 PDF 也不收（412，會話已是 REJECTED）。
		pdf := []byte("%PDF-1.7\n" + string(body[:len(body)-9]))
		if code, err := d.PutSecureChunk(ctx, ch.Buyer, s.ID, 0, pdf); err != nil || code != http.StatusPreconditionFailed {
			t.Errorf("被拒絕的會話不該再收段：status = %d, err = %v，預期 412", code, err)
		}
	}},
	{Name: "real pdf bytes are accepted", Run: func(ctx context.Context, t ch.T, d MonolithDriver) {
		// B1-D17：PDF 的 magic bytes 是 %PDF-；單段的檔案第 0 段就是最後一段，回 204。
		pdf := []byte("%PDF-1.7\n" + ch.Nonce() + "\n%%EOF")
		s := ch.Must(t, ch.Ret(d.BeginSecureUpload(ctx, ch.Buyer, int64(len(pdf)))))
		if s.ChunkSize != media.SecureChunkSize || s.ChunkCount != 1 {
			t.Errorf("chunkSize = %d、chunkCount = %d，預期 %d 與 1", s.ChunkSize, s.ChunkCount, media.SecureChunkSize)
		}
		wantStatus(ctx, t, d, s.Token, 0, pdf, http.StatusNoContent)
	}},
	{Name: "oversize secure upload is rejected", Run: func(ctx context.Context, t ch.T, d MonolithDriver) {
		// U7（Java 不檢查上限，只有 multipart 的 100MB 與 gateway 的 20MiB）：上限單一來源 media.MaxSecureUploadSize，宣告超過回
		// invalid_argument，不建會話（缺陷 5.8）；0 與負數同樣被拒。
		for _, size := range []int64{media.MaxSecureUploadSize + 1, 0, -1} {
			_, err := d.BeginSecureUpload(ctx, ch.Buyer, size)
			ch.WantKind(t, err, ch.ErrInvalid)
		}
	}},
	{Name: "chunk over segment size is rejected", Run: func(ctx context.Context, t ch.T, d MonolithDriver) {
		// U7：實際位元組不信宣告。超過段大小的 body 被伺服器擋下（413），不讀完、不呼叫模組。
		s := ch.Must(t, ch.Ret(d.BeginSecureUpload(ctx, ch.Buyer, 2*media.SecureChunkSize)))
		body := append([]byte("%PDF-1.7\n"), make([]byte, media.SecureChunkSize)...)
		wantStatus(ctx, t, d, s.Token, 0, body, http.StatusRequestEntityTooLarge)
	}},
	{Name: "chunk length that differs from the declared size is rejected", Run: func(ctx context.Context, t ch.T, d MonolithDriver) {
		// U7：宣告大小與實際大小要相符（Java 不比對）。宣告 100 位元組的單段會話，送 50 位元組回 400。
		s := ch.Must(t, ch.Ret(d.BeginSecureUpload(ctx, ch.Buyer, 100)))
		wantStatus(ctx, t, d, s.Token, 0, append([]byte("%PDF-1.7\n"), make([]byte, 41)...), http.StatusBadRequest)
	}},
	{Name: "chunk of another accounts session is not found", Run: func(ctx context.Context, t ch.T, d MonolithDriver) {
		// U3：別人的會話與不存在的會話同一個 404。
		pdf := []byte("%PDF-1.7\n" + ch.Nonce())
		s := ch.Must(t, ch.Ret(d.BeginSecureUpload(ctx, ch.Buyer, int64(len(pdf)))))
		code, err := d.PutSecureChunk(ctx, ch.Buyer2, s.Token, 0, pdf)
		if err != nil || code != http.StatusNotFound {
			t.Errorf("別人的會話：status = %d, err = %v，預期 404", code, err)
		}
	}},
	{Name: "chunk index gap is a conflict", Run: func(ctx context.Context, t ch.T, d MonolithDriver) {
		// B1-D17：段必須依序，序號不是下一段回 409（並帶 Hawkr-Next-Index）。
		s := ch.Must(t, ch.Ret(d.BeginSecureUpload(ctx, ch.Buyer, 2*media.SecureChunkSize)))
		wantStatus(ctx, t, d, s.Token, 1, make([]byte, media.SecureChunkSize), http.StatusConflict)
	}},
}

func wantStatus(ctx context.Context, t ch.T, d MonolithDriver, token string, index int, body []byte, want int) {
	t.Helper()
	code, err := d.PutSecureChunk(ctx, ch.Buyer, token, index, body)
	if err != nil || code != want {
		t.Errorf("PUT 第 %d 段：status = %d, err = %v，預期 %d", index, code, err, want)
	}
}
