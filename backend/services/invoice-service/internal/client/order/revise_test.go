package order

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	pb "github.com/Will413028/hawkr/go-pkg/proto/order"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func reviseInput() domain.ReviseSnapshotInput {
	return domain.ReviseSnapshotInput{
		OrderNo:                 "O20260831001",
		ExpectedSnapshotVersion: 1,
		InvoiceKind:             "B2B",
		CarrierSelectionMode:    "NONE",
		BuyerTaxID:              "12345678",
		BuyerTitle:              "範例生鮮股份有限公司",
		IdempotencyKey:          "correction-7788",
	}
}

func TestReviseOrderInvoiceSnapshot_Success(t *testing.T) {
	srv := &fakeOrder{reviseResp: &pb.ReviseOrderInvoiceSnapshotInternalResponse{
		Code: 0, Message: "success", SnapshotVersion: "2",
	}}
	c := newTestClient(t, srv, nil, time.Second)

	got, err := c.ReviseOrderInvoiceSnapshot(context.Background(), reviseInput())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 2 {
		t.Fatalf("snapshot version = %d, want 2", got)
	}

	req := srv.gotRevise
	if req.GetExpectedSnapshotVersion() != "1" {
		t.Errorf("expected_snapshot_version = %q, want \"1\"（少了它，並行更正是後寫的靜默覆蓋前者）",
			req.GetExpectedSnapshotVersion())
	}
	if req.GetIdempotencyKey() != "correction-7788" {
		t.Errorf("idempotency_key = %q, want correction-7788", req.GetIdempotencyKey())
	}
	if req.GetBuyerTaxId() != "12345678" || req.GetInvoiceKind() != "B2B" {
		t.Errorf("內容軸欄位沒送對：tax_id=%q kind=%q", req.GetBuyerTaxId(), req.GetInvoiceKind())
	}
	if req.GetBuyerInvoicePreferenceId() != "" {
		t.Errorf("buyer_invoice_preference_id = %q, want 空字串（0 ⇒ 未提供）",
			req.GetBuyerInvoicePreferenceId())
	}
}

func TestReviseOrderInvoiceSnapshot_VersionConflictIsDistinguishable(t *testing.T) {
	srv := &fakeOrder{reviseResp: &pb.ReviseOrderInvoiceSnapshotInternalResponse{
		Code: 9308, Message: "發票資料已被更新，請重新整理後再試",
	}}
	c := newTestClient(t, srv, nil, time.Second)

	_, err := c.ReviseOrderInvoiceSnapshot(context.Background(), reviseInput())
	if !errors.Is(err, ErrOrderSnapshotVersionConflict) {
		t.Fatalf("error = %v, want ErrOrderSnapshotVersionConflict（呼叫端要靠它決定重讀重試，"+
			"混進一般錯誤就只能整筆失敗）", err)
	}
}

func TestReviseOrderInvoiceSnapshot_CodeZeroWithEmptyVersionIsError(t *testing.T) {
	srv := &fakeOrder{reviseResp: &pb.ReviseOrderInvoiceSnapshotInternalResponse{
		Code: 0, Message: "success", SnapshotVersion: "",
	}}
	c := newTestClient(t, srv, nil, time.Second)

	_, err := c.ReviseOrderInvoiceSnapshot(context.Background(), reviseInput())
	if err == nil {
		t.Fatal("code=0 ＋ 空 snapshot_version 必須是錯誤")
	}
	if !strings.Contains(err.Error(), "code=0 with empty snapshot_version") {
		t.Fatalf("error = %v，want 明確指出對端回了 code=0 卻沒給版本", err)
	}
}

func TestReviseOrderInvoiceSnapshot_NotNewerVersionIsError(t *testing.T) {
	for _, v := range []string{"1", "0"} {
		srv := &fakeOrder{reviseResp: &pb.ReviseOrderInvoiceSnapshotInternalResponse{
			Code: 0, Message: "success", SnapshotVersion: v,
		}}
		c := newTestClient(t, srv, nil, time.Second)

		if _, err := c.ReviseOrderInvoiceSnapshot(context.Background(), reviseInput()); err == nil {
			t.Fatalf("snapshot_version=%s（不新於 expected=1）必須是錯誤", v)
		}
	}
}

func TestReviseOrderInvoiceSnapshot_EmptyIdempotencyKeyNeverReachesWire(t *testing.T) {
	srv := &fakeOrder{reviseResp: &pb.ReviseOrderInvoiceSnapshotInternalResponse{
		Code: 0, SnapshotVersion: "2",
	}}
	c := newTestClient(t, srv, nil, time.Second)

	in := reviseInput()
	in.IdempotencyKey = ""
	if _, err := c.ReviseOrderInvoiceSnapshot(context.Background(), in); err == nil {
		t.Fatal("空 idempotency_key 必須是錯誤")
	}
	if srv.reviseCalls != 0 {
		t.Errorf("reviseCalls = %d, want 0（不該送出去）", srv.reviseCalls)
	}
}

func TestReviseOrderInvoiceSnapshot_NotFoundIsDistinguishable(t *testing.T) {
	srv := &fakeOrder{reviseErr: status.Error(codes.NotFound, "no snapshot")}
	c := newTestClient(t, srv, nil, time.Second)

	_, err := c.ReviseOrderInvoiceSnapshot(context.Background(), reviseInput())
	if !errors.Is(err, ErrOrderSnapshotNotFound) {
		t.Fatalf("error = %v, want ErrOrderSnapshotNotFound", err)
	}
}
