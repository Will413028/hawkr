package mediaservice

import (
	"cmp"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"

	mediapb "github.com/Will413028/hawkr/backend/internal/legacy/gen/media"
	"github.com/Will413028/hawkr/backend/internal/legacy/gen/media/mediaconnect"
	"github.com/Will413028/hawkr/backend/internal/legacy/grpcclient"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

const secret = "test-secret-at-least-32-bytes-long!!"

// fakeMediaService 是以同一份 proto 生成的 handler 假扮的 media-service，走 gRPC 協定與 h2c，
// 記下每個呼叫的 procedure 與 header。
type fakeMediaService struct {
	mediaconnect.UnimplementedMediaServiceHandler

	mu           sync.Mutex
	calls        []call
	finalizeCode int32
	finalizeMsg  string
	// secure 是 FinalizeImages 回報的 is_secure；readCode、readData 是 ServerReadSecureAsset 的回答。
	secure       bool
	readCode     int32
	readData     []byte
	linkFailures int
	linked       *mediapb.LinkImagesRequest
	unlinked     *mediapb.UnlinkImagesRequest
}

type call struct {
	procedure string
	header    http.Header
}

func (f *fakeMediaService) record(spec connect.Spec, h http.Header) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call{spec.Procedure, h.Clone()})
}

func (f *fakeMediaService) FinalizeImages(_ context.Context, req *connect.Request[mediapb.FinalizeImagesRequest]) (*connect.Response[mediapb.FinalizeImagesResponse], error) {
	f.record(req.Spec(), req.Header())
	if f.finalizeCode != 0 {
		return connect.NewResponse(&mediapb.FinalizeImagesResponse{Code: f.finalizeCode, Message: cmp.Or(f.finalizeMsg, "staged token 已過期")}), nil
	}
	tok := req.Msg.GetStagedTokens()[0]
	return connect.NewResponse(&mediapb.FinalizeImagesResponse{Assets: []*mediapb.FinalizedAsset{{
		StagedToken: tok, AssetId: "asset-" + tok, ObjectKey: "public/" + req.Msg.GetRefType() + "/" + tok + ".jpg", IsSecure: f.secure,
	}}}), nil
}

func (f *fakeMediaService) ServerReadSecureAsset(_ context.Context, req *connect.Request[mediapb.ServerReadSecureAssetRequest]) (*connect.Response[mediapb.ServerReadSecureAssetResponse], error) {
	f.record(req.Spec(), req.Header())
	if f.readCode != 0 {
		return connect.NewResponse(&mediapb.ServerReadSecureAssetResponse{Code: f.readCode, Message: "media says no"}), nil
	}
	return connect.NewResponse(&mediapb.ServerReadSecureAssetResponse{FileData: f.readData, ContentType: "image/png", Filename: req.Msg.GetObjectKey()}), nil
}

func (f *fakeMediaService) LinkImages(_ context.Context, req *connect.Request[mediapb.LinkImagesRequest]) (*connect.Response[mediapb.LinkImagesResponse], error) {
	f.record(req.Spec(), req.Header())
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.linkFailures > 0 {
		f.linkFailures--
		return nil, connect.NewError(connect.CodeUnavailable, nil)
	}
	f.linked = req.Msg
	return connect.NewResponse(&mediapb.LinkImagesResponse{LinkedCount: "1"}), nil
}

func (f *fakeMediaService) UnlinkImages(_ context.Context, req *connect.Request[mediapb.UnlinkImagesRequest]) (*connect.Response[mediapb.UnlinkImagesResponse], error) {
	f.record(req.Spec(), req.Header())
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.linkFailures > 0 {
		f.linkFailures--
		return nil, connect.NewError(connect.CodeUnavailable, nil)
	}
	f.unlinked = req.Msg
	return connect.NewResponse(&mediapb.UnlinkImagesResponse{UnlinkedCount: "1"}), nil
}

