package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/media"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

const testProspectSourcePage = "https://www.example/about"

func okMediaClient() *mockMediaClient {
	return &mockMediaClient{
		FinalizeImagesFn: func(context.Context, []string, string) ([]AssetInfo, error) {
			return []AssetInfo{{AssetId: "9001", ObjectKey: "public/seller_prospect_logo/1/20260920/a.png"}}, nil
		},
	}
}

func TestUpdateProspectMedia_LogoWritesKeyAndSource(t *testing.T) {
	repo := &fakeProspectRepo{updateMediaOut: domain.SellerProspect{ID: 7, MediaVersion: 4}}
	mc := okMediaClient()
	var finalizeRefType string
	mc.FinalizeImagesFn = func(_ context.Context, tokens []string, refType string) ([]AssetInfo, error) {
		finalizeRefType = refType
		if len(tokens) != 1 || tokens[0] != "tok-1" {
			t.Fatalf("staged token not forwarded verbatim: %v", tokens)
		}
		return []AssetInfo{{AssetId: "9001", ObjectKey: "public/seller_prospect_logo/1/20260920/a.png"}}, nil
	}
	uc := NewSellerProspectUseCase(repo, mc)

	got, err := uc.UpdateProspectMedia(context.Background(), ProspectMediaInput{
		ProspectID: 7, Kind: ProspectMediaKindLogo, StagedToken: "tok-1",
		SourceURL: testProspectSourcePage, ExpectedMediaVersion: 3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.MediaVersion != 4 {
		t.Fatalf("expected the repository row back, got version %d", got.MediaVersion)
	}
	// refType 錯了會把圖存進別人的 R2 前綴（#2199 就是這一類），所以逐字斷言。
	if finalizeRefType != media.RefTypeSellerProspectLogo {
		t.Fatalf("logo must finalize under %s, got %s", media.RefTypeSellerProspectLogo, finalizeRefType)
	}
	if repo.lastMediaUpdate.ObjectKey != "public/seller_prospect_logo/1/20260920/a.png" {
		t.Fatalf("objectKey not taken from the finalized asset: %q", repo.lastMediaUpdate.ObjectKey)
	}
	if repo.lastMediaUpdate.SourceURL != testProspectSourcePage {
		t.Fatalf("sourceURL must round-trip verbatim, got %q", repo.lastMediaUpdate.SourceURL)
	}
	if repo.lastMediaUpdate.ExpectedMediaVersion != 3 {
		t.Fatalf("CAS base not forwarded: %d", repo.lastMediaUpdate.ExpectedMediaVersion)
	}
}

func TestUpdateProspectMedia_CoverUsesItsOwnRefType(t *testing.T) {
	repo := &fakeProspectRepo{updateMediaOut: domain.SellerProspect{ID: 7, MediaVersion: 1}}
	mc := okMediaClient()
	var refType string
	mc.FinalizeImagesFn = func(_ context.Context, _ []string, rt string) ([]AssetInfo, error) {
		refType = rt
		return []AssetInfo{{AssetId: "9002", ObjectKey: "public/seller_prospect_cover/1/20260920/b.png"}}, nil
	}
	uc := NewSellerProspectUseCase(repo, mc)

	if _, err := uc.UpdateProspectMedia(context.Background(), ProspectMediaInput{
		ProspectID: 7, Kind: ProspectMediaKindCover, StagedToken: "tok-1", SourceURL: testProspectSourcePage,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if refType != media.RefTypeSellerProspectCover {
		t.Fatalf("cover must finalize under %s, got %s", media.RefTypeSellerProspectCover, refType)
	}
	if repo.lastMediaUpdate.Kind != ProspectMediaKindCover {
		t.Fatalf("kind not forwarded to the repository: %q", repo.lastMediaUpdate.Kind)
	}
}

// Link 失敗必須在寫 DB 之前中止。反過來（先寫 DB）會留下一個指著第 8 天被
// media GC 實體刪除的 objectKey 的業務列，而且只有顯示時才 404。
func TestUpdateProspectMedia_LinkFailureDoesNotWriteRow(t *testing.T) {
	repo := &fakeProspectRepo{}
	mc := okMediaClient()
	mc.LinkImagesFn = func(context.Context, string, string, []string) error {
		return errors.New("media unavailable")
	}
	uc := NewSellerProspectUseCase(repo, mc)

	_, err := uc.UpdateProspectMedia(context.Background(), ProspectMediaInput{
		ProspectID: 7, Kind: ProspectMediaKindLogo, StagedToken: "tok-1", SourceURL: testProspectSourcePage,
	})
	if err == nil {
		t.Fatal("link failure must be fatal")
	}
	if repo.updateMediaCalls != 0 {
		t.Fatalf("row was written despite a failed link (%d calls)", repo.updateMediaCalls)
	}
}

// Release 是清掉被取代的舊資產。失敗的後果是多留一份存量（成本），
// 不是顯示壞掉，所以不能讓它把一次成功的換圖翻成失敗。
func TestUpdateProspectMedia_ReleaseFailureStillSucceeds(t *testing.T) {
	repo := &fakeProspectRepo{updateMediaOut: domain.SellerProspect{ID: 7, MediaVersion: 2}}
	mc := okMediaClient()
	mc.ReleaseReferencesFn = func(context.Context, string, []string, []string) error {
		return errors.New("release failed")
	}
	uc := NewSellerProspectUseCase(repo, mc)

	got, err := uc.UpdateProspectMedia(context.Background(), ProspectMediaInput{
		ProspectID: 7, Kind: ProspectMediaKindLogo, StagedToken: "tok-1", SourceURL: testProspectSourcePage,
	})
	if err != nil {
		t.Fatalf("release failure must not fail the update: %v", err)
	}
	if got.MediaVersion != 2 {
		t.Fatalf("expected the written row back, got version %d", got.MediaVersion)
	}
}

func TestUpdateProspectMedia_ConflictSurfacesUnchanged(t *testing.T) {
	repo := &fakeProspectRepo{updateMediaErr: domain.ErrProspectMediaConflict}
	uc := NewSellerProspectUseCase(repo, okMediaClient())

	_, err := uc.UpdateProspectMedia(context.Background(), ProspectMediaInput{
		ProspectID: 7, Kind: ProspectMediaKindLogo, StagedToken: "tok-1", SourceURL: testProspectSourcePage,
	})
	if !errors.Is(err, domain.ErrProspectMediaConflict) {
		t.Fatalf("conflict must reach the handler as-is, got %v", err)
	}
}

func TestUpdateProspectMedia_RejectsUnknownKindBeforeTouchingMedia(t *testing.T) {
	repo := &fakeProspectRepo{}
	mc := &mockMediaClient{FinalizeImagesFn: func(context.Context, []string, string) ([]AssetInfo, error) {
		t.Fatal("media must not be called for an invalid kind")
		return nil, nil
	}}
	uc := NewSellerProspectUseCase(repo, mc)

	_, err := uc.UpdateProspectMedia(context.Background(), ProspectMediaInput{
		ProspectID: 7, Kind: "photo", StagedToken: "tok-1", SourceURL: testProspectSourcePage,
	})
	if !errors.Is(err, ErrProspectMediaKindInvalid) {
		t.Fatalf("expected kind rejection, got %v", err)
	}
}

// 後端的來源網址值域必須跟 Admin 的 publicHttpsUrl 一致。後端比前端寬，
// 等於讓一筆前端拒收的網址從別的呼叫端寫進來，之後 Admin 讀回時會把它
// 判成「存檔結果不完整」。
func TestValidateProspectSourceURL_MatchesAdminDomain(t *testing.T) {
	rejected := map[string]string{
		"http":        "http://www.example/about",
		"userinfo":    "https://u:p@www.example/about",
		"fragment":    "https://www.example/about#logo",
		"literal ip":  "https://203.0.113.10/logo.png",
		"private ip":  "https://192.168.1.10/logo.png",
		"dotless":     "https://intranet/logo.png",
		"local":       "https://printer.local/logo.png",
		"not a url":   "://",
		"empty":       "",
		"javascript":  "javascript:alert(1)",
		"data scheme": "data:image/png;base64,AAAA",
	}
	for name, raw := range rejected {
		if err := validateProspectSourceURL(raw); err == nil {
			t.Errorf("%s: %q should have been rejected", name, raw)
		}
	}

	accepted := []string{
		"https://www.example/",
		"https://www.example/story_page.php?id=128",
		"https://sub.example.co.jp/a/b/c.html",
	}
	for _, raw := range accepted {
		if err := validateProspectSourceURL(raw); err != nil {
			t.Errorf("%q should have been accepted: %v", raw, err)
		}
	}
}

func TestValidateProspectSourceURL_RejectsOverlongURL(t *testing.T) {
	long := "https://www.example/?q=" + string(make([]byte, sourceURLMaxLen))
	if err := validateProspectSourceURL(long); err == nil {
		t.Fatal("a URL past the Admin length cap must be rejected")
	}
}
