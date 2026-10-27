package session

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/model"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/adminperm"
)

// idleExpired 回傳閒置逾時是否已到（設計書 56 §7.5.1 第 3 順位）。與 SQL 的 last_active_at > idle_since 互補。
func idleExpired(s store.DelegationDelegatedOperationSession, now time.Time) bool {
	return model.IdleExpired(s.LastActiveAt, now)
}

// delegationStatus 依設計書 56 §7.5.1 的順位推導：撤銷、hard 到期、閒置逾時，否則進行中。兩種到期共用 expired：
// 前端的狀態是三個值的封閉列舉，區分成因是稽核的事。
func delegationStatus(s store.DelegationDelegatedOperationSession, now time.Time) model.DelegationStatus {
	return model.SessionStatus(s.Status, s.ExpiresAt, s.LastActiveAt, now)
}

func toDelegationSession(s store.DelegationDelegatedOperationSession, now time.Time) model.DelegationSession {
	return model.DelegationSession{
		ID:               s.ID,
		DisplayCode:      s.DisplayCode,
		Actor:            model.DelegationActor{AdminID: s.ActorAdminID, DisplayName: s.ActorDisplayName},
		Target:           model.DelegationTarget{StoreID: s.TargetStoreID, SellerAccountID: s.TargetSellerAccountID, StoreName: s.TargetStoreName},
		Reason:           model.DelegationReason(s.ReasonCode),
		ReasonLabel:      s.ReasonLabel,
		CaseReference:    s.CaseReference,
		Capabilities:     model.ToCapabilities(s.Capabilities),
		IssuedAt:         s.IssuedAt,
		ExpiresAt:        s.ExpiresAt,
		EndedAt:          s.EndedAt,
		Status:           delegationStatus(s, now),
		Revision:         s.Revision,
		RevokedAt:        s.RevokedAt,
		RevokedByAdminID: s.RevokedByAdminID,
		RevokeReason:     s.RevokeReason,
	}
}

// DelegationPreflight 依序判斷（設計書 56 §7.1.4 P1）：管理員有沒有開通、店家存不存在、能不能營運、有沒有有效的
// 授權、管理員或店家是不是已經在代行。只讀、可以重複呼叫（入口每 30 秒、focus 與每次建立前都會問）：不寫任何列。
// 讀店家狀態失敗時原樣回傳 seller 的錯誤（seller 在行程內，沒有跨服務的「暫時讀不到」），不回「不允許」（那會告訴操作者
// 一家大概可以代行的店不能代行）。
func (m *Module) DelegationPreflight(ctx context.Context, in model.DelegationPreflightInput) (model.DelegationPreflight, error) {
	return m.preflight(ctx, m.q(ctx), in, m.d.Now(), true)
}

