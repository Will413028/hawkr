package authorization

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/model"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
)

// authorizationEffective 回傳授權此刻是否有效：沒有撤回、已經生效、還沒到期。與 ListDelegationAuthorizations 的 SQL
// 篩選是同一個定義。
func authorizationEffective(a store.DelegationSellerDelegationAuthorization, now time.Time) bool {
	return model.IsAuthorizationEffective(a.RevokedAt, a.EffectiveFrom, a.EffectiveUntil, now)
}

func toDelegationAuthorization(a store.DelegationSellerDelegationAuthorization, admins map[int64]identity.Admin, now time.Time) model.DelegationAuthorization {
	out := model.DelegationAuthorization{
		ID:                    a.ID,
		Source:                model.AuthorizationSource(a.Source),
		SourceReference:       a.SourceReference,
		Capabilities:          model.ToCapabilities(a.GrantedCapabilities),
		EffectiveFrom:         a.EffectiveFrom,
		EffectiveUntil:        a.EffectiveUntil,
		ApprovedByAdminID:     a.ApprovedByAdminID,
		ApprovedByDisplayName: admins[a.ApprovedByAdminID].Email,
		ApprovedAt:            a.ApprovedAt,
		RevokedAt:             a.RevokedAt,
		RevokedByAdminID:      a.RevokedByAdminID,
		RevokeReason:          a.RevokeReason,
		Effective:             authorizationEffective(a, now),
		CreatedAt:             a.CreatedAt,
		UpdatedAt:             a.UpdatedAt,
	}
	if a.TargetStoreID != nil {
		out.StoreID = *a.TargetStoreID
	}
	if a.RevokedByAdminID != nil {
		if revoker, ok := admins[*a.RevokedByAdminID]; ok {
			out.RevokedByDisplayName = &revoker.Email
		}
	}
	return out
}

func toAuthorizationAuditEntry(e store.DelegationDelegationAuthorizationAuditLog) model.AuthorizationAuditEntry {
	out := model.AuthorizationAuditEntry{
		ID:                   e.ID,
		AuthorizationID:      e.AuthorizationID,
		StoreID:              e.TargetStoreID,
		Action:               model.AuthorizationAction(e.Action),
		ActorAdminID:         e.ActorAdminID,
		ActorDisplayName:     e.ActorDisplayName,
		OperationReason:      e.OperationReason,
		SourceReference:      e.SourceReference,
		EffectiveUntilBefore: e.EffectiveUntilBefore,
		EffectiveUntilAfter:  e.EffectiveUntilAfter,
		CreatedAt:            e.CreatedAt,
	}
	if e.Source != nil {
		s := model.AuthorizationSource(*e.Source)
		out.Source = &s
	}
	if e.CapabilitiesBefore != nil {
		out.CapabilitiesBefore = model.ToCapabilities(e.CapabilitiesBefore)
	}
	if e.CapabilitiesAfter != nil {
		out.CapabilitiesAfter = model.ToCapabilities(e.CapabilitiesAfter)
	}
	return out
}

// governingAdmin 是六個治理動作共用的唯一閘門：管理員要開通代行（與發起代行同一把尺）。規則只在這一個地方，
// 日後要「核准者不得是操作者」時改這裡，不是改六個分散的 if。
func (m *Module) governingAdmin(ctx context.Context, adminID int64) (identity.Admin, error) {
	if adminID <= 0 {
		return identity.Admin{}, model.Deny(model.DenialAdminRequired, "缺少管理員身分")
	}
	a, err := m.d.Identity.Admin(ctx, adminID)
	if err != nil {
		// gateway 從有效的管理員 session 解析出來的 ID 在這裡找不到，是不變量被打破。
		return a, fmt.Errorf("delegation：讀取管理員 %d：%w", adminID, err)
	}
	eligible, err := m.d.Session.AdminEligible(ctx, adminID)
	if err != nil {
		return a, fmt.Errorf("delegation：讀取管理員代行資格：%w", err)
	}
	if !eligible {
		return a, model.Deny(model.DenialAdminNotAllowlisted, "此管理員未開通代行營運治理權限")
	}
	return a, nil
}

