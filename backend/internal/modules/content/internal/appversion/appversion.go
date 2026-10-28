package appversion

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"

	"github.com/Will413028/hawkr/backend/internal/modules/content/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/content/internal/validation"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/content/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

type Service struct {
	d Deps
}

type Deps struct {
	DB     *db.DB
	Logger *slog.Logger
}

func New(d Deps) *Service {
	return &Service{d: d}
}

func (m *Service) q(ctx context.Context) *store.Queries {
	return store.New(m.d.DB.Conn(ctx))
}

// CheckAppVersion 判斷 clientVersion 是否低於門檻、必須強制更新。
//
// 一律 fail-open：沒有設定、門檻或 App 送來的版本格式不對，都回傳不需要更新，不讓設定錯誤把
// 使用者擋在門外。這兩種情況各記一筆 log，是發現設定漏填的唯一線索。
func (m *Service) CheckAppVersion(ctx context.Context, appType model.AppType, clientVersion string) (model.VersionCheck, error) {
	if err := validateAppType(appType); err != nil {
		return model.VersionCheck{}, err
	}
	if err := validation.RequireText("ClientVersion", "App 的版本", clientVersion); err != nil {
		return model.VersionCheck{}, err
	}
	gate, err := m.q(ctx).GetAppVersionGate(ctx, string(appType))
	if errors.Is(err, pgx.ErrNoRows) {
		m.d.Logger.InfoContext(ctx, "App 版本門檻沒有設定，放行", "app_type", appType)
		return model.VersionCheck{}, nil
	}
	if err != nil {
		return model.VersionCheck{}, fmt.Errorf("content：讀取 %s 的版本門檻：%w", appType, err)
	}
	g := toGate(gate)
	result := model.VersionCheck{
		MinimumVersion: g.MinimumVersion,
		StoreURL:       g.StoreURL,
		NoticeTitle:    g.NoticeTitle,
		NoticeBody:     g.NoticeBody,
	}
	minimum, okMin := parseSemver(g.MinimumVersion)
	client, okClient := parseSemver(clientVersion)
	if !okMin || !okClient {
		m.d.Logger.WarnContext(ctx, "App 版本格式不對，放行",
			"app_type", appType, "minimum_version", g.MinimumVersion, "client_version", clientVersion)
		return result, nil
	}
	result.UpdateRequired = compareSemver(client, minimum) < 0
	return result, nil
}

// AppVersionGate 回傳一組門檻設定（後台）。
func (m *Service) AppVersionGate(ctx context.Context, appType model.AppType) (model.AppVersionGate, error) {
	if err := validateAppType(appType); err != nil {
		return model.AppVersionGate{}, err
	}
	row, err := m.q(ctx).GetAppVersionGate(ctx, string(appType))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.AppVersionGate{}, apperr.NotFound("這組版本門檻不存在")
	}
	if err != nil {
		return model.AppVersionGate{}, fmt.Errorf("content：讀取 %s 的版本門檻：%w", appType, err)
	}
	return toGate(row), nil
}

// ListAppVersionGates 回傳全部門檻設定（後台）。
func (m *Service) ListAppVersionGates(ctx context.Context) ([]model.AppVersionGate, error) {
	rows, err := m.q(ctx).ListAppVersionGates(ctx)
	if err != nil {
		return nil, fmt.Errorf("content：列出版本門檻：%w", err)
	}
	out := make([]model.AppVersionGate, len(rows))
	for i, r := range rows {
		out[i] = toGate(r)
	}
	return out, nil
}

