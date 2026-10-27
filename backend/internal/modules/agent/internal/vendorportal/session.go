package vendorportal

import (
	"context"
	"errors"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/agent/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/agent/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
)

// toInfo 把三個 session 查詢共同的欄位轉成 SessionInfo；三個驗證欄位同有同無（CHECK）。
func toInfo(sessionID, inviteID uuid.UUID, rfqRecipientID int64, method, email *string, at *time.Time) model.SessionInfo {
	out := model.SessionInfo{SessionID: sessionID, InviteID: inviteID, RfqRecipientID: rfqRecipientID}
	if method != nil && email != nil && at != nil {
		out.Verification = &model.Verification{Method: model.VerificationMethod(*method), Email: *email, At: *at}
	}
	return out
}

// Validate 驗證 session token 並回傳 session 的內容。有效 ⇔ session 沒撤銷、idle 與絕對期限都沒到，且邀請沒撤銷、沒到期；
// 有效時在同一個 statement 內滑動 idle 期限（距上次看到不到 5 分鐘不寫入），寫入失敗就是錯誤，不吞掉。不可用（未知、格式不對、
// 過期、撤銷）一律是同一個 Unauthenticated。
func (s *Service) Validate(ctx context.Context, token string) (model.SessionInfo, error) {
	hash, ok := hashSecret(token)
	if !ok {
		return model.SessionInfo{}, errSessionInvalid()
	}
	now := s.d.Now().UTC()
	row, err := store.New(s.d.DB.Conn(ctx)).ValidateSession(ctx, store.ValidateSessionParams{
		TokenHash: hash, Now: now, IdleUntil: now.Add(model.SessionIdleTTL), TouchBefore: now.Add(-model.TouchInterval),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return model.SessionInfo{}, errSessionInvalid()
	}
	if err != nil {
		return model.SessionInfo{}, err
	}
	return toInfo(row.SessionID, row.InviteID, row.RfqRecipientID, row.VerificationMethod, row.VerifiedEmail, row.VerifiedAt), nil
}

// Verify 記下 session 已驗證業者持有邀請的信箱。verifiedEmail 必須是呼叫端已經驗證過的 email（Google token 由入口經 identity
// 驗證，模組只收 email）；去掉頭尾空白（platform/trimtext，與邀請快照的 CHECK 同一個集合）後與邀請的快照不分大小寫相等時，以單一條件 UPDATE（CAS）寫入，比對與存活條件都在述詞裡。
// 已經以相同 email 驗證過的 session 重送是冪等的成功；不符、不可用、已用別的 email 驗證一律是同一個 PermissionDenied。
func (s *Service) Verify(ctx context.Context, token string, method model.VerificationMethod, verifiedEmail string) (model.SessionInfo, error) {
	if !method.Valid() {
		return model.SessionInfo{}, apperr.InvalidInputOn([]string{"Method"}, "不支援的驗證方式")
	}
	hash, ok := hashSecret(token)
	verifiedEmail = trimtext.Trim(verifiedEmail)
	if !ok || verifiedEmail == "" {
		return model.SessionInfo{}, errVerifyDenied()
	}
	q := store.New(s.d.DB.Conn(ctx))
	now := s.d.Now().UTC()
	m := string(method)
	row, err := q.VerifySession(ctx, store.VerifySessionParams{Method: &m, VerifiedEmail: verifiedEmail, Now: &now, TokenHash: hash})
	if err == nil {
		return toInfo(row.SessionID, row.InviteID, row.RfqRecipientID, row.VerificationMethod, row.VerifiedEmail, row.VerifiedAt), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return model.SessionInfo{}, err
	}
	again, err := q.VerifiedSession(ctx, store.VerifiedSessionParams{TokenHash: hash, Method: &m, VerifiedEmail: verifiedEmail, Now: now})
	if errors.Is(err, pgx.ErrNoRows) {
		return model.SessionInfo{}, errVerifyDenied()
	}
	if err != nil {
		return model.SessionInfo{}, err
	}
	return toInfo(again.SessionID, again.InviteID, again.RfqRecipientID, again.VerificationMethod, again.VerifiedEmail, again.VerifiedAt), nil
}
