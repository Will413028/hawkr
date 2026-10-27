package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type fakeIDTokens struct {
	id    model.Identity
	err   error
	calls int
}

func (f *fakeIDTokens) Verify(context.Context, string) (model.Identity, error) {
	f.calls++
	return f.id, f.err
}

var googleNow = time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)

func googleService(v model.IDTokenVerifier) *Service {
	return &Service{d: Deps{Google: v, Now: func() time.Time { return googleNow }}}
}

func TestVerifyGoogleChecksTheNonceAndFreshness(t *testing.T) {
	valid := model.Identity{Subject: "g1", Email: "v@example.com", EmailVerified: true, Nonce: "n-1", IssuedAt: googleNow.Add(-9 * time.Minute)}
	cases := []struct {
		name  string
		id    model.Identity
		nonce string
		kind  apperr.Kind // pass 為 false 時預期的錯誤類別
		pass  bool
	}{
		{name: "nonce 相符、iat 9 分鐘前", id: valid, nonce: "n-1", pass: true},
		{name: "iat 剛好 10 分鐘前", id: with(valid, func(i *model.Identity) { i.IssuedAt = googleNow.Add(-10 * time.Minute) }), nonce: "n-1", pass: true},
		{name: "iat 11 分鐘前", id: with(valid, func(i *model.Identity) { i.IssuedAt = googleNow.Add(-11 * time.Minute) }), nonce: "n-1", kind: apperr.KindUnauthenticated},
		{name: "iat 剛好超過 10 分鐘", id: with(valid, func(i *model.Identity) { i.IssuedAt = googleNow.Add(-10*time.Minute - time.Second) }), nonce: "n-1", kind: apperr.KindUnauthenticated},
		{name: "沒有 iat", id: with(valid, func(i *model.Identity) { i.IssuedAt = time.Time{} }), nonce: "n-1", kind: apperr.KindUnauthenticated},
		{name: "token 沒有 nonce", id: with(valid, func(i *model.Identity) { i.Nonce = "" }), nonce: "n-1", kind: apperr.KindUnauthenticated},
		{name: "nonce 不符", id: valid, nonce: "n-2", kind: apperr.KindUnauthenticated},
		{name: "nonce 只是前綴", id: with(valid, func(i *model.Identity) { i.Nonce = "n-" }), nonce: "n-1", kind: apperr.KindUnauthenticated},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := googleService(&fakeIDTokens{id: c.id}).VerifyGoogle(t.Context(), "tok", c.nonce)
			if c.pass {
				if err != nil || got.Email != "v@example.com" {
					t.Fatalf("got=%+v err=%v", got, err)
				}
				return
			}
			if apperr.KindOf(err) != c.kind || got != (model.Identity{}) {
				t.Fatalf("got=%+v err=%v，預期 %v", got, err, c.kind)
			}
		})
	}
}

func with(id model.Identity, f func(*model.Identity)) model.Identity {
	f(&id)
	return id
}

// 空的 nonce 是呼叫方的程式錯誤：不放行，也不當成 token 無效（那會被入口轉成一般的拒絕而藏起 bug），而且不呼叫 provider。
func TestVerifyGoogleRejectsAnEmptyNonceAsAProgrammingError(t *testing.T) {
	v := &fakeIDTokens{id: model.Identity{Nonce: "", IssuedAt: googleNow}}
	_, err := googleService(v).VerifyGoogle(t.Context(), "tok", "")
	if err == nil || apperr.KindOf(err) != apperr.KindInternal || v.calls != 0 {
		t.Fatalf("err=%v kind=%v calls=%d", err, apperr.KindOf(err), v.calls)
	}
}

func TestVerifyGoogleErrorClasses(t *testing.T) {
	s := googleService(&fakeIDTokens{err: errors.Join(errors.New("bad sig"), model.ErrInvalidToken)})
	if _, err := s.VerifyGoogle(t.Context(), "tok", "n"); apperr.KindOf(err) != apperr.KindUnauthenticated {
		t.Fatalf("無效 token：%v", err)
	}
	s = googleService(&fakeIDTokens{err: errors.New("jwks down")})
	if _, err := s.VerifyGoogle(t.Context(), "tok", "n"); apperr.KindOf(err) != apperr.KindUnavailable {
		t.Fatalf("公鑰問不到：%v", err)
	}
	if _, err := googleService(nil).VerifyGoogle(t.Context(), "tok", "n"); apperr.KindOf(err) != apperr.KindUnavailable {
		t.Fatalf("沒有設定：%v", err)
	}
	if _, err := googleService(&fakeIDTokens{}).VerifyGoogle(t.Context(), "", "n"); apperr.KindOf(err) != apperr.KindUnauthenticated {
		t.Fatalf("空 token：%v", err)
	}
}