func (m *Module) authorizationAdmins(ctx context.Context, rows []store.DelegationSellerDelegationAuthorization) (map[int64]identity.Admin, error) {
	ids := make([]int64, 0, len(rows)*2)
	for _, row := range rows {
		ids = append(ids, row.ApprovedByAdminID)
		if row.RevokedByAdminID != nil {
			ids = append(ids, *row.RevokedByAdminID)
		}
	}
	admins, err := m.d.Identity.Admins(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("delegation：讀取授權管理員：%w", err)
	}
	return admins, nil
}

// DelegationAuthorizations 列出店家的授權（治理總覽），新到舊，不含全域列。
func (m *Module) DelegationAuthorizations(ctx context.Context, in model.ListDelegationAuthorizationsInput) (model.DelegationAuthorizationPage, error) {
	adminID, f, p := in.AdminID, in.Filter, in.Paging
	q := m.q(ctx)
	if _, err := m.governingAdmin(ctx, adminID); err != nil {
		return model.DelegationAuthorizationPage{}, err
	}
	switch f.Status {
	case model.AuthorizationAny, model.AuthorizationEffective, model.AuthorizationIneffective:
	default:
		return model.DelegationAuthorizationPage{}, model.Deny(model.DenialInvalidCommand, "status 篩選值不正確")
	}
	if f.StoreID != nil && *f.StoreID <= 0 {
		return model.DelegationAuthorizationPage{}, model.Deny(model.DenialInvalidCommand, "storeId 不正確")
	}
	p, err := p.Normalize()
	if err != nil {
		return model.DelegationAuthorizationPage{}, err
	}
	now := m.d.Now()
	rows, err := q.ListDelegationAuthorizations(ctx, store.ListDelegationAuthorizationsParams{
		TargetStoreID: f.StoreID, Status: string(f.Status), Now: now, RowOffset: p.Offset(), RowLimit: p.PageSize,
	})
	if err != nil {
		return model.DelegationAuthorizationPage{}, fmt.Errorf("delegation：列出代行授權：%w", err)
	}
	total, err := q.CountDelegationAuthorizations(ctx, store.CountDelegationAuthorizationsParams{
		TargetStoreID: f.StoreID, Status: string(f.Status), Now: now,
	})
	if err != nil {
		return model.DelegationAuthorizationPage{}, fmt.Errorf("delegation：計算代行授權數：%w", err)
	}
	page := model.DelegationAuthorizationPage{Total: total, Paging: p, Authorizations: []model.DelegationAuthorization{}}
	admins, err := m.authorizationAdmins(ctx, rows)
	if err != nil {
		return model.DelegationAuthorizationPage{}, err
	}
	for _, r := range rows {
		page.Authorizations = append(page.Authorizations, toDelegationAuthorization(r, admins, now))
	}
	return page, nil
}

// DelegationAuthorizationOfStore 回傳單店的授權面板：只看店家層級的那一列，不退回全域列。
func (m *Module) DelegationAuthorizationOfStore(ctx context.Context, adminID, storeID int64) (model.DelegationAuthorizationPanel, error) {
	q := m.q(ctx)
	if _, err := m.governingAdmin(ctx, adminID); err != nil {
		return model.DelegationAuthorizationPanel{}, err
	}
	if storeID <= 0 {
		return model.DelegationAuthorizationPanel{}, model.Deny(model.DenialInvalidCommand, "storeId 不正確")
	}
	now := m.d.Now()
	count, err := m.d.Session.ActiveSessionsOfStore(ctx, storeID, now)
	if err != nil {
		return model.DelegationAuthorizationPanel{}, fmt.Errorf("delegation：計算店家進行中的代行：%w", err)
	}
	a, ok, err := m.authorizationOfStore(ctx, q, storeID, now)
	if err != nil {
		return model.DelegationAuthorizationPanel{}, err
	}
	return model.DelegationAuthorizationPanel{Exists: ok, Authorization: a, ActiveSessions: count}, nil
}

