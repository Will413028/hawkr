package seller

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// 賣家讀自己申請的補充文件的端點（B1-D19）。為什麼是普通的 HTTP GET、不是 Connect，見 admin_document.go（API 設計規範（Connect）§十三）：
// 路由放在 /api/files/seller/ 底下，與 Connect 同一條入口中介鏈；授權是顧客 session 的帳號擁有目前的店家（模組做，
// 申請以帳號目前的店家為準），以業務資源定址：{document} 是文件在申請裡的位置（0 起），asset ID 不出現在 URL。回應是機敏檔案，不可快取。
// 檔案經 media 的機敏讀取串出；賣家讀自己的證件不寫讀取紀錄（讀取紀錄是後台管理員的稽核）。

// OwnDocumentRoute 是賣家讀自己補充文件端點的 ServeMux 樣式。
const OwnDocumentRoute = "GET /api/files/seller/kyc/{document}"

// ownDocumentURI 是 GetKYCApplication 回給賣家的下載網址（OwnDocumentRoute 的實例）。
func ownDocumentURI(position int) string {
	return "/api/files/seller/kyc/" + strconv.Itoa(position)
}

// OwnDocumentModule 是端點需要的模組方法。
type OwnDocumentModule interface {
	ReadOwnKYCDocument(context.Context, model.ReadOwnKYCDocumentInput) (model.FileContent, error)
}

// OwnDocuments 是賣家讀自己證件的 handler。
type OwnDocuments struct {
	m   OwnDocumentModule
	log *slog.Logger
}

// NewOwnDocuments 建立端點的 handler。
func NewOwnDocuments(m OwnDocumentModule, log *slog.Logger) *OwnDocuments {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &OwnDocuments{m: m, log: log}
}

func (h *OwnDocuments) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	accountID, ok := accountFile(w, r)
	if !ok {
		return
	}
	position, err := strconv.Atoi(r.PathValue("document"))
	if err != nil || position < 0 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	content, err := h.m.ReadOwnKYCDocument(r.Context(), model.ReadOwnKYCDocumentInput{AccountID: accountID, Position: position})
	if err != nil {
		fileFailure(w, r, h.log, err, "賣家自己的 KYC 證件")
		return
	}
	writeFile(w, content)
}

// accountFile 設好機敏檔回應的 header，取出呼叫的顧客帳號；沒有登入回 401、不是顧客帳號回 403。
func accountFile(w http.ResponseWriter, r *http.Request) (int64, bool) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	p, ok := rpc.PrincipalFrom(r.Context())
	switch {
	case !ok:
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return 0, false
	case p.Kind != identity.PrincipalAccount:
		http.Error(w, "forbidden", http.StatusForbidden)
		return 0, false
	}
	return p.ID, true
}
