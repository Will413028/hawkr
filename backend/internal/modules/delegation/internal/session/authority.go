package session

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/adminperm"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type authorityIn struct {
	AdminID, StoreID int64
	Now              time.Time
	Eligibility bool
	Permission bool
	MissingAdminDenies bool
	Capability model.Capability
}

type authorityState struct {
	Admin      identity.Admin
	Store      model.StoreState
	StoreFound bool
	Auth       store.DelegationSellerDelegationAuthorization
	HasAuth    bool
	Denial     model.DelegationDenial
	Label      string
}

func (s authorityState) holds() bool { return s.Denial == 0 }

func (s *authorityState) deny(d model.DelegationDenial) {
	s.Denial, s.Label = d, model.DeniedLabel(d)
}

func (m *Module) checkAuthority(ctx context.Context, q *store.Queries, in authorityIn) (authorityState, error) {
	var s authorityState
	admin, err := m.d.Identity.Admin(ctx, in.AdminID)
	switch {
	case err == nil:
		s.Admin = admin
	case in.MissingAdminDenies && apperr.KindOf(err) == apperr.KindNotFound:
		s.deny(model.DenialAdminNotAllowlisted)
		return s, nil
	default:
		return s, fmt.Errorf("delegation：讀取管理員 %d：%w", in.AdminID, err)
	}
	if in.Eligibility {
		eligible, err := q.AdminDelegationEligible(ctx, in.AdminID)
		if err != nil {
			return s, fmt.Errorf("delegation：讀取管理員代行資格：%w", err)
		}
		if !eligible {
			s.deny(model.DenialAdminNotAllowlisted)
			return s, nil
		}
	}
	if in.Permission {
		allowed, err := m.d.Identity.AdminAllowsNow(ctx, in.AdminID, adminperm.DelegationSessionsCreate)
		if err != nil {
			return s, fmt.Errorf("delegation：檢查管理員的權限：%w", err)
		}
		if !allowed {
			s.deny(model.DenialAdminNotAllowlisted)
			return s, nil
		}
	}
	st, err := m.d.StoreState(ctx, in.StoreID)
	switch {
	case err != nil:
		return s, err
	case !st.Found:
		s.deny(model.DenialStoreNotDelegable)
		return s, nil
	}
	s.Store, s.StoreFound = st, true

	auth, err := q.GetDelegationAuthorizationForStore(ctx, in.StoreID)
	s.HasAuth = err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return s, fmt.Errorf("delegation：讀取代行授權：%w", err)
	}
	s.Auth = auth
	if !st.Operable {
		if st.UnavailableText == "" {
			return s, fmt.Errorf("delegation：店家 %d 不可營運但 seller 沒有給原因", in.StoreID)
		}
		d := model.DenialStoreNotOperable
		if st.Closed {
			d = model.DenialStoreClosed
		}
		s.Denial, s.Label = d, st.UnavailableText
		return s, nil
	}
	switch {
	case !s.HasAuth:
		s.deny(model.DenialAuthorizationNotFound)
	case !model.IsAuthorizationEffective(auth.RevokedAt, auth.EffectiveFrom, auth.EffectiveUntil, in.Now):
		s.deny(model.DenialAuthorizationRevoked)
	case in.Capability != "" && !slices.Contains(auth.GrantedCapabilities, string(in.Capability)):
		s.deny(model.DenialCapabilityDenied)
	}
	return s, nil
}

// SessionAuthority 回答排程發布到點時，代行 session 發起的排程還有沒有權限發布：session 沒有被治理撤銷（已結束或到期不擋）、session 的店家就是排程的店家，而且述詞
// （資格、角色權限、店家可代行、授權有效且含 posts.publish）此刻成立。全部是不鎖的讀取：呼叫者（social 的排程 job）已經鎖了貼文列，
// 再鎖 identity 或 delegation 的列會倒退鎖順序（單體規則 4）。拒絕回 ok 為 false 與原因；session 不存在也是拒絕，只有讀取失敗回錯誤。
func (m *Module) SessionAuthority(ctx context.Context, sessionID, storeID int64) (ok bool, reason string, err error) {
	q := m.q(ctx)
	row, err := q.GetDelegationSession(ctx, sessionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, "找不到代行工作階段", nil
	}
	if err != nil {
		return false, "", fmt.Errorf("delegation：讀取代行：%w", err)
	}
	if model.DelegationStatus(row.Status) == model.DelegationRevoked || row.RevokedAt != nil {
		return false, "代行工作階段已被撤銷", nil
	}
	if row.TargetStoreID != storeID {
		return false, "排程的店家不是代行工作階段的店家", nil
	}
	st, err := m.checkAuthority(ctx, q, authorityIn{
		AdminID: row.ActorAdminID, StoreID: row.TargetStoreID, Now: m.d.Now(),
		Eligibility: true, Permission: true, MissingAdminDenies: true, Capability: model.CapabilityPostsPublish,
	})
	if err != nil {
		return false, "", err
	}
	return st.holds(), st.Label, nil
}