func (m *Module) authorizationOfStore(ctx context.Context, q *store.Queries, storeID int64, now time.Time) (model.DelegationAuthorization, bool, error) {
	r, err := q.GetDelegationAuthorizationOfStore(ctx, storeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.DelegationAuthorization{}, false, nil
	}
	if err != nil {
		return model.DelegationAuthorization{}, false, fmt.Errorf("delegation：讀取代行授權：%w", err)
	}
	admins, err := m.authorizationAdmins(ctx, []store.DelegationSellerDelegationAuthorization{r})
	if err != nil {
		return model.DelegationAuthorization{}, false, err
	}
	return toDelegationAuthorization(r, admins, now), true, nil
}

func validateAuthorizationChange(in model.DelegationAuthorizationChange) error {
	switch {
	case in.StoreID <= 0:
		return model.Deny(model.DenialInvalidCommand, "storeId 不正確")
	case !in.Source.Valid():
		return model.Deny(model.DenialInvalidCommand, "source 不在允許的選項內")
	}
	// 依據編號：4–200 字、至少一個數字（是編號，不是說明）。DB 的 CHECK 是保證，這裡給訊息（讓 CHECK 擋會中止
	// transaction，變成 500）。
	if n := utf8.RuneCountInString(in.SourceReference); n < 4 || n > 200 {
		return model.Deny(model.DenialInvalidCommand, "sourceReference 長度必須介於 4 與 200 之間")
	}
	if !strings.ContainsAny(in.SourceReference, "0123456789") {
		return model.Deny(model.DenialInvalidCommand, "sourceReference 必須包含至少一個數字")
	}
	if n := utf8.RuneCountInString(in.OperationReason); n < 1 || n > 200 {
		return model.Deny(model.DenialInvalidCommand, "operationReason 必須為 1–200 字")
	}
	// 能力的相依（例如 write 要有 read）由 gateway 的 authz 檢查，那是九個值唯一的擁有者；這裡只守值域。
	return model.ValidateCapabilities("grantedCapabilities", in.Capabilities)
}

// GrantDelegationAuthorization 開通一家店，或重新開通已撤回的店（每店終生一列，重新開通是同一列）。已經有有效的授權
// 時是 9285；同一把冪等鍵重放時回第一次的答案。store 是呼叫方讀到的店家狀態：店家不存在時不開通（target_store_id
// 沒有 FK，這是唯一擋住打錯 ID 的地方）。
func (m *Module) GrantDelegationAuthorization(ctx context.Context, in model.DelegationAuthorizationChange) (model.GrantedAuthorization, error) {
	admin, err := m.governingAdmin(ctx, in.AdminID)
	if err != nil {
		return model.GrantedAuthorization{}, err
	}
	if err := validateAuthorizationChange(in); err != nil {
		return model.GrantedAuthorization{}, err
	}
	now := m.d.Now()
	// 生效時間是 now：已經過去的期限會違反 CHECK。
	if in.EffectiveUntil != nil && !in.EffectiveUntil.After(now) {
		return model.GrantedAuthorization{}, model.Deny(model.DenialInvalidCommand, "effectiveUntil 必須晚於現在")
	}
	st, err := m.d.StoreState(ctx, in.StoreID)
	switch {
	case err != nil:
		return model.GrantedAuthorization{}, err
	case !st.Found:
		return model.GrantedAuthorization{}, model.Deny(model.DenialStoreNotDelegable, "此店家不存在或已刪除，無法開通代行營運")
	case st.Closed:
		return model.GrantedAuthorization{}, model.Deny(model.DenialStoreClosed, "%s", st.UnavailableText)
	}

	var out model.GrantedAuthorization
	err = m.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := m.q(ctx)
		before, found, err := lockAuthorization(ctx, q, in.StoreID)
		if err != nil {
			return err
		}
		id := m.d.NewID()
		granted, err := q.GrantDelegationAuthorization(ctx, store.GrantDelegationAuthorizationParams{
			ID: id, TargetStoreID: in.StoreID, Source: string(in.Source), SourceReference: in.SourceReference,
			GrantedCapabilities: model.FromCapabilities(in.Capabilities), Now: now, EffectiveUntil: in.EffectiveUntil,
			ApprovedByAdminID: in.AdminID,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			out, err = m.grantReplay(ctx, q, in, before, found, now)
			return err
		}
		if err != nil {
			return fmt.Errorf("delegation：開通代行授權：%w", err)
		}
		// 重新開通時主鍵不變，回傳的 id 是哪一個分支唯一可信的證據。
		outcome, action := model.GrantReinstated, model.AuthorizationReinstate
		if granted.ID == id {
			outcome, action = model.GrantCreated, model.AuthorizationGrant
		}
		out.Outcome = outcome
		if err := m.writeAuthorizationAudit(ctx, q, authorizationAudit{
			action: action, admin: admin, row: granted, before: before, found: found,
			reason: in.OperationReason, key: in.IdempotencyKey, requestID: in.RequestID, correlationID: in.CorrelationID,
		}, now); err != nil {
			return err
		}
		out.Authorization, _, err = m.authorizationOfStore(ctx, q, in.StoreID, now)
		return err
	})
	return out, err
}

