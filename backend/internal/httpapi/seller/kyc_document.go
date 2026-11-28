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

// OwnDocumentRoute 是賣家讀自己補充文件端點的 ServeMux 樣式。
const OwnDocumentRoute = "GET /api/files/seller/kyc/{document}"

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
	writeFile(w, r, h.log, content)
}

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
