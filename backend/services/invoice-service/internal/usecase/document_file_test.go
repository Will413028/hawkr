package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// #3008：修好取檔而沒補歸屬判定，會從「都拿不到」變成「誰都拿得到」。
//
// 🔴 這一組測試的核心不是「拿得到」，而是**拿不到的那幾格長什麼樣**：
// 越權與不存在必須逐字相同（同一個業務碼、同一段訊息），否則這條路由就是一台
// 可枚舉的 oracle——攻擊者拿一串 document_id 打過去，兩種回應的差別就是
// 「這張發票存在」。發票 PDF 上印著完整的買受人姓名、統編與金額。

type fakeSecureReader struct {
	// wantKey／wantRefID 是這個替身**唯一接受**的參數組合。
	// 無條件回 bytes 的替身會讓「有沒有拿對 key」這件事零鑑別力。
	wantKey   string
	wantRefID string
	data      []byte
	calls     int
}

func (f *fakeSecureReader) ReadSecure(_ context.Context, objectKey, refID string) ([]byte, string, string, error) {
	f.calls++
	if objectKey != f.wantKey || refID != f.wantRefID {
		return nil, "", "", errors.New("media: 不是這個替身接受的 (object_key, ref_id)")
	}
	return f.data, "application/pdf", "invoice-AA10000444.pdf", nil
}

func newFileFixture(t *testing.T) (*DocumentFile, *fakeDocRepo, *fakeSecureReader) {
	t.Helper()
	buyerID := int64(2623900520)
	key := "secure/invoice/2685914682/202609/7500466617315229696.pdf"
	no := "AA10000444"
	issuedAt := time.Date(2026, 9, 1, 8, 22, 0, 0, time.UTC)

	docs := &fakeDocRepo{
		stored: domain.Document{
			ID:                7500466617315229696,
			Status:            "ISSUED",
			RecipientRefID:    &buyerID,
			ProviderInvoiceNo: &no,
			ProviderIssuedAt:  &issuedAt,
			PdfObjectKey:      &key,
			RecipientTaxID:    domain.B2CBuyerIdentifier,
		},
		ownerStoreID: 2685914682,
	}
	media := &fakeSecureReader{
		wantKey:   key,
		wantRefID: "7500466617315229696",
		data:      []byte("%PDF-1.4 fake"),
	}
	u := NewDocumentFile(docs, media, zap.NewNop())
	u.now = func() time.Time { return time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC) }
	return u, docs, media
}

func TestDocumentFile_OwnershipPredicates(t *testing.T) {
	const docID = int64(7500466617315229696)

	cases := []struct {
		name    string
		req     FileRequest
		wantErr bool
	}{
		// ---- 陽性：三個合法讀者面各自拿得到 ----
		{"買家本人", FileRequest{Actor: FileActorBuyer, DocumentID: docID, BuyerID: 2623900520}, false},
		{"開票賣家", FileRequest{Actor: FileActorSeller, DocumentID: docID, StoreID: 2685914682}, false},
		{"ADM（無 ownership 概念，授權在 gateway）", FileRequest{Actor: FileActorAdmin, DocumentID: docID}, false},

		// ---- 陰性：越權 ----
		{"別的買家", FileRequest{Actor: FileActorBuyer, DocumentID: docID, BuyerID: 2688637900}, true},
		{"別的賣家", FileRequest{Actor: FileActorSeller, DocumentID: docID, StoreID: 2632049831}, true},

		// ---- 陰性：fail-closed。gateway 的 header 注入失敗時這兩個值會是 0，
		//      而「沒有述詞就不篩」在這裡等於把每一張發票開放給任何登入者。----
		{"買家面缺 x-buyer-id", FileRequest{Actor: FileActorBuyer, DocumentID: docID}, true},
		{"賣家面缺 x-store-id", FileRequest{Actor: FileActorSeller, DocumentID: docID}, true},

		{"不存在的 document", FileRequest{Actor: FileActorBuyer, DocumentID: 1, BuyerID: 2623900520}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u, _, _ := newFileFixture(t)
			_, err := u.Resolve(context.Background(), tc.req)
			if tc.wantErr {
				if err == nil {
					t.Fatal("預期被擋下，實際放行 —— 這條路由會變成「誰都拿得到」")
				}
				if !isBizCode(err, bizcode.InvoiceDocumentNotFound) {
					t.Fatalf("錯誤碼 = %v，want 9301。403 不算通過：可分辨即為可枚舉 oracle", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("合法讀者被擋下：%v", err)
			}
		})
	}
}

// 越權與不存在必須**逐字相同**。分開斷言而不是併進上表：上表只驗「都被擋下」，
// 而本票的驗收條件是「不可區分」——兩者差一個字就破功。
func TestDocumentFile_UnauthorizedIsIndistinguishableFromMissing(t *testing.T) {
	u, _, _ := newFileFixture(t)
	ctx := context.Background()

	_, unauthorized := u.Resolve(ctx, FileRequest{
		Actor: FileActorBuyer, DocumentID: 7500466617315229696, BuyerID: 2688637900})
	_, missing := u.Resolve(ctx, FileRequest{
		Actor: FileActorBuyer, DocumentID: 1, BuyerID: 2623900520})

	if unauthorized == nil || missing == nil {
		t.Fatal("兩者都應該有錯誤")
	}
	if unauthorized.Error() != missing.Error() {
		t.Fatalf("越權與不存在可區分：\n越權   = %q\n不存在 = %q", unauthorized, missing)
	}
}

// 沒有 pdf_object_key 的文件不得回 bytes，要回對應的業務碼（§6.1.4 第 4 條）。
// 9312 是**正常業務分支**不是錯誤：載具發票中獎前我方拿不到 PDF。
func TestDocumentFile_UnavailableReturnsBusinessCode(t *testing.T) {
	buyerID := int64(2623900520)
	carrier := "3J0002"
	issuedAt := time.Date(2026, 9, 1, 8, 22, 0, 0, time.UTC)

	t.Run("載具未中獎 → 9312", func(t *testing.T) {
		docs := &fakeDocRepo{stored: domain.Document{
			ID: 7, Status: "ISSUED", RecipientRefID: &buyerID,
			CarrierType: &carrier, ProviderIssuedAt: &issuedAt,
		}}
		u := NewDocumentFile(docs, &fakeSecureReader{}, zap.NewNop())
		u.now = func() time.Time { return time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC) }
		_, err := u.Resolve(context.Background(), FileRequest{
			Actor: FileActorBuyer, DocumentID: 7, BuyerID: buyerID})
		if !isBizCode(err, bizcode.InvoicePdfUnavailableCarrier) {
			t.Fatalf("err = %v, want 9312", err)
		}
	})

	t.Run("逾 AMEGO 查詢期限 → 9313", func(t *testing.T) {
		old := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		docs := &fakeDocRepo{stored: domain.Document{
			ID: 7, Status: "ISSUED", RecipientRefID: &buyerID, ProviderIssuedAt: &old,
		}}
		u := NewDocumentFile(docs, &fakeSecureReader{}, zap.NewNop())
		u.now = func() time.Time { return time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC) }
		_, err := u.Resolve(context.Background(), FileRequest{
			Actor: FileActorBuyer, DocumentID: 7, BuyerID: buyerID})
		if !isBizCode(err, bizcode.InvoicePdfExpired) {
			t.Fatalf("err = %v, want 9313", err)
		}
	})
}