// CreateAppVersionGate 建立一組還沒有設定的門檻（後台）。那一組已經存在時回 AlreadyExists、不覆寫：
// 以 key 建立而沒有帶版本，就只能建立（ADR 2026-09-25-optimistic-concurrency-etag D3）。
func (m *Service) CreateAppVersionGate(ctx context.Context, g model.AppVersionGate) (model.AppVersionGate, error) {
	if err := validateGate(g); err != nil {
		return model.AppVersionGate{}, err
	}
	row, err := m.q(ctx).InsertAppVersionGate(ctx, store.InsertAppVersionGateParams{
		AppType:        string(g.AppType),
		MinimumVersion: g.MinimumVersion,
		StoreUrl:       g.StoreURL,
		NoticeTitle:    g.NoticeTitle,
		NoticeBody:     g.NoticeBody,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// 業務原因與版本衝突相同：有人先建立了，client 保留輸入、顯示最新的設定（ADR 的 D3）。
		return model.AppVersionGate{}, apperr.AlreadyExists("這組版本門檻已經有人建立，請對照最新的設定後再送出").
			WithReason(apperr.ReasonVersionConflict)
	}
	if err != nil {
		return model.AppVersionGate{}, fmt.Errorf("content：建立 %s 的版本門檻：%w", g.AppType, err)
	}
	return toGate(row), nil
}

// UpdateAppVersionGate 整筆覆寫一組已經存在的門檻（後台）。version 是讀取時拿到的版本，
// 與目前的不符就回 VersionConflict、不寫入。
func (m *Service) UpdateAppVersionGate(ctx context.Context, g model.AppVersionGate, version int64) (model.AppVersionGate, error) {
	if err := validation.RequireVersion("版本門檻", version); err != nil {
		return model.AppVersionGate{}, err
	}
	if err := validateGate(g); err != nil {
		return model.AppVersionGate{}, err
	}
	q := m.q(ctx)
	row, err := q.UpdateAppVersionGate(ctx, store.UpdateAppVersionGateParams{
		AppType:         string(g.AppType),
		MinimumVersion:  g.MinimumVersion,
		StoreUrl:        g.StoreURL,
		NoticeTitle:     g.NoticeTitle,
		NoticeBody:      g.NoticeBody,
		ExpectedVersion: version,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// 分辨不存在與版本不符。
		_, err := q.GetAppVersionGate(ctx, string(g.AppType))
		if errors.Is(err, pgx.ErrNoRows) {
			return model.AppVersionGate{}, apperr.NotFound("這組版本門檻不存在")
		}
		if err != nil {
			return model.AppVersionGate{}, fmt.Errorf("content：讀取 %s 的版本門檻：%w", g.AppType, err)
		}
		return model.AppVersionGate{}, apperr.VersionConflict("這組版本門檻已被其他人修改，請對照最新的設定後再送出")
	}
	if err != nil {
		return model.AppVersionGate{}, fmt.Errorf("content：修改 %s 的版本門檻：%w", g.AppType, err)
	}
	return toGate(row), nil
}

// validate 檢查寫入的值域，DB 的 CHECK 是第二道。
func validateGate(g model.AppVersionGate) error {
	if err := validateAppType(g.AppType); err != nil {
		return err
	}
	if _, ok := parseSemver(g.MinimumVersion); !ok {
		return apperr.InvalidInputOn([]string{"MinimumVersion"}, "最低版本 %q 必須是三段數字，例如 1.2.3", g.MinimumVersion)
	}
	if !validStoreURL(g.StoreURL) {
		return apperr.InvalidInputOn([]string{"StoreURL"}, "商店網址只接受 https://、itms-apps:// 開頭")
	}
	return nil
}

func validateAppType(appType model.AppType) error {
	if !slices.Contains(model.AppTypes(), appType) {
		return apperr.InvalidInputOn([]string{"AppType"}, "請選擇買家或賣家 App")
	}
	return nil
}

// validStoreURL 限制商店網址的 scheme：這個值會交給 App 的 url_launcher 開啟，
// 不設限等於讓誤填的任意 scheme 直接送到使用者裝置上。空字串表示沒有設定。
func validStoreURL(v string) bool {
	if v == "" {
		return true
	}
	for _, prefix := range []string{"https://", "itms-apps://"} {
		if strings.HasPrefix(v, prefix) {
			return true
		}
	}
	return false
}

// parseSemver 把 "major.minor.patch" 拆成三個整數，逐段比較，所以 1.0.10 大於 1.0.9。
// 只接受三段純數字（與 DB 的 CHECK 相同）；build number 不參與（各平台不一致）。
func parseSemver(v string) ([3]int, bool) {
	var out [3]int
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		if p == "" || strings.Trim(p, "0123456789") != "" {
			return out, false // strconv.Atoi 接受 "+1"，DB 的 CHECK 不接受，兩道防線要一致
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

// compareSemver 回傳 -1（a<b）、0 或 1（a>b）。
func compareSemver(a, b [3]int) int {
	for i := range 3 {
		switch {
		case a[i] < b[i]:
			return -1
		case a[i] > b[i]:
			return 1
		}
	}
	return 0
}

func toGate(r store.ContentAppVersionGate) model.AppVersionGate {
	return model.AppVersionGate{
		AppType:        model.AppType(r.AppType),
		MinimumVersion: r.MinimumVersion,
		StoreURL:       r.StoreUrl,
		NoticeTitle:    r.NoticeTitle,
		NoticeBody:     r.NoticeBody,
		Version:        r.Version,
		UpdatedAt:      r.UpdatedAt,
	}
}