// grantReplay 處理 Grant 沒有寫入的分支：已經有有效的授權。兩種成因要相反的答案：別人開通了這家店（9285），或這個
// 請求第二次抵達（回應遺失）——後者要回第一次的答案。治理歷史分得出兩者：它與授權同一個 transaction 寫入，所以這把
// 鍵在這列授權上有紀錄，若且唯若這把鍵開通了它。
func (m *Module) grantReplay(ctx context.Context, q *store.Queries, in model.DelegationAuthorizationChange, before store.DelegationSellerDelegationAuthorization, found bool, now time.Time) (model.GrantedAuthorization, error) {
	alreadyActive := model.Deny(model.DenialAuthorizationAlreadyActive, "此店家已開通代行營運授權")
	if in.IdempotencyKey == "" {
		return model.GrantedAuthorization{}, alreadyActive
	}
	if !found {
		// 另一個 transaction 在鎖定讀取與 upsert 之間建立了這一列：現在讀得到它。
		var err error
		if before, found, err = lockAuthorization(ctx, q, in.StoreID); err != nil {
			return model.GrantedAuthorization{}, err
		}
		if !found {
			return model.GrantedAuthorization{}, alreadyActive
		}
	}
	e, err := q.GetDelegationAuthorizationAuditLogByIdempotencyKey(ctx, store.GetDelegationAuthorizationAuditLogByIdempotencyKeyParams{
		AuthorizationID: before.ID, IdempotencyKey: &in.IdempotencyKey,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return model.GrantedAuthorization{}, alreadyActive
	}
	if err != nil {
		return model.GrantedAuthorization{}, fmt.Errorf("delegation：讀取治理歷史的冪等鍵：%w", err)
	}
	var outcome model.GrantOutcome
	switch model.AuthorizationAction(e.Action) {
	case model.AuthorizationGrant:
		outcome = model.GrantCreated
	case model.AuthorizationReinstate:
		outcome = model.GrantReinstated
	case model.AuthorizationUpdate, model.AuthorizationRevoke:
	}
	if outcome == "" || !sameAuthorizationChange(e, in) {
		return model.GrantedAuthorization{}, errKeyReusedForAuthorization()
	}
	a, _, err := m.authorizationOfStore(ctx, q, in.StoreID, now)
	return model.GrantedAuthorization{Authorization: a, Outcome: outcome}, err
}

// UpdateDelegationAuthorization 調整有效授權的範圍、依據與期限（全量取代）。刻意與開通分開（CAS 的述詞相反）：合成一個
// upsert 的話，過時的畫面按「調整範圍」會把已撤回的店靜默地重新開通。沒有授權是 9286，已撤回是 9264（引導去開通與
// 去重新開通，不能合成一個模糊的錯誤）。
func (m *Module) UpdateDelegationAuthorization(ctx context.Context, in model.DelegationAuthorizationChange) (model.DelegationAuthorization, error) {
	admin, err := m.governingAdmin(ctx, in.AdminID)
	if err != nil {
		return model.DelegationAuthorization{}, err
	}
	if err := validateAuthorizationChange(in); err != nil {
		return model.DelegationAuthorization{}, err
	}
	now := m.d.Now()
	var out model.DelegationAuthorization
	err = m.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := m.q(ctx)
		before, found, err := lockAuthorization(ctx, q, in.StoreID)
		switch {
		case err != nil:
			return err
		case !found:
			return model.Deny(model.DenialAuthorizationNotFound, "找不到此店家的代行營運授權設定")
		case before.RevokedAt != nil:
			return model.Deny(model.DenialAuthorizationRevoked, "此店家的代行營運授權已撤回，請改用重新開通")
		case in.EffectiveUntil != nil && !in.EffectiveUntil.After(before.EffectiveFrom):
			// 調整不動生效時間，期限與這一列自己的生效時間比。
			return model.Deny(model.DenialInvalidCommand, "effectiveUntil 必須晚於授權生效時間")
		}
		// 重放的檢查在寫入之前：這個 CAS 第二次照樣成立，會把同樣的值再套一次（updated_at 往前推，而且第二列歷史被
		// 唯一鍵擋下之後，這次變更就沒有任何紀錄）。
		if in.IdempotencyKey != "" {
			e, err := q.GetDelegationAuthorizationAuditLogByIdempotencyKey(ctx, store.GetDelegationAuthorizationAuditLogByIdempotencyKeyParams{
				AuthorizationID: before.ID, IdempotencyKey: &in.IdempotencyKey,
			})
			switch {
			case err == nil:
				if model.AuthorizationAction(e.Action) != model.AuthorizationUpdate || !sameAuthorizationChange(e, in) {
					return errKeyReusedForAuthorization()
				}
				out, _, err = m.authorizationOfStore(ctx, q, in.StoreID, now)
				return err
			case !errors.Is(err, pgx.ErrNoRows):
				return fmt.Errorf("delegation：讀取治理歷史的冪等鍵：%w", err)
			}
		}
		updated, err := q.UpdateDelegationAuthorization(ctx, store.UpdateDelegationAuthorizationParams{
			Source: string(in.Source), SourceReference: in.SourceReference, GrantedCapabilities: model.FromCapabilities(in.Capabilities),
			EffectiveUntil: in.EffectiveUntil, Now: now, TargetStoreID: in.StoreID,
		})
		if err != nil {
			// 上面鎖住的讀取已經排除兩種成因，沒有列只可能是不變量被打破。
			return fmt.Errorf("delegation：調整代行授權：%w", err)
		}
		if err := m.writeAuthorizationAudit(ctx, q, authorizationAudit{
			action: model.AuthorizationUpdate, admin: admin, row: updated, before: before, found: true,
			reason: in.OperationReason, key: in.IdempotencyKey, requestID: in.RequestID, correlationID: in.CorrelationID,
		}, now); err != nil {
			return err
		}
		out, _, err = m.authorizationOfStore(ctx, q, in.StoreID, now)
		return err
	})
	return out, err
}