// style 白名單（§6.1.4 鐵則 3）。AMEGO 對 download_style 無值域驗證 ⇒ 我方必須自驗。
func TestDocumentFile_StyleWhitelist(t *testing.T) {
	u, _, _ := newFileFixture(t)
	req := FileRequest{Actor: FileActorBuyer, DocumentID: 7500466617315229696, BuyerID: 2623900520}

	for _, s := range []string{"0", "1", "2", "3", "5"} {
		r := req
		r.Style = s
		if _, err := u.Resolve(context.Background(), r); err != nil {
			t.Errorf("style=%q 被擋下：%v", s, err)
		}
	}
	for _, s := range []string{"4", "9", "-1", "abc"} {
		r := req
		r.Style = s
		if _, err := u.Resolve(context.Background(), r); !errors.Is(err, ErrInvalidFileStyle) {
			t.Errorf("style=%q 被放行（err=%v）—— AMEGO 不會幫你擋", s, err)
		}
	}
	// 沒帶 style ⇒ 依統編推導（B2C 只有 0）。
	got, err := u.Resolve(context.Background(), req)
	if err != nil {
		t.Fatalf("預設 style 解析失敗：%v", err)
	}
	if got.Style != "0" {
		t.Errorf("B2C 的預設 style = %q, want \"0\"（沒打統編的發票只有 0，其餘 AMEGO 回 code 53）", got.Style)
	}
}

// Read 必須拿**這份文件自己的** (object_key, document_id) 去換 bytes。
func TestDocumentFile_ReadPassesOwnKeyAndRefID(t *testing.T) {
	u, _, media := newFileFixture(t)
	_, data, contentType, err := u.Read(context.Background(), FileRequest{
		Actor: FileActorBuyer, DocumentID: 7500466617315229696, BuyerID: 2623900520})
	if err != nil {
		t.Fatalf("Read 失敗：%v", err)
	}
	if string(data) != "%PDF-1.4 fake" || contentType != "application/pdf" {
		t.Fatalf("bytes/contentType 不符：%q / %q", data, contentType)
	}
	if media.calls != 1 {
		t.Errorf("ReadSecure 呼叫 %d 次, want 1", media.calls)
	}
	// 被擋下的請求不得打到 media：那代表授權判定發生在取檔之後。
	media.calls = 0
	_, _, _, err = u.Read(context.Background(), FileRequest{
		Actor: FileActorBuyer, DocumentID: 7500466617315229696, BuyerID: 2688637900})
	if err == nil {
		t.Fatal("越權請求竟然成功")
	}
	if media.calls != 0 {
		t.Errorf("越權請求仍打了 media %d 次 —— 授權判定發生在取檔之後", media.calls)
	}
}
