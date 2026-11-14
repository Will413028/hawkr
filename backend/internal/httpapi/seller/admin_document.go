package seller

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"uuid"

	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// 後台讀 KYC 公司補充文件的端點（Stripe 計畫 D36：補充文件不是證件，有後台的 KYC 讀取權限就能讀，不要理由與限時授權）。
//
// 為什麼是普通的 HTTP GET，不是 Connect：回應是圖片本身（瀏覽器的 <img> 與下載直接顯示），Connect 的 unary 只能回訊息、要自己
// 包 bytes 欄位（base64 膨脹約三分之一，也進不了瀏覽器的圖片快取與串流）。API 設計規範（Connect）§「檔案回應」規則：
// 不經 Connect 的二進位回應放在 /api/files/<使用者類別>/ 底下，與 Connect 同一條入口中介鏈（tracing、CSRF、
// withSession、限流），授權由擁有的 handler 做，機敏檔案 no-store，以業務資源定址、不以 asset ID。驗證與後台的 Connect 方法相同
// （Authorization: Bearer 的 Access JWT 或本機的 dev token），授權是這條路由的權限 adminperm.SellerKYCDocumentsGet（server.go 的路由宣告）。
// 每次讀取寫一列 seller.kyc_document_accesses（誰、哪一張、何時），在送出內容之前。
//
// 以 (申請, 位置) 定址。

// DocumentRoute 是補充文件端點的 ServeMux 樣式。
const DocumentRoute = "GET /api/files/admin/seller/kyc_applications/{application_id}/documents/{position}"

// documentURI 是 GetKYCApplication 回給管理員的檢視網址（DocumentRoute 的實例）。
func documentURI(applicationID uuid.UUID, position int) string {
	return "/api/files/admin/seller/kyc_applications/" + applicationID.String() + "/documents/" + strconv.Itoa(position)
}

// DocumentModule 是端點需要的模組方法。
type DocumentModule interface {
	ReadKYCDocument(context.Context, model.ReadKYCDocumentInput) (model.FileContent, error)
}

// AdminDocuments 是證件圖片端點的 handler。
type AdminDocuments struct {
	m   DocumentModule
	log *slog.Logger
}

// NewAdminDocuments 建立端點的 handler。
func NewAdminDocuments(m DocumentModule, log *slog.Logger) *AdminDocuments {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &AdminDocuments{m: m, log: log}
}

func (h *AdminDocuments) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	adminID, ok := adminFile(w, r)
	if !ok {
		return
	}
	in, found := documentRequest(r)
	if !found {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	in.AdminID = adminID
	content, err := h.m.ReadKYCDocument(r.Context(), in)
	if err != nil {
		fileFailure(w, r, h.log, err, "KYC 證件")
		return
	}
	writeFile(w, content)
}

// documentRequest 解析路徑；格式不對的位置與一般的找不到同一種回應（404）。
func documentRequest(r *http.Request) (model.ReadKYCDocumentInput, bool) {
	id, err := uuid.Parse(r.PathValue("application_id"))
	if err != nil {
		return model.ReadKYCDocumentInput{}, false
	}
	position, err := strconv.Atoi(r.PathValue("position"))
	if err != nil || position < 0 {
		return model.ReadKYCDocumentInput{}, false
	}
	return model.ReadKYCDocumentInput{ApplicationID: id, Position: position}, true
}