func start(t *testing.T, f *fakeMediaService) *Client {
	t.Helper()
	_, h := mediaconnect.NewMediaServiceHandler(f)
	srv := httptest.NewUnstartedServer(h)
	srv.Config.Protocols = new(http.Protocols)
	srv.Config.Protocols.SetUnencryptedHTTP2(true)
	srv.Start()
	t.Cleanup(srv.Close)
	c, err := New(Config{Addr: strings.TrimPrefix(srv.URL, "http://"), Secret: secret})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

var admin = media.Uploader{ID: 9001, Admin: true}

// 簽章照 Java 的 InternalAuthVerifier 重算：canonical 的最後一段是同一個請求的 x-account-id。
func verifySignature(t *testing.T, h http.Header) {
	t.Helper()
	if h.Get("x-internal-service") != grpcclient.CallerName {
		t.Fatalf("x-internal-service = %q", h.Get("x-internal-service"))
	}
	ts := h.Get("x-internal-ts")
	sec, err := strconv.ParseInt(ts, 10, 64)
	if err != nil || time.Since(time.Unix(sec, 0)).Abs() > time.Minute {
		t.Fatalf("x-internal-ts = %q", ts)
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(grpcclient.CallerName + "\n" + ts + "\n" + h.Get("x-internal-nonce") + "\n" + h.Get("x-account-id")))
	if want := base64.StdEncoding.EncodeToString(mac.Sum(nil)); h.Get("x-internal-sig") != want {
		t.Fatalf("簽章不符：%q", h.Get("x-internal-sig"))
	}
}

func TestFinalizeSignsCarriesTheUploaderAndCreatesNoReference(t *testing.T) {
	f := &fakeMediaService{}
	c := start(t, f)
	a, err := c.Finalize(context.Background(), admin, "tok", media.RefTypeBanner)
	if err != nil {
		t.Fatal(err)
	}
	if a != (media.Asset{ID: "asset-tok", ObjectKey: "public/7109/tok.jpg", Secure: false}) {
		t.Fatalf("資產 = %+v", a)
	}
	if len(f.calls) != 1 || f.calls[0].procedure != mediaconnect.MediaServiceFinalizeImagesProcedure {
		t.Fatalf("轉正只能呼叫 FinalizeImages：%+v", f.calls)
	}
	h := f.calls[0].header
	if h.Get("x-account-id") != "9001" || h.Get("x-account-type") != "4" {
		t.Errorf("身分 = %q／%q", h.Get("x-account-id"), h.Get("x-account-type"))
	}
	verifySignature(t, h)
}

// 參數在本地檢查過之後，遠端的 400、404 一律是 token 的問題（不解析訊息）：只帶 ErrStagedTokenRejected，遠端的碼與訊息留在文字（進
// log），不放進給使用者的文字。
func TestRemoteRejectionsAreTokenProblemsWhateverTheMessage(t *testing.T) {
	for _, code := range []int32{400, 404} {
		for _, msg := range []string{"", "staged token 已過期", "無效的 ref_type: x", "staged_tokens 不可為空"} {
			f := &fakeMediaService{finalizeCode: code, finalizeMsg: msg}
			_, err := start(t, f).Finalize(context.Background(), admin, "tok", media.RefTypeBanner)
			if !errors.Is(err, media.ErrStagedTokenRejected) || !strings.Contains(err.Error(), fmt.Sprintf("code=%d", code)) {
				t.Fatalf("code %d／%q：err = %v，預期 ErrStagedTokenRejected 並帶遠端的碼", code, msg, err)
			}
			var ae *apperr.Error
			if errors.As(err, &ae) {
				t.Fatalf("code %d：不該帶 apperr 分類（由 FinalizeField 決定給使用者的文字）：%v", code, err)
			}
		}
	}
	f := &fakeMediaService{finalizeCode: 500}
	_, err := start(t, f).Finalize(context.Background(), admin, "tok", media.RefTypeBanner)
	if err == nil || errors.Is(err, media.ErrStagedTokenRejected) || apperr.KindOf(err) != apperr.KindInternal {
		t.Fatalf("media-service 內部錯誤：err = %v，預期未分類", err)
	}
}

// 呼叫方的參數錯誤（缺上傳者、無效的 ref_type、空白的 token）在本地擋下，不打 media-service，是未分類的程式錯誤。
func TestFinalizeCallerBugsAreCaughtLocally(t *testing.T) {
	for name, c := range map[string]struct {
		u       media.Uploader
		token   string
		refType string
	}{
		"缺上傳者":         {media.Uploader{}, "tok", media.RefTypeBanner},
		"無效的 ref_type": {admin, "tok", "x"},
		"空白的 token":    {admin, "  ", media.RefTypeBanner},
	} {
		f := &fakeMediaService{}
		_, err := start(t, f).Finalize(context.Background(), c.u, c.token, c.refType)
		if err == nil || errors.Is(err, media.ErrStagedTokenRejected) || apperr.KindOf(err) != apperr.KindInternal || len(f.calls) != 0 {
			t.Fatalf("%s：err = %v、呼叫 %d 次，預期本地擋下", name, err, len(f.calls))
		}
	}
}

func TestLinkAndUnlinkAreSignedWithoutAnAccountAndCarryExactAssets(t *testing.T) {
	f := &fakeMediaService{}
	c := start(t, f)
	ref := media.Ref{Type: media.RefTypeBanner, ID: "b1"}
	if err := c.Link(context.Background(), ref, []string{"a1", "a2"}); err != nil {
		t.Fatal(err)
	}
	if err := c.Unlink(context.Background(), ref, []string{"a3"}); err != nil {
		t.Fatal(err)
	}
	if got := f.linked; got.GetRefType() != "7109" || got.GetRefId() != "b1" || len(got.GetAssetIds()) != 2 {
		t.Fatalf("登記 = %+v", got)
	}
	if got := f.unlinked; got.GetRefType() != "7109" || got.GetRefId() != "b1" || len(got.GetAssetIds()) != 1 || got.GetAssetIds()[0] != "a3" {
		t.Fatalf("解除 = %+v", got)
	}
	for _, call := range f.calls {
		if call.header.Get("x-account-id") != "" {
			t.Errorf("%s 不帶帳號：%q", call.procedure, call.header.Get("x-account-id"))
		}
		verifySignature(t, call.header)
	}
	if err := c.Link(context.Background(), ref, nil); err != nil || len(f.calls) != 2 {
		t.Fatalf("沒有資產時不呼叫：%v, %d", err, len(f.calls))
	}
	if err := c.Unlink(context.Background(), ref, nil); err != nil || len(f.calls) != 2 {
		t.Fatalf("沒有資產時不呼叫：%v, %d", err, len(f.calls))
	}
}

func TestLinkAndUnlinkAreSingleCallsWithoutInnerRetries(t *testing.T) {
	ref := media.Ref{Type: media.RefTypeBanner, ID: "x"}
	for name, call := range map[string]func(*Client) error{
		"link":   func(c *Client) error { return c.Link(context.Background(), ref, []string{"a"}) },
		"unlink": func(c *Client) error { return c.Unlink(context.Background(), ref, []string{"a"}) },
	} {
		f := &fakeMediaService{linkFailures: 1}
		if err := call(start(t, f)); err == nil {
			t.Fatalf("%s：暫時性失敗要回錯誤，由 River job 重試", name)
		}
		if len(f.calls) != 1 {
			t.Fatalf("%s：呼叫了 %d 次，預期 1 次", name, len(f.calls))
		}
	}
}

// media-service 只拿 x-account-type 當閘門：管理員帶 4，帳號不論角色都帶買家的 1（兩者都會放行）。
func TestLegacyAccountType(t *testing.T) {
	if got := legacyAccountType(media.Uploader{ID: 1, Admin: true}); got != "4" {
		t.Errorf("管理員 = %q", got)
	}
	if got := legacyAccountType(media.Uploader{ID: 1}); got != "1" {
		t.Errorf("帳號 = %q", got)
	}
}

// is_secure 照 media 的回報帶出（證件的擁有者以它拒絕非機敏的上傳）。
func TestFinalizeCarriesIsSecure(t *testing.T) {
	for _, secure := range []bool{true, false} {
		a, err := start(t, &fakeMediaService{secure: secure}).Finalize(context.Background(), admin, "tok", media.RefTypeCertification)
		if err != nil || a.Secure != secure {
			t.Fatalf("secure=%v：asset=%+v err=%v", secure, a, err)
		}
	}
}

// 機敏讀取：三元組原樣送出；內容與型別帶回；「不是機敏資產」（400）與「找不到」（403、404）是不同的錯誤；其他碼是未分類。
func TestReadSecureSendsTheTripleAndSeparatesNotSecureFromNotFound(t *testing.T) {
	ref := media.Ref{Type: media.RefTypeCertification, ID: "app-1"}
	f := &fakeMediaService{readData: []byte("IMG")}
	got, err := start(t, f).ReadSecure(context.Background(), "secure/7111/x", ref)
	if err != nil || string(got.Data) != "IMG" || got.ContentType != "image/png" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	verifySignature(t, f.calls[0].header)
	for code, check := range map[int32]func(error) bool{
		400: func(err error) bool {
			return errors.Is(err, media.ErrNotSecure) && apperr.KindOf(err) != apperr.KindNotFound
		},
		403: func(err error) bool {
			return apperr.KindOf(err) == apperr.KindNotFound && !errors.Is(err, media.ErrNotSecure)
		},
		404: func(err error) bool {
			return apperr.KindOf(err) == apperr.KindNotFound && !errors.Is(err, media.ErrNotSecure)
		},
		500: func(err error) bool {
			return err != nil && apperr.KindOf(err) == apperr.KindInternal && !errors.Is(err, media.ErrNotSecure)
		},
	} {
		_, err := start(t, &fakeMediaService{readCode: code}).ReadSecure(context.Background(), "k", ref)
		if !check(err) {
			t.Errorf("code %d：err=%v", code, err)
		}
	}
}