// RevokeDelegationAuthorization 撤回一家店的授權，並在同一個 transaction 內中止這家店進行中的代行（分開做會留下「授權
// 已撤回、代行還在跑」的窗口）。已經撤回時成功、保留第一次的撤回，不補寫第二列歷史。
func (m *Module) RevokeDelegationAuthorization(ctx context.Context, in model.DelegationAuthorizationRevocation) (model.RevokedAuthorization, error) {
	admin, err := m.governingAdmin(ctx, in.AdminID)
	if err != nil {
		return model.RevokedAuthorization{}, err
	}
	if in.StoreID <= 0 {
		return model.RevokedAuthorization{}, model.Deny(model.DenialInvalidCommand, "storeId 不正確")
	}
	if n := utf8.RuneCountInString(in.Reason); n < 1 || n > 200 {
		return model.RevokedAuthorization{}, model.Deny(model.DenialInvalidCommand, "revokeReason 必須為 1–200 字")
	}
	now := m.d.Now()
	var out model.RevokedAuthorization
	err = m.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := m.q(ctx)
		before, found, err := lockAuthorization(ctx, q, in.StoreID)
		if err != nil {
			return err
		}
		if !found {
			return model.Deny(model.DenialAuthorizationNotFound, "找不到此店家的代行營運授權設定")
		}
		revoked, err := q.RevokeDelegationAuthorization(ctx, store.RevokeDelegationAuthorizationParams{
			Now: now, RevokedByAdminID: in.AdminID, RevokeReason: in.Reason, TargetStoreID: in.StoreID,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			out = model.RevokedAuthorization{AlreadyRevoked: true, RevokedSessions: []int64{}}
			out.Authorization, _, err = m.authorizationOfStore(ctx, q, in.StoreID, now)
			return err
		}
		if err != nil {
			return fmt.Errorf("delegation：撤回代行授權：%w", err)
		}
		// 固定的理由：稽核以它機械地區分撤銷的來源。要找回是哪一次治理動作，比對同一個 transaction 的時刻（這家店
		// REVOKE 那一列的 created_at 截秒等於 session 的 revoked_at）。
		sessions, err := m.d.Session.RevokeActiveOfStore(ctx, model.RevokeStoreSessionsInput{StoreID: in.StoreID, AdminID: in.AdminID, Now: now})
		if err != nil {
			return fmt.Errorf("delegation：撤銷店家的代行：%w", err)
		}
		if err := m.writeAuthorizationAudit(ctx, q, authorizationAudit{
			action: model.AuthorizationRevoke, admin: admin, row: revoked, before: before, found: true,
			reason: in.Reason, key: in.IdempotencyKey, requestID: in.RequestID, correlationID: in.CorrelationID,
		}, now); err != nil {
			return err
		}
		out = model.RevokedAuthorization{RevokedSessions: append([]int64{}, sessions...)}
		out.Authorization, _, err = m.authorizationOfStore(ctx, q, in.StoreID, now)
		return err
	})
	return out, err
}