func (m *Module) preflight(ctx context.Context, q *store.Queries, in model.DelegationPreflightInput, now time.Time, checkEligibility bool) (model.DelegationPreflight, error) {
	admin, err := m.d.Identity.Admin(ctx, in.AdminID)
	if err != nil {
		// gateway 從有效的管理員 session 解析出來的 ID 在這裡找不到，是不變量被打破，不是業務的答案。
		return model.DelegationPreflight{}, fmt.Errorf("delegation：讀取管理員 %d：%w", in.AdminID, err)
	}
	res := model.DelegationPreflight{
		Target:                model.DelegationTarget{StoreID: in.StoreID},
		Actor:                 model.DelegationActor{AdminID: admin.ID, DisplayName: admin.Email},
		ReasonOptions:         model.DelegationReasonOptions(),
		Capabilities:          model.Capabilities(),
		TTL:                   model.DelegationTTL,
		IdleTimeout:           model.DelegationIdleTimeout,
		CaseReferenceRequired: model.AnyCaseReferenceRequired(),
	}
	denied := func(d model.DelegationDenial) (model.DelegationPreflight, error) {
		res.Denial, res.DeniedLabel = d, model.DeniedLabel(d)
		return res, nil
	}
	if checkEligibility {
		eligible, err := q.AdminDelegationEligible(ctx, in.AdminID)
		if err != nil {
			return model.DelegationPreflight{}, fmt.Errorf("delegation：讀取管理員代行資格：%w", err)
		}
		if !eligible {
			return denied(model.DenialAdminNotAllowlisted)
		}
	}
	st, err := m.d.StoreState(ctx, in.StoreID)
	switch {
	case err != nil:
		return model.DelegationPreflight{}, err
	case !st.Found:
		return denied(model.DenialStoreNotDelegable)
	}
	res.Target = model.DelegationTarget{StoreID: st.StoreID, SellerAccountID: st.SellerAccountID, StoreName: st.Name}
	if res.Target.StoreID == 0 {
		res.Target.StoreID = in.StoreID
	}
	res.TargetKnown = true

	// 授權在判斷能不能營運之前讀：不允許時也要回「這家店會被核發的集合」。
	auth, err := q.GetDelegationAuthorizationForStore(ctx, in.StoreID)
	hasAuth := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return model.DelegationPreflight{}, fmt.Errorf("delegation：讀取代行授權：%w", err)
	}
	if hasAuth && len(auth.GrantedCapabilities) > 0 {
		res.Capabilities = model.ToCapabilities(auth.GrantedCapabilities)
	}
	if !st.Operable {
		// 句子由 seller 給（StoreActionDelegate 的第一個原因）；沒有句子是程式錯誤，不寫通用句。
		if st.UnavailableText == "" {
			return model.DelegationPreflight{}, fmt.Errorf("delegation：店家 %d 不可營運但 seller 沒有給原因", in.StoreID)
		}
		d := model.DenialStoreNotOperable
		if st.Closed {
			d = model.DenialStoreClosed
		}
		res.Denial, res.DeniedLabel = d, st.UnavailableText
		return res, nil
	}
	// 從來沒有開通（9286，引導去開通）與已撤回或到期（9264，引導去重新開通）刻意分開。
	switch {
	case !hasAuth:
		return denied(model.DenialAuthorizationNotFound)
	case !model.IsAuthorizationEffective(auth.RevokedAt, auth.EffectiveFrom, auth.EffectiveUntil, now):
		return denied(model.DenialAuthorizationRevoked)
	}
	// 管理員與店家分兩句問（OR 會讓兩個 partial unique index 都用不上）；已經到期的列不算衝突。
	if _, err := q.GetActiveDelegationSessionOfActor(ctx, store.GetActiveDelegationSessionOfActorParams{
		ActorAdminID: in.AdminID, Now: now, IdleSince: model.IdleSince(now),
	}); err == nil {
		return denied(model.DenialActorBusy)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return model.DelegationPreflight{}, fmt.Errorf("delegation：查詢管理員進行中的代行：%w", err)
	}
	if _, err := q.GetActiveDelegationSessionOfStore(ctx, store.GetActiveDelegationSessionOfStoreParams{
		TargetStoreID: in.StoreID, Now: now, IdleSince: model.IdleSince(now),
	}); err == nil {
		return denied(model.DenialStoreBusy)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return model.DelegationPreflight{}, fmt.Errorf("delegation：查詢店家進行中的代行：%w", err)
	}
	res.Allowed = true
	return res, nil
}

// displayCodeAttempts 是 display_code 撞號時重試的次數上限（67 bit 的亂數，撞一次都可以忽略）。
const displayCodeAttempts = 3

