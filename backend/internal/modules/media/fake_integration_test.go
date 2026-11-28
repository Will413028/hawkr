//go:build integration

package media_test

import (
	"context"
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func fakeHoldFixture(t *testing.T) (*fixture, *media.Fake, time.Time) {
	t.Helper()
	f := setup(t)
	fake := media.NewFake()
	now := f.clock.Now()
	fake.Now = func() time.Time { return now }
	return f, fake, now
}

func inFakeTx(f *fixture, fn func(ctx context.Context) error) error {
	return f.d.InTx(f.t.Context(), fn)
}

func fakeSecureAsset(t *testing.T, f *fixture, fake *media.Fake, token string) uuid.UUID {
	t.Helper()
	var a media.ConfirmedAsset
	err := inFakeTx(f, func(ctx context.Context) error {
		var err error
		a, err = fake.Confirm(ctx, owner, token, media.VisibilitySecure)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return a.ID
}

func TestFakeHoldRejectsWhatTheRealModuleRejects(t *testing.T) {
	f, fake, now := fakeHoldFixture(t)
	secure := fakeSecureAsset(t, f, fake, "s1")
	var public media.ConfirmedAsset
	if err := inFakeTx(f, func(ctx context.Context) error {
		var err error
		public, err = fake.Confirm(ctx, owner, "p1", media.VisibilityPublic)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		ids    []uuid.UUID
		source string
		until  time.Time
		want   error
	}{
		{"未知資產", []uuid.UUID{uuid.New()}, "s", now.Add(time.Hour), media.ErrAssetNotFound},
		{"公開資產", []uuid.UUID{public.ID}, "s", now.Add(time.Hour), media.ErrNotSecure},
	}
	for _, c := range cases {
		err := inFakeTx(f, func(ctx context.Context) error { return fake.Hold(ctx, c.ids, c.source, uuid.New(), c.until) })
		if !errors.Is(err, c.want) {
			t.Fatalf("%s = %v，預期 %v", c.name, err, c.want)
		}
	}
	for name, args := range map[string]struct {
		source string
		until  time.Time
	}{"空來源": {"", now.Add(time.Hour)}, "期限已過": {"s", now}, "期限在過去": {"s", now.Add(-time.Hour)}} {
		err := inFakeTx(f, func(ctx context.Context) error {
			return fake.Hold(ctx, []uuid.UUID{secure}, args.source, uuid.New(), args.until)
		})
		if err == nil {
			t.Fatalf("%s 應被拒絕", name)
		}
	}
	if fake.IsHeld(secure) {
		t.Fatal("被拒的 Hold 不得留下保全")
	}
	if err := inFakeTx(f, func(ctx context.Context) error { return fake.MarkDeleted(ctx, []uuid.UUID{secure}) }); err != nil {
		t.Fatal(err)
	}
	err := inFakeTx(f, func(ctx context.Context) error {
		return fake.Hold(ctx, []uuid.UUID{secure}, "s", uuid.New(), now.Add(time.Hour))
	})
	if !errors.Is(err, media.ErrAssetGone) {
		t.Fatalf("已刪除資產 = %v，預期 ErrAssetGone", err)
	}
}

func TestFakeHoldExpiryDrivesIsHeldAndExtendHold(t *testing.T) {
	f, fake, now := fakeHoldFixture(t)
	secure := fakeSecureAsset(t, f, fake, "s1")
	src := uuid.New()
	if err := inFakeTx(f, func(ctx context.Context) error {
		return fake.Hold(ctx, []uuid.UUID{secure}, "s", src, now.Add(time.Hour))
	}); err != nil {
		t.Fatal(err)
	}
	if !fake.IsHeld(secure) {
		t.Fatal("期限內的保全要生效")
	}
	later := now.Add(2 * time.Hour)
	fake.Now = func() time.Time { return later }
	if fake.IsHeld(secure) {
		t.Fatal("到期的保全不得算生效")
	}
	err := inFakeTx(f, func(ctx context.Context) error { return fake.ExtendHold(ctx, "s", src, later.Add(time.Hour)) })
	if !errors.Is(err, media.ErrHoldNotFound) {
		t.Fatalf("延長已到期的保全 = %v，預期 ErrHoldNotFound", err)
	}
	if fake.IsHeld(secure) {
		t.Fatal("被拒的延長不得讓保全復活")
	}
}

func TestFakeCopyToSecureRefusesNonPublicSourcesAndReportsStatus(t *testing.T) {
	f, fake, _ := fakeHoldFixture(t)
	secure := fakeSecureAsset(t, f, fake, "s1")
	err := inFakeTx(f, func(ctx context.Context) error {
		_, err := fake.CopyToSecure(ctx, owner, secure)
		return err
	})
	if apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("機敏來源 = %v，預期 NotFound", err)
	}
	var public media.ConfirmedAsset
	var copyID uuid.UUID
	if err := inFakeTx(f, func(ctx context.Context) error {
		var err error
		if public, err = fake.Confirm(ctx, owner, "p1", media.VisibilityPublic); err != nil {
			return err
		}
		copyID, err = fake.CopyToSecure(ctx, owner, public.ID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if got, err := fake.CopyStatus(t.Context(), copyID); err != nil || got != media.CopyCompleted {
		t.Fatalf("CopyStatus = %q, %v", got, err)
	}
	if _, err := fake.CopyStatus(t.Context(), uuid.New()); !errors.Is(err, media.ErrAssetNotFound) {
		t.Fatalf("未知副本 CopyStatus = %v", err)
	}
}