// DelegationAuthorizationHistory 回傳單店的治理歷史，新到舊。從來沒有開通的店是 9286，不是空的一頁：「沒有授權」與
// 「有授權但沒改過」是兩回事，而後者不會發生（開通本身就寫第一列）。
func (m *Module) DelegationAuthorizationHistory(ctx context.Context, in model.DelegationAuthorizationHistoryInput) (model.AuthorizationHistoryPage, error) {
	adminID, storeID, p := in.AdminID, in.StoreID, in.Paging
	q := m.q(ctx)
	if _, err := m.governingAdmin(ctx, adminID); err != nil {
		return model.AuthorizationHistoryPage{}, err
	}
	if storeID <= 0 {
		return model.AuthorizationHistoryPage{}, model.Deny(model.DenialInvalidCommand, "storeId 不正確")
	}
	p, err := p.Normalize()
	if err != nil {
		return model.AuthorizationHistoryPage{}, err
	}
	if _, ok, err := m.authorizationOfStore(ctx, q, storeID, m.d.Now()); err != nil {
		return model.AuthorizationHistoryPage{}, err
	} else if !ok {
		return model.AuthorizationHistoryPage{}, model.Deny(model.DenialAuthorizationNotFound, "找不到此店家的代行營運授權設定")
	}
	rows, err := q.ListDelegationAuthorizationAuditLogs(ctx, store.ListDelegationAuthorizationAuditLogsParams{
		TargetStoreID: storeID, RowOffset: p.Offset(), RowLimit: p.PageSize,
	})
	if err != nil {
		return model.AuthorizationHistoryPage{}, fmt.Errorf("delegation：列出治理歷史：%w", err)
	}
	total, err := q.CountDelegationAuthorizationAuditLogs(ctx, storeID)
	if err != nil {
		return model.AuthorizationHistoryPage{}, fmt.Errorf("delegation：計算治理歷史數：%w", err)
	}
	page := model.AuthorizationHistoryPage{Total: total, Paging: p, Entries: []model.AuthorizationAuditEntry{}}
	for _, r := range rows {
		page.Entries = append(page.Entries, toAuthorizationAuditEntry(r))
	}
	return page, nil
}