// StartDelegation 建立代行的 session，或重放同一把冪等鍵的 session（設計書 56 §7.4.1）：
//   - 同一把 key：回既有的 session；尚未兌換時改簽 handoff code，已兌換時不回 code；內容不同是 9267。
//   - 不同的 key、管理員已經在代行：9268；店家已經被代行：9269。
//
// 「同一位管理員、同一家店、不同的 key 就回既有的 session」是禁止的：回應會帶上一次請求的理由與案件編號，前端逐字
// 比對後 502，操作者無從自救。能力由伺服端重新評估：要求的能力不在目前的授權內是 9262，不會被悄悄丟掉。
func (m *Module) StartDelegation(ctx context.Context, in model.StartDelegationInput) (model.StartedDelegation, error) {
	if err := validateStart(in); err != nil {
		return model.StartedDelegation{}, err
	}
	now := m.d.Now()
	q := m.q(ctx)
	existing, err := q.GetDelegationSessionByIdempotencyKey(ctx, store.GetDelegationSessionByIdempotencyKeyParams{
		ActorAdminID: in.AdminID, IdempotencyKey: in.IdempotencyKey,
	})
	if err == nil {
		return m.replayDelegation(ctx, q, existing, in, now)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return model.StartedDelegation{}, fmt.Errorf("delegation：讀取代行的冪等鍵：%w", err)
	}

	pre, err := m.preflight(ctx, q, model.DelegationPreflightInput{AdminID: in.AdminID, StoreID: in.StoreID}, now, false)
	if err != nil {
		return model.StartedDelegation{}, err
	}
	if !pre.Allowed {
		// 同一把 key 的另一個請求可能剛好在上面的查詢之後 commit：它的 session 讓管理員或店家變成「已在代行」（兩個衝突
		// 探測之間 commit 時是後者），答案卻是重放。重放一律優先於目前的判定，與上面的順序一致。
		existing, err := q.GetDelegationSessionByIdempotencyKey(ctx, store.GetDelegationSessionByIdempotencyKeyParams{
			ActorAdminID: in.AdminID, IdempotencyKey: in.IdempotencyKey,
		})
		if err == nil {
			return m.replayDelegation(ctx, q, existing, in, now)
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return model.StartedDelegation{}, fmt.Errorf("delegation：讀取代行的冪等鍵：%w", err)
		}
		// 拒絕原樣轉出（碼與那一句）：9286 帶著「這家店從來沒有開通」，換成模糊的碼只會讓操作者失去它。
		return model.StartedDelegation{}, model.Deny(pre.Denial, "%s", pre.DeniedLabel)
	}
	for _, c := range in.Capabilities {
		if !slices.Contains(pre.Capabilities, c) {
			return model.StartedDelegation{}, model.Deny(model.DenialCapabilityDenied, "請求的能力不在本次核發範圍內")
		}
	}

	issued := now.Truncate(time.Second)
	handoff := model.NewHandoffCode()
	displayName := in.AdminDisplayName
	if displayName == "" {
		displayName = pre.Actor.DisplayName
	}
	params := store.InsertDelegationSessionParams{
		HandoffCodeHash:       model.TokenHash(handoff),
		HandoffExpiresAt:      issued.Add(model.DelegationHandoffTTL),
		ActorAdminID:          in.AdminID,
		ActorDisplayName:      displayName,
		TargetStoreID:         pre.Target.StoreID,
		TargetSellerAccountID: pre.Target.SellerAccountID,
		TargetStoreName:       pre.Target.StoreName,
		ReasonCode:            string(in.Reason),
		ReasonLabel:           in.Reason.Label(),
		CaseReference:         in.CaseReference,
		Capabilities:          model.FromCapabilities(in.Capabilities),
		IssuedAt:              issued,
		ExpiresAt:             issued.Add(time.Duration(in.TTLSeconds) * time.Second),
		IdempotencyKey:        in.IdempotencyKey,
		RequestID:             in.RequestID,
		CorrelationID:         in.CorrelationID,
		UserAgent:             model.Optional(in.UserAgent),
		Now:                   now,
	}
	var out model.StartedDelegation
	err = m.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := m.q(ctx)
		// 權限在這個 transaction 內讀並鎖住角色列（identity 的 FOR SHARE）：只在入口檢查一次的話，入口通過之後、寫入之前
		// 撤掉角色，會留下沒有角色的管理員的代行；撤銷的 DELETE 與這次讀取互相等待。
		allowed, err := m.d.Identity.AdminAllows(ctx, in.AdminID, adminperm.DelegationSessionsCreate)
		if err != nil {
			return fmt.Errorf("delegation：檢查管理員的權限：%w", err)
		}
		if !allowed {
			return model.Deny(model.DenialAdminNotAllowlisted, "此管理員沒有開始代行營運的權限")
		}
		if _, err := q.LockAdminDelegationEligibility(ctx, in.AdminID); errors.Is(err, pgx.ErrNoRows) {
			return model.Deny(model.DenialAdminNotAllowlisted, "此管理員未開通代行營運權限")
		} else if err != nil {
			return fmt.Errorf("delegation：鎖定管理員代行資格：%w", err)
		}
		// 先結束這位管理員與這家店已經到期、卻還佔著 partial unique index 的列（hard 與閒置兩種都要）。
		if _, err := q.EndStaleDelegationSessions(ctx, store.EndStaleDelegationSessionsParams{
			Now: now, IdleSince: model.IdleSince(now), ActorAdminID: in.AdminID, TargetStoreID: in.StoreID,
		}); err != nil {
			return fmt.Errorf("delegation：結束到期的代行：%w", err)
		}
		for attempt := 1; ; attempt++ {
			params.DisplayCode = model.NewDisplayCode()
			row, err := q.InsertDelegationSession(ctx, params)
			if err == nil {
				out = model.StartedDelegation{HandoffCode: handoff, Session: toDelegationSession(row, now)}
				return nil
			}
			if !errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("delegation：建立代行：%w", err)
			}
			// 沒有寫入：判斷撞到哪一個 unique index。同一把 key 的另一個請求先寫入了，它的列就是兩者的答案。
			existing, err := q.GetDelegationSessionByIdempotencyKey(ctx, store.GetDelegationSessionByIdempotencyKeyParams{
				ActorAdminID: in.AdminID, IdempotencyKey: in.IdempotencyKey,
			})
			if err == nil {
				out, err = m.replayDelegation(ctx, q, existing, in, now)
				return err
			}
			if !errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("delegation：讀取代行的冪等鍵：%w", err)
			}
			if taken, err := q.DelegationActorSlotTaken(ctx, in.AdminID); err != nil {
				return fmt.Errorf("delegation：查詢管理員進行中的代行：%w", err)
			} else if taken {
				return model.Deny(model.DenialActorBusy, "%s", model.DeniedLabel(model.DenialActorBusy))
			}
			if taken, err := q.DelegationStoreSlotTaken(ctx, pre.Target.StoreID); err != nil {
				return fmt.Errorf("delegation：查詢店家進行中的代行：%w", err)
			} else if taken {
				return model.Deny(model.DenialStoreBusy, "%s", model.DeniedLabel(model.DenialStoreBusy))
			}
			if attempt == displayCodeAttempts {
				return errors.New("delegation：代行的 display_code 連續撞號")
			}
		}
	})
	return out, err
}

