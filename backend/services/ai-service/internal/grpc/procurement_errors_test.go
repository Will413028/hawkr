package grpc

import (
	"strings"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestProcurementStatus_AttachesBusinessErrorDetail(t *testing.T) {
	err := procurementStatus(codes.FailedPrecondition, "QUOTE_NOT_READY", "品項、配送城市、數量、到貨日皆備齊才能寄送報價單")

	st := status.Convert(err)
	if st.Code() != codes.FailedPrecondition {
		t.Fatalf("code = %v, want FailedPrecondition", st.Code())
	}
	var found *aipb.BusinessError
	for _, d := range st.Details() {
		if be, ok := d.(*aipb.BusinessError); ok {
			found = be
		}
	}
	if found == nil {
		t.Fatalf("no BusinessError detail attached")
	}
	if found.GetReason() != "QUOTE_NOT_READY" {
		t.Fatalf("reason = %q, want QUOTE_NOT_READY", found.GetReason())
	}
	if found.GetMessage() != "品項、配送城市、數量、到貨日皆備齊才能寄送報價單" {
		t.Fatalf("message = %q, unexpected", found.GetMessage())
	}
}

func TestSellerResponseErrorStatus_DraftCASConflictSpeaksToTheSupplier(t *testing.T) {
	draft := status.Convert(sellerResponseErrorStatus(repository.ErrSellerResponseDraftStale))
	submitted := status.Convert(sellerResponseErrorStatus(repository.ErrSellerResponseStale))

	if draft.Code() != codes.Aborted || submitted.Code() != codes.Aborted {
		t.Fatalf("codes = %v/%v, want Aborted for both", draft.Code(), submitted.Code())
	}

	draftMsg := businessErrorMessage(t, draft)
	submittedMsg := businessErrorMessage(t, submitted)

	if draftMsg == submittedMsg {
		t.Fatalf("草稿 CAS 衝突與正式回價 stale 共用同一句：%q。前者是供應商在存自己的草稿，"+
			"後者是買家看到的報價變了——同一句話對其中一方一定是錯的", draftMsg)
	}
	if strings.Contains(draftMsg, "供應商") {
		t.Fatalf("draft message = %q，這句是講給買家聽的（第三人稱指涉供應商），"+
			"但收到它的就是供應商本人", draftMsg)
	}
	if !strings.Contains(draftMsg, "草稿") {
		t.Fatalf("draft message = %q，沒有講出這是草稿衝突，供應商無從知道要重新載入什麼", draftMsg)
	}

	if reason := businessErrorReason(t, draft); reason != codemeta.ProcurementErrorReasonStaleSellerResponse.Value {
		t.Fatalf("reason = %q，機器可讀的碼刻意不變（新增碼值要跑 CodeMeta 五 repo 同步，"+
			"而語意本來就是同一個）", reason)
	}
}

func businessErrorDetail(t *testing.T, st *status.Status) *aipb.BusinessError {
	t.Helper()
	for _, d := range st.Details() {
		if be, ok := d.(*aipb.BusinessError); ok {
			return be
		}
	}
	t.Fatalf("no BusinessError detail attached")
	return nil
}

func businessErrorMessage(t *testing.T, st *status.Status) string {
	t.Helper()
	return businessErrorDetail(t, st).GetMessage()
}

func businessErrorReason(t *testing.T, st *status.Status) string {
	t.Helper()
	return businessErrorDetail(t, st).GetReason()
}
