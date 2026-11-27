package admin

import (
	"context"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Will413028/hawkr/backend/internal/modules/imageai/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/imageai/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

const defaultConstraint = "styles_default_per_category"

type txKey struct{}

type fakeQ struct {
	styles map[uuid.UUID]store.ImageaiStyle
	quotas map[int64]store.ImageaiQuota
	onLock func(uuid.UUID)
}

func newFakeQ() *fakeQ {
	return &fakeQ{styles: map[uuid.UUID]store.ImageaiStyle{}, quotas: map[int64]store.ImageaiQuota{}}
}

func (f *fakeQ) ListStylesForAdmin(_ context.Context, category string) ([]store.ImageaiStyle, error) {
	var out []store.ImageaiStyle
	for _, s := range f.styles {
		if category == "" || s.Category == category {
			out = append(out, s)
		}
	}
	slices.SortFunc(out, func(a, b store.ImageaiStyle) int {
		if a.DisplayOrder != b.DisplayOrder {
			return int(a.DisplayOrder - b.DisplayOrder)
		}
		return strings.Compare(a.Name, b.Name)
	})
	return out, nil
}

func (f *fakeQ) GetStyle(_ context.Context, id uuid.UUID) (store.ImageaiStyle, error) {
	s, ok := f.styles[id]
	if !ok {
		return store.ImageaiStyle{}, pgx.ErrNoRows
	}
	return s, nil
}

func (f *fakeQ) LockStyle(ctx context.Context, id uuid.UUID) (store.ImageaiStyle, error) {
	if f.onLock != nil {
		f.onLock(id)
	}
	return f.GetStyle(ctx, id)
}

func unique(constraint string) error {
	return &pgconn.PgError{Code: "23505", ConstraintName: constraint}
}

func (f *fakeQ) InsertStyle(_ context.Context, a store.InsertStyleParams) (store.ImageaiStyle, error) {
	for _, s := range f.styles {
		if s.Category == a.Category && s.Name == a.Name {
			return store.ImageaiStyle{}, unique(nameConstraint)
		}
	}
	s := store.ImageaiStyle{
		ID: a.ID, Name: a.Name, Label: a.Label, Prompt: a.Prompt, Category: a.Category, ThumbnailKey: a.ThumbnailKey, ThumbnailAssetID: a.ThumbnailAssetID,
		DisplayOrder: a.DisplayOrder, Enabled: a.Enabled, Version: 1, CreatedAt: a.Now, UpdatedAt: a.Now,
	}
	f.styles[a.ID] = s
	return s, nil
}

func (f *fakeQ) UpdateStyle(_ context.Context, a store.UpdateStyleParams) (store.ImageaiStyle, error) {
	s, ok := f.styles[a.ID]
	if !ok || s.Version != a.ExpectedVersion {
		return store.ImageaiStyle{}, pgx.ErrNoRows
	}
	s.Label, s.Prompt, s.ThumbnailKey = a.Label, a.Prompt, a.ThumbnailKey
	if a.ThumbnailAssetID != nil {
		s.ThumbnailAssetID = a.ThumbnailAssetID
	}
	s.DisplayOrder, s.Enabled, s.Version, s.UpdatedAt = a.DisplayOrder, a.Enabled, s.Version+1, a.Now
	f.styles[a.ID] = s
	return s, nil
}

func (f *fakeQ) LockCategoryStyles(_ context.Context, category string) ([]store.ImageaiStyle, error) {
	var out []store.ImageaiStyle
	for _, s := range f.styles {
		if s.Category == category {
			out = append(out, s)
		}
	}
	slices.SortFunc(out, func(a, b store.ImageaiStyle) int { return strings.Compare(a.ID.String(), b.ID.String()) })
	return out, nil
}

func (f *fakeQ) MarkStyleDefault(_ context.Context, a store.MarkStyleDefaultParams) (store.ImageaiStyle, error) {
	s, ok := f.styles[a.ID]
	if !ok {
		return store.ImageaiStyle{}, pgx.ErrNoRows
	}
	for _, o := range f.styles {
		if o.Category == s.Category && o.IsDefault {
			return store.ImageaiStyle{}, unique(defaultConstraint)
		}
	}
	s.IsDefault, s.Version, s.UpdatedAt = true, s.Version+1, a.Now
	f.styles[a.ID] = s
	return s, nil
}

func (f *fakeQ) ClearDefaultStyle(_ context.Context, a store.ClearDefaultStyleParams) ([]uuid.UUID, error) {
	var n []uuid.UUID
	for id, s := range f.styles {
		if s.Category == a.Category && s.IsDefault && id != a.ExcludeID {
			s.IsDefault, s.Version = false, s.Version+1
			f.styles[id] = s
			n = append(n, id)
		}
	}
	return n, nil
}

func (f *fakeQ) DeleteStyle(_ context.Context, id uuid.UUID) (int64, error) {
	if _, ok := f.styles[id]; !ok {
		return 0, nil
	}
	delete(f.styles, id)
	return 1, nil
}

func (f *fakeQ) GetQuota(_ context.Context, id int64) (store.ImageaiQuota, error) {
	q, ok := f.quotas[id]
	if !ok {
		return store.ImageaiQuota{}, pgx.ErrNoRows
	}
	return q, nil
}

func (f *fakeQ) LockQuota(ctx context.Context, id int64) (store.ImageaiQuota, error) {
	return f.GetQuota(ctx, id)
}

func (f *fakeQ) InsertQuota(_ context.Context, a store.InsertQuotaParams) (store.ImageaiQuota, error) {
	if _, ok := f.quotas[a.StoreID]; ok {
		return store.ImageaiQuota{}, pgx.ErrNoRows
	}
	q := store.ImageaiQuota{StoreID: a.StoreID, WeeklyLimit: a.WeeklyLimit, Memo: a.Memo, Version: 1, CreatedAt: a.Now, UpdatedAt: a.Now}
	f.quotas[a.StoreID] = q
	return q, nil
}

func (f *fakeQ) UpdateQuota(_ context.Context, a store.UpdateQuotaParams) (store.ImageaiQuota, error) {
	q, ok := f.quotas[a.StoreID]
	if !ok || q.Version != a.ExpectedVersion {
		return store.ImageaiQuota{}, pgx.ErrNoRows
	}
	q.WeeklyLimit, q.Memo, q.Version, q.UpdatedAt = a.WeeklyLimit, a.Memo, q.Version+1, a.Now
	f.quotas[a.StoreID] = q
	return q, nil
}

func (f *fakeQ) DeleteQuota(_ context.Context, id int64) (int64, error) {
	if _, ok := f.quotas[id]; !ok {
		return 0, nil
	}
	delete(f.quotas, id)
	return 1, nil
}

type fakeAudit struct{ entries []adminaudit.Entry }

func (a *fakeAudit) Record(ctx context.Context, e adminaudit.Entry) error {
	if ctx.Value(txKey{}) == nil {
		return adminaudit.ErrNoTransaction
	}
	a.entries = append(a.entries, e)
	return nil
}

func assetOf(name string) uuid.UUID {
	return media.AssetFor(name)
}

type fakeMedia struct {
	confirmed []string
	deleted   []uuid.UUID
	failToken string
}

func (f *fakeMedia) Confirm(ctx context.Context, _ media.Uploader, token string, want media.Visibility) (media.ConfirmedAsset, error) {
	if ctx.Value(txKey{}) == nil {
		panic("Confirm 必須在 transaction 內呼叫")
	}
	if want != media.VisibilityPublic {
		panic("風格縮圖必須是公開可讀")
	}
	if token == f.failToken {
		return media.ConfirmedAsset{}, apperr.InvalidInputOn([]string{"ThumbnailToken"}, "縮圖無法使用")
	}
	f.confirmed = append(f.confirmed, token)
	return media.ConfirmedAsset{ID: assetOf(token), ObjectKey: "public/" + token + ".jpg", Visibility: media.VisibilityPublic}, nil
}

func (f *fakeMedia) MarkDeleted(ctx context.Context, ids []uuid.UUID) error {
	if ctx.Value(txKey{}) == nil {
		panic("MarkDeleted 必須在 transaction 內呼叫")
	}
	f.deleted = append(f.deleted, ids...)
	return nil
}

type fakeStores struct{ missing map[int64]bool }

func (f fakeStores) Exists(_ context.Context, id int64) error {
	if f.missing[id] {
		return apperr.NotFound("seller：找不到店家")
	}
	return nil
}

func (f fakeStores) LockNotRetired(ctx context.Context, id int64) error {
	if ctx.Value(txKey{}) == nil {
		panic("LockNotRetired 必須在 transaction 內呼叫")
	}
	return f.Exists(ctx, id)
}

type harness struct {
	s      *Service
	q      *fakeQ
	audit  *fakeAudit
	media  *fakeMedia
	stores fakeStores
	ids    int
	used   int32
}

var (
	actor = model.Actor{AdminID: 7, Action: "/hawkr.admin.imageai.v1alpha1.ImageAiAdminService/Test"}
	t0    = time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
)

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{q: newFakeQ(), audit: &fakeAudit{}, media: &fakeMedia{}, stores: fakeStores{missing: map[int64]bool{}}}
	h.s = New(Deps{
		Tx: func(ctx context.Context, fn func(context.Context) error) error {
			return fn(context.WithValue(ctx, txKey{}, true))
		},
		Q:      func(context.Context) Queries { return h.q },
		Audit:  h.audit,
		Media:  h.media,
		Stores: h.stores,
		Used:   func(context.Context, int64) (int32, error) { return h.used, nil },
		NewID: func() uuid.UUID {
			h.ids++
			return uuid.MustParse("00000000-0000-7000-8000-" + strings.Repeat("0", 11) + string(rune('0'+h.ids)))
		},
		Now: func() time.Time { return t0 },
	})
	return h
}

func (h *harness) seed(name, category string, isDefault bool, asset string) store.ImageaiStyle {
	h.ids++
	id := uuid.MustParse("10000000-0000-7000-8000-" + strings.Repeat("0", 11) + string(rune('0'+h.ids)))
	key := ""
	var assetID *uuid.UUID
	if asset != "" {
		key = "public/" + asset + ".jpg"
		id := assetOf(asset)
		assetID = &id
	}
	s := store.ImageaiStyle{
		ID: id, Name: name, Label: name, Prompt: "p", Category: category, ThumbnailKey: key, ThumbnailAssetID: assetID,
		IsDefault: isDefault, Enabled: true, Version: 3, CreatedAt: t0, UpdatedAt: t0,
	}
	h.q.styles[id] = s
	return s
}

func wantKind(t *testing.T, err error, want apperr.Kind) {
	t.Helper()
	if err == nil || apperr.KindOf(err) != want {
		t.Fatalf("err = %v（分類 %v），預期 %v", err, apperr.KindOf(err), want)
	}
}
