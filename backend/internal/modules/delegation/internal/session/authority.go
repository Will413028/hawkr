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

// authorityIn 是「這位管理員現在能不能代行這家店」述詞的輸入。述詞是 preflight（開始代行）與排程發布到點重驗共用的唯一一份；
// 並行上限（管理員或店家已在代行）、TTL、閒置不是授權，不在述詞內。
type authorityIn struct {
	AdminID, StoreID int64
	Now              time.Time
	// Eligibility 以不鎖的讀取檢查管理員的代行資格。StartDelegation 在自己的 transaction 內鎖著讀，這裡不重複。
	Eligibility bool
	// Permission 以不鎖的讀取檢查角色有 delegation.sessions.create。StartDelegation 在 transaction 內以 FOR SHARE 讀（與撤銷角色互相等待），這裡不重複。
	Permission bool
	// MissingAdminDenies 為真時管理員不存在是拒絕；否則是錯誤（gateway 從有效的管理員 session 解出的 ID 找不到，是不變量被打破）。
	MissingAdminDenies bool
	// Capability 非空時，店家授權的能力要含它。
	Capability model.Capability
}

// authorityState 是述詞讀到的事實與結論：Denial 為零表示成立。沒走到的步驟，對應的欄位維持零值。
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

// checkAuthority 依序判斷：管理員存在、開通資格、角色權限、店家存在、授權存在、店家可營運、授權有效、授權含指定能力。所有讀取都不鎖；
// 讀取失敗回錯誤，不當成拒絕（店家狀態讀取失敗也是 seller 的錯誤）。
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

	// 授權在判斷能不能營運之前讀：不允許時 preflight 也要回「這家店會被核發的集合」。
	auth, err := q.GetDelegationAuthorizationForStore(ctx, in.StoreID)
	s.HasAuth = err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return s, fmt.Errorf("delegation：讀取代行授權：%w", err)
	}
	s.Auth = auth
	if !st.Operable {
		// 句子由 seller 給（StoreActionDelegate 的第一個原因）；沒有句子是程式錯誤，不寫通用句。
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
	// 從來沒有開通（9286，引導去開通）與已撤回或到期（9264，引導去重新開通）刻意分開。
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