func validateStart(in model.StartDelegationInput) error {
	switch {
	case in.IdempotencyKey == "":
		return model.Deny(model.DenialInvalidCommand, "缺少 Idempotency-Key")
	case in.StoreID <= 0:
		return model.Deny(model.DenialInvalidCommand, "storeId 不正確")
	case in.Reason.Label() == "":
		return model.Deny(model.DenialInvalidCommand, "reasonCode 不在允許的選項內")
	}
	if err := model.ValidateCaseReference(in.CaseReference); err != nil {
		return err
	}
	// 前端也會擋，但藏起來的按鈕不是授權：伺服端再驗一次。
	if in.Reason.CaseReferenceRequired() && in.CaseReference == nil {
		return model.Deny(model.DenialInvalidCommand, "此理由必須填寫案件編號")
	}
	if err := model.ValidateCapabilities("capabilities", in.Capabilities); err != nil {
		return err
	}
	// 超出範圍是錯誤的請求，不悄悄夾住：回應會與要求的不同，下游沒有人會發現。
	if in.TTLSeconds < 1 || time.Duration(in.TTLSeconds)*time.Second > model.DelegationTTL {
		return model.Deny(model.DenialInvalidCommand, "ttlSeconds 必須介於 1 與 %d 之間", int(model.DelegationTTL.Seconds()))
	}
	return nil
}

