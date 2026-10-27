package media

import (
	"context"
	"errors"
	"testing"

	pb "github.com/Will413028/hawkr/go-pkg/proto/media"
	"google.golang.org/grpc"
)

type captureReadStub struct {
	pb.MediaServiceClient
	got  *pb.ServerReadSecureAssetRequest
	code int32
}

func (s *captureReadStub) ServerReadSecureAsset(_ context.Context, in *pb.ServerReadSecureAssetRequest, _ ...grpc.CallOption) (*pb.ServerReadSecureAssetResponse, error) {
	s.got = in
	if s.code != 0 {
		return &pb.ServerReadSecureAssetResponse{Code: s.code, Message: "media says no"}, nil
	}
	return &pb.ServerReadSecureAssetResponse{FileData: []byte("x"), ContentType: "image/png", Filename: "a.png"}, nil
}

// 稅籍佐證的引用是 FinalizeEvidence 建的 (CERTIFICATION, profile id)；
// PDF 與解鎖證據則是 (InvoiceDocumentRefType, 自己的 id)。兩條不得互相借用。
func TestReadTaxEvidenceUsesCertificationRefType(t *testing.T) {
	stub := &captureReadStub{}
	c := &Client{stub: stub, deadline: DefaultDeadline}
	if _, _, _, err := c.ReadTaxEvidence(t.Context(), "secure/certification/a.png", 5000); err != nil {
		t.Fatal(err)
	}
	if stub.got.GetRefType() != RefType || stub.got.GetRefId() != "5000" || stub.got.GetObjectKey() != "secure/certification/a.png" {
		t.Errorf("request = %v", stub.got)
	}
	if RefType == InvoiceDocumentRefType {
		t.Fatal("兩個 refType 不得相同")
	}
}

func TestReadSecureAndUnlockEvidenceKeepInvoiceDocumentRefType(t *testing.T) {
	stub := &captureReadStub{}
	c := &Client{stub: stub, deadline: DefaultDeadline}
	if _, _, _, err := c.ReadSecure(t.Context(), "secure/invoice/x.pdf", "77"); err != nil {
		t.Fatal(err)
	}
	if stub.got.GetRefType() != InvoiceDocumentRefType || stub.got.GetRefId() != "77" {
		t.Errorf("ReadSecure request = %v", stub.got)
	}
	if _, _, _, err := c.ReadUnlockEvidence(t.Context(), "secure/invoice/e.png", 9001); err != nil {
		t.Fatal(err)
	}
	if stub.got.GetRefType() != InvoiceDocumentRefType || stub.got.GetRefId() != "9001" {
		t.Errorf("ReadUnlockEvidence request = %v", stub.got)
	}
}

// Java 的 MediaGrpcService 把 MediaServiceException 轉成 gRPC OK ＋ body code，
// 所以「資產不存在」只會以 code=404（查無與三元組不符同碼，另有 403）出現在回應裡。
func TestReadSecureMapsBodyCodes(t *testing.T) {
	for _, tc := range []struct {
		code int32
		want error
	}{
		{404, ErrAssetNotFound},
		{403, ErrAssetNotFound},
		{400, ErrAssetNotSecure},
	} {
		c := &Client{stub: &captureReadStub{code: tc.code}, deadline: DefaultDeadline}
		for name, call := range map[string]func() error{
			"ReadSecure": func() error { _, _, _, err := c.ReadSecure(t.Context(), "k", "1"); return err },
			"ReadTaxEvidence": func() error {
				_, _, _, err := c.ReadTaxEvidence(t.Context(), "k", 1)
				return err
			},
			"ReadUnlockEvidence": func() error {
				_, _, _, err := c.ReadUnlockEvidence(t.Context(), "k", 1)
				return err
			},
		} {
			if err := call(); !errors.Is(err, tc.want) {
				t.Errorf("%s code=%d: err=%v, want %v", name, tc.code, err, tc.want)
			}
		}
	}
	c := &Client{stub: &captureReadStub{code: 500}, deadline: DefaultDeadline}
	if _, _, _, err := c.ReadSecure(t.Context(), "k", "1"); err == nil || errors.Is(err, ErrAssetNotFound) || errors.Is(err, ErrAssetNotSecure) {
		t.Errorf("code=500 應是一般錯誤，got %v", err)
	}
}