// lockAuthorization 鎖住店家層級的授權列並回傳變更前的狀態；沒有這一列時 found 是 false。它不參與判定（判定一律看 CAS
// 的列數），只讓歷史的 before 真的是這次取代的狀態。
func lockAuthorization(ctx context.Context, q *store.Queries, storeID int64) (store.DelegationSellerDelegationAuthorization, bool, error) {
	a, err := q.LockDelegationAuthorizationOfStore(ctx, storeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, false, nil
	}
	if err != nil {
		return a, false, fmt.Errorf("delegation：鎖定代行授權：%w", err)
	}
	return a, true, nil
}

// sameAuthorizationChange 回傳既有的歷史是不是同一個請求寫的。operationReason 不比（它是動作的理由，不是內容：換個說法
// 重試不該變成 9267）；能力以集合比。REVOKE 的歷史沒有 source，永遠不相同。
func sameAuthorizationChange(e store.DelegationDelegationAuthorizationAuditLog, in model.DelegationAuthorizationChange) bool {
	if e.Source == nil || *e.Source != string(in.Source) || e.SourceReference == nil || *e.SourceReference != in.SourceReference {
		return false
	}
	if !model.SameCapabilitySet(model.ToCapabilities(e.CapabilitiesAfter), in.Capabilities) {
		return false
	}
	switch {
	case e.EffectiveUntilAfter == nil || in.EffectiveUntil == nil:
		return e.EffectiveUntilAfter == nil && in.EffectiveUntil == nil
	default:
		return e.EffectiveUntilAfter.Equal(*in.EffectiveUntil)
	}
}

// errKeyReusedForAuthorization 是「這把鍵已經在這家店做過別的事」的唯一答案；恢復方式都是換一把鍵。
func errKeyReusedForAuthorization() error {
	return model.Deny(model.DenialIdempotencyKeyReused, "此冪等鍵已用於這家店的另一次治理動作，請改用新的 Idempotency-Key")
}

type authorizationAudit struct {
	action        model.AuthorizationAction
	admin         identity.Admin
	row           store.DelegationSellerDelegationAuthorization
	before        store.DelegationSellerDelegationAuthorization
	found         bool
	reason        string
	key           string
	requestID     string
	correlationID string
}

// writeAuthorizationAudit 在呼叫方的 transaction 內寫一列治理歷史。冪等鍵已經在這列授權上做過事時回 9267：授權的寫入
// 已經執行了，讓它沒有紀錄地 commit 正是這張表要防的事，所以整個 transaction rollback。
func (m *Module) writeAuthorizationAudit(ctx context.Context, q *store.Queries, a authorizationAudit, now time.Time) error {
	params := store.InsertDelegationAuthorizationAuditLogParams{
		ID:               m.d.NewID(),
		AuthorizationID:  a.row.ID,
		Action:           string(a.action),
		ActorAdminID:     a.admin.ID,
		ActorDisplayName: a.admin.Email,
		OperationReason:  a.reason,
		// 撤回不改能力，before 與 after 相同；是 action 說明授權沒了。
		CapabilitiesAfter:   a.row.GrantedCapabilities,
		EffectiveUntilAfter: a.row.EffectiveUntil,
		IdempotencyKey:      model.Optional(a.key),
		RequestID:           model.Optional(a.requestID),
		CorrelationID:       model.Optional(a.correlationID),
		Now:                 now,
	}
	if a.row.TargetStoreID != nil {
		params.TargetStoreID = *a.row.TargetStoreID
	}
	if a.found {
		params.CapabilitiesBefore = a.before.GrantedCapabilities
		params.EffectiveUntilBefore = a.before.EffectiveUntil
	}
	// 撤回不改依據；記下沒變的值會讀成「這次撤回把依據設成 X」。
	if a.action != model.AuthorizationRevoke {
		params.Source, params.SourceReference = &a.row.Source, &a.row.SourceReference
	}
	_, err := q.InsertDelegationAuthorizationAuditLog(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		return errKeyReusedForAuthorization()
	}
	if err != nil {
		return fmt.Errorf("delegation：寫入治理歷史：%w", err)
	}
	return nil
}