// replayDelegation 回傳既有的 session；credential 已核發時不再簽發 handoff code。
func (m *Module) replayDelegation(ctx context.Context, q *store.Queries, s store.DelegationDelegatedOperationSession, in model.StartDelegationInput, now time.Time) (model.StartedDelegation, error) {
	if !sameStartPayload(s, in) {
		return model.StartedDelegation{}, model.Deny(model.DenialIdempotencyKeyReused, "冪等鍵已用於不同的請求內容")
	}
	if len(s.CredentialHash) != 0 {
		return model.StartedDelegation{Session: toDelegationSession(s, now)}, nil
	}
	handoff := model.NewHandoffCode()
	row, err := q.ReissueDelegationHandoff(ctx, store.ReissueDelegationHandoffParams{
		HandoffCodeHash: model.TokenHash(handoff), HandoffExpiresAt: now.Truncate(time.Second).Add(model.DelegationHandoffTTL), ID: s.ID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		row, err = q.GetDelegationSession(ctx, s.ID)
		if err != nil {
			return model.StartedDelegation{}, fmt.Errorf("delegation：讀取已兌換的代行：%w", err)
		}
		return model.StartedDelegation{Session: toDelegationSession(row, now)}, nil
	}
	if err != nil {
		return model.StartedDelegation{}, fmt.Errorf("delegation：重簽 handoff code：%w", err)
	}
	return model.StartedDelegation{HandoffCode: handoff, Session: toDelegationSession(row, now)}, nil
}

// sameStartPayload 比對請求與既有的 session。TTL 以 expires_at − issued_at 比對（它變成的值）。
func sameStartPayload(s store.DelegationDelegatedOperationSession, in model.StartDelegationInput) bool {
	if s.TargetStoreID != in.StoreID || s.ReasonCode != string(in.Reason) {
		return false
	}
	if (s.CaseReference == nil) != (in.CaseReference == nil) ||
		(s.CaseReference != nil && *s.CaseReference != *in.CaseReference) {
		return false
	}
	if !model.SameCapabilitySet(model.ToCapabilities(s.Capabilities), in.Capabilities) {
		return false
	}
	return s.ExpiresAt.Sub(s.IssuedAt) == time.Duration(in.TTLSeconds)*time.Second
}

// RedeemDelegationHandoff 以一次性的 handoff code 簽發 session 的 credential（設計書 56 §4.5、A29）。前面的端點沒有
// 任何其他憑證，授權全靠 code：判斷是一句 CAS（見 RedeemDelegationHandoff 的 query），沒有列一律是 9282。
func (m *Module) RedeemDelegationHandoff(ctx context.Context, code string) (model.RedeemedDelegation, error) {
	invalid := model.Deny(model.DenialHandoffInvalid, "此代行連結已失效，請回管理後台重新開始")
	if !model.WellFormedToken(code, model.HandoffPrefix) {
		return model.RedeemedDelegation{}, invalid
	}
	now := m.d.Now()
	credential := model.NewOperationCredential()
	row, err := m.q(ctx).RedeemDelegationHandoff(ctx, store.RedeemDelegationHandoffParams{
		HandoffCodeHash: model.TokenHash(code), CredentialHash: model.TokenHash(credential), Now: now, IdleSince: model.IdleSince(now),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return model.RedeemedDelegation{}, invalid
	}
	if err != nil {
		return model.RedeemedDelegation{}, fmt.Errorf("delegation：兌換 handoff code：%w", err)
	}
	return model.RedeemedDelegation{Credential: credential, Session: toDelegationSession(row, now)}, nil
}

// ResolveDelegation 以 credential 找回 session。activity 為 false 時完全唯讀（GET current 每 30 秒輪詢，輪詢若算活動，
// 閒置逾時永遠不會發生）；為 true 時（代行資源的路由）在同一句順延閒置逾時。兩者回傳的內容相同。
// 終態的 session 照常回傳（Status 是 expired 或 revoked），由呼叫方決定怎麼處理。
func (m *Module) ResolveDelegation(ctx context.Context, credential string, activity bool) (model.DelegationSession, error) {
	if credential == "" {
		return model.DelegationSession{}, model.Deny(model.DenialCredentialInvalid, "缺少代行憑證")
	}
	now := m.d.Now()
	q := m.q(ctx)
	var (
		row store.DelegationDelegatedOperationSession
		err error
	)
	if activity {
		row, err = q.TouchDelegationSession(ctx, store.TouchDelegationSessionParams{
			Now: now, IdleSince: model.IdleSince(now), CredentialHash: model.TokenHash(credential),
		})
	} else {
		row, err = q.GetDelegationSessionByCredential(ctx, model.TokenHash(credential))
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return model.DelegationSession{}, model.Deny(model.DenialCredentialInvalid, "代行憑證無效，請重新建立工作階段")
	}
	if err != nil {
		return model.DelegationSession{}, fmt.Errorf("delegation：以 credential 讀取代行：%w", err)
	}
	return toDelegationSession(row, now), nil
}

// EndDelegation 結束 session（立刻到期）。任何方向都冪等：已經結束、到期、撤銷，甚至對不到任何 session 的 credential
// 都成功——回 401 或 404 會讓前端停在「結果未知」而永遠關不掉工作區。
func (m *Module) EndDelegation(ctx context.Context, credential string) error {
	if credential == "" {
		return nil
	}
	q := m.q(ctx)
	row, err := q.GetDelegationSessionByCredential(ctx, model.TokenHash(credential))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("delegation：以 credential 讀取代行：%w", err)
	}
	if _, err := q.EndDelegationSession(ctx, store.EndDelegationSessionParams{Now: m.d.Now(), ID: row.ID}); err != nil {
		return fmt.Errorf("delegation：結束代行：%w", err)
	}
	return nil
}

// RevokeDelegation 撤銷 session（治理的動作，不是代行的能力）。已經到期或結束的也能撤銷，留下治理的痕跡；重放回傳
// 第一次撤銷的列（revoked_at 不變、revision 不再加一）。
//
// 刻意不檢查管理員有沒有開通代行：撤銷只會讓權限變少，它也是治理側主動止血的路徑，鎖在
// allowlist 後面等於在最需要剎車時讓剎車失效（設計書 56 §6.1.1）。
func (m *Module) RevokeDelegation(ctx context.Context, in model.RevokeDelegationInput) (model.DelegationSession, error) {
	if n := utf8.RuneCountInString(in.Reason); n < 1 || n > 200 {
		return model.DelegationSession{}, model.Deny(model.DenialInvalidCommand, "revokeReason 必須為 1–200 字")
	}
	now := m.d.Now()
	q := m.q(ctx)
	row, err := q.RevokeDelegationSession(ctx, store.RevokeDelegationSessionParams{
		Now: now, RevokeReason: in.Reason, RevokedByAdminID: in.AdminID, ID: in.SessionID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// 已經撤銷過，或不存在。
		row, err = q.GetDelegationSession(ctx, in.SessionID)
		if errors.Is(err, pgx.ErrNoRows) {
			return model.DelegationSession{}, model.Deny(model.DenialSessionNotFound, "找不到指定的代行工作階段")
		}
	}
	if err != nil {
		return model.DelegationSession{}, fmt.Errorf("delegation：撤銷代行：%w", err)
	}
	return toDelegationSession(row, now), nil
}

// RevokeDelegationsOfAdmin 在一個 transaction 內撤銷管理員進行中的代行。
func (m *Module) RevokeDelegationsOfAdmin(ctx context.Context, adminID int64) error {
	return m.d.DB.InTx(ctx, func(ctx context.Context) error {
		return m.revokeDelegationsOfAdmin(ctx, adminID, &adminID, model.RevokeReasonAdminLogout)
	})
}

// OnAdminRoleRevoked 在 identity 撤銷管理員角色的 transaction 內，若剩下的角色不再允許 delegation.sessions.create，
// 撤銷這位管理員進行中的代行，不等代行自己到期。撤銷與事件同一個 transaction：這裡失敗時角色不會被撤銷。
func (m *Module) OnAdminRoleRevoked(ctx context.Context, e identity.AdminRoleRevoked) error {
	allowed, err := m.d.Identity.AdminAllows(ctx, e.AdminID, adminperm.DelegationSessionsCreate)
	if err != nil {
		return fmt.Errorf("delegation：檢查管理員剩下的權限：%w", err)
	}
	if allowed {
		return nil
	}
	return m.revokeDelegationsOfAdmin(ctx, e.AdminID, nil, model.RevokeReasonAdminRoleRevoked)
}

// revokeDelegationsOfAdmin 的 operatorAdminID 為 nil 表示沒有操作者（系統因角色變動撤銷），revoked_by_admin_id 寫 NULL。
func (m *Module) revokeDelegationsOfAdmin(ctx context.Context, adminID int64, operatorAdminID *int64, reason string) error {
	now := m.d.Now()
	ids, err := m.q(ctx).RevokeActiveDelegationsOfActor(ctx, store.RevokeActiveDelegationsOfActorParams{
		ActorAdminID: adminID, Now: now, IdleSince: model.IdleSince(now),
		RevokedByAdminID: operatorAdminID, RevokeReason: reason,
	})
	if err != nil {
		return fmt.Errorf("delegation：撤銷管理員進行中的代行：%w", err)
	}
	if len(ids) > 0 {
		m.d.Logger.InfoContext(ctx, "delegation：撤銷管理員進行中的代行", "admin_id", adminID, "sessions", len(ids))
	}
	return nil
}

// SetAdminEligibility 開通或撤回管理員的代行資格；撤回時一併撤銷進行中的工作階段。
func (m *Module) SetAdminEligibility(ctx context.Context, in model.AdminEligibilityChange) error {
	if in.AdminID <= 0 || in.OperatorAdminID <= 0 {
		return model.Deny(model.DenialAdminRequired, "缺少管理員身分")
	}
	if in.Enabled {
		if _, err := m.d.Identity.Admin(ctx, in.AdminID); err != nil {
			return fmt.Errorf("delegation：讀取管理員：%w", err)
		}
		if err := m.q(ctx).EnableAdminDelegation(ctx, in.AdminID); err != nil {
			return fmt.Errorf("delegation：開通管理員代行資格：%w", err)
		}
		return nil
	}
	return m.d.DB.InTx(ctx, func(ctx context.Context) error {
		if err := m.q(ctx).DisableAdminDelegation(ctx, in.AdminID); err != nil {
			return fmt.Errorf("delegation：撤回管理員代行資格：%w", err)
		}
		return m.revokeDelegationsOfAdmin(ctx, in.AdminID, &in.OperatorAdminID, model.RevokeReasonDelegationDisabled)
	})
}

// AdminEligible 判斷管理員是否已開通代行。
func (m *Module) AdminEligible(ctx context.Context, adminID int64) (bool, error) {
	return m.q(ctx).AdminDelegationEligible(ctx, adminID)
}

func (m *Module) RevokeActiveOfStore(ctx context.Context, in model.RevokeStoreSessionsInput) ([]int64, error) {
	now := in.Now
	ids, err := m.q(ctx).RevokeActiveDelegationsOfStore(ctx, store.RevokeActiveDelegationsOfStoreParams{
		TargetStoreID: in.StoreID, Now: now, IdleSince: model.IdleSince(now),
		RevokedByAdminID: in.AdminID, RevokeReason: model.RevokeReasonAuthorizationRevoked,
	})
	if err != nil {
		return nil, fmt.Errorf("delegation：撤銷店家進行中的代行：%w", err)
	}
	return ids, nil
}
