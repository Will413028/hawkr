package vendorportal

import (
	"bytes"
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

// EnsureInvite 建立或重取一個 RFQ 收件人的邀請，整段在一個 transaction 內。先讀 prospect（Contacts 在這個 transaction 內對
// prospect 列取共享鎖，與 seller 的封存、合併序列化：Ensure 讀到進行中之後，封存要等 Ensure 提交才進得去，之後它的撤銷會
// 撤掉這張邀請；封存先到，Ensure 讀到的就是非進行中），再鎖住那張邀請。鎖的順序固定是 prospect、邀請，與封存（prospect、
// 再撤銷邀請）一致，不會互鎖。
//   - 沒有邀請：prospect 必須進行中（ErrProspectNotActive）、email 收得到信（ErrNoDeliverableEmail），建立（以唯一鍵去重：
//     併發的 Ensure 輸的一方改走下一種，不會撞出 unique violation）。
//   - 有邀請，依序判斷：已撤銷 ErrInviteRevoked；prospect 不是進行中 ErrProspectNotActive；邀請掛的 prospect 與輸入不同
//     InvalidInput（呼叫方 ID 錯了，不改任何東西；合併後邀請已改掛贏家，用輸家的 ID 呼叫會先得到 ErrProspectNotActive）；已標記寄出
//     ErrInviteAlreadyDelivered；其餘重讀 email、換 secret 與到期時間（舊 secret 失效），回傳新的 secret。
//
// ExpiresAt 必須晚於現在、不超過現在起 180 天。邀請內容可以重複兌換直到到期或撤銷。
func (s *Service) EnsureInvite(ctx context.Context, in model.EnsureInviteInput) (model.IssuedInvite, error) {
	if err := requirePositive("ProspectID", in.ProspectID); err != nil {
		return model.IssuedInvite{}, err
	}
	if err := requirePositive("RfqRecipientID", in.RfqRecipientID); err != nil {
		return model.IssuedInvite{}, err
	}
	now := s.d.Now().UTC()
	if !in.ExpiresAt.After(now) || in.ExpiresAt.After(now.Add(model.MaxInviteLifetime)) {
		return model.IssuedInvite{}, apperr.InvalidInputOn([]string{"ExpiresAt"}, "ExpiresAt 必須晚於現在，且不超過 %d 天", int(model.MaxInviteLifetime/(24*time.Hour)))
	}
	var out model.IssuedInvite
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		contact, err := s.d.Contacts.ProspectContact(ctx, in.ProspectID)
		if err != nil {
			return err
		}
		row, err := q.LockInviteByRecipient(ctx, in.RfqRecipientID)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			if !contact.Active {
				return model.ErrProspectNotActive
			}
			email, err := deliverableEmail(contact)
			if err != nil {
				return err
			}
			secret, hash, err := newSecret()
			if err != nil {
				return err
			}
			id := s.d.NewID()
			_, err = q.InsertInviteIfAbsent(ctx, store.InsertInviteIfAbsentParams{
				InviteID: id, RfqRecipientID: in.RfqRecipientID, ProspectID: in.ProspectID, InvitedEmail: email,
				SecretHash: hash, ExpiresAt: in.ExpiresAt.UTC(), Now: now,
			})
			if err == nil {
				out = model.IssuedInvite{InviteID: id, Secret: secret, InvitedEmail: email}
				return nil
			}
			if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			// 併發的另一個 Ensure 先建立了這一列（ON CONFLICT DO NOTHING 等它提交才回來）：鎖住它，走「已存在」。
			if row, err = q.LockInviteByRecipient(ctx, in.RfqRecipientID); err != nil {
				return err
			}
		case err != nil:
			return err
		}
		return s.rotate(ctx, q, row, contact, in, now, &out)
	})
	if err != nil {
		return model.IssuedInvite{}, err
	}
	return out, nil
}

// rotate 處理已存在的邀請（呼叫方已持有列鎖與 prospect 的共享鎖）。
func (s *Service) rotate(ctx context.Context, q *store.Queries, row store.AgentVendorPortalInvite, contact Contact, in model.EnsureInviteInput, now time.Time, out *model.IssuedInvite) error {
	if row.RevokedAt != nil {
		return model.ErrInviteRevoked
	}
	if !contact.Active {
		return model.ErrProspectNotActive
	}
	if row.ProspectID != in.ProspectID {
		return apperr.InvalidInputOn([]string{"ProspectID"}, "RfqRecipientID %d 的邀請屬於別的業者", in.RfqRecipientID)
	}
	if row.SecretDeliveredAt != nil {
		return model.ErrInviteAlreadyDelivered
	}
	email, err := deliverableEmail(contact)
	if err != nil {
		return err
	}
	secret, hash, err := newSecret()
	if err != nil {
		return err
	}
	n, err := q.RotateInviteSecret(ctx, store.RotateInviteSecretParams{
		SecretHash: hash, ExpiresAt: in.ExpiresAt.UTC(), InvitedEmail: email, Now: now, InviteID: row.InviteID,
	})
	if err != nil {
		return err
	}
	if n != 1 {
		// 列鎖之下條件不會變；到這裡代表不變量被破壞，不要把它說成「已寄出」（舊實作的 D2）。
		return errors.New("agent: 邀請在列鎖之下更新了 0 列")
	}
	*out = model.IssuedInvite{InviteID: row.InviteID, Secret: secret, InvitedEmail: email}
	return nil
}

// deliverableEmail 取 prospect 的 email（去前後空白後儲存）；沒有或收不到信回 ErrNoDeliverableEmail。
func deliverableEmail(c Contact) (string, error) {
	if c.Email == nil || !model.IsDeliverableEmail(*c.Email) {
		return "", model.ErrNoDeliverableEmail
	}
	return trimtext.Trim(*c.Email), nil
}

// MarkDelivered 記下邀請的 secret 已經寄出。secret 是寄出的那一把明文：一個條件 UPDATE，只有它的 SHA-256 等於目前的
// secret_hash 才標記，所以 Ensure 在寄信途中換過 secret，過時的 Mark 不生效（ErrInviteSecretSuperseded，寄出的連結已經失效，
// 呼叫方要重新 Ensure）。已用同一把 secret 標記過是冪等成功（不改時間）。邀請不存在回 NotFound；secret 格式不對回 InvalidInput。
func (s *Service) MarkDelivered(ctx context.Context, inviteID uuid.UUID, secret string) error {
	hash, ok := hashSecret(secret)
	if !ok {
		return apperr.InvalidInputOn([]string{"Secret"}, "secret 格式不正確")
	}
	q := store.New(s.d.DB.Conn(ctx))
	now := s.d.Now().UTC()
	n, err := q.MarkInviteDelivered(ctx, store.MarkInviteDeliveredParams{Now: &now, InviteID: inviteID, SecretHash: hash})
	if err != nil || n > 0 {
		return err
	}
	cur, err := q.InviteDelivery(ctx, inviteID)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound("邀請不存在")
	}
	if err != nil {
		return err
	}
	if cur.SecretDeliveredAt != nil && bytes.Equal(cur.SecretHash, hash) {
		return nil
	}
	return model.ErrInviteSecretSuperseded
}

// Preview 回傳遮罩後的 email 與到期時間；沒有任何副作用。不可用（未知、格式不對、過期、撤銷）一律是同一個 NotFound。
func (s *Service) Preview(ctx context.Context, secret string) (model.InvitePreview, error) {
	hash, ok := hashSecret(secret)
	if !ok {
		return model.InvitePreview{}, errInviteNotUsable()
	}
	row, err := store.New(s.d.DB.Conn(ctx)).PreviewInvite(ctx, store.PreviewInviteParams{SecretHash: hash, Now: s.d.Now().UTC()})
	if errors.Is(err, pgx.ErrNoRows) {
		return model.InvitePreview{}, errInviteNotUsable()
	}
	if err != nil {
		return model.InvitePreview{}, err
	}
	return model.InvitePreview{InviteID: row.InviteID, RfqRecipientID: row.RfqRecipientID, MaskedEmail: model.MaskEmail(row.InvitedEmail), ExpiresAt: row.ExpiresAt}, nil
}

// Exchange 以邀請 secret 換一個新的 session：邀請必須沒撤銷、沒到期，到期之前可以重複兌換，每次都發新的 session（只屬於這張
// 邀請，不影響別張邀請的 session）。session 的絕對期限不超過邀請的到期時間；同一張邀請有效的 session 最多 5 個，超過時在
// 同一個 transaction 撤銷最舊的未驗證 session，全部都已驗證才撤銷最舊的；本次新發的不在撤銷之列。邀請列在 transaction 內加鎖，同一張邀請的併發兌換與撤銷序列化。
func (s *Service) Exchange(ctx context.Context, secret string) (model.IssuedSession, error) {
	hash, ok := hashSecret(secret)
	if !ok {
		return model.IssuedSession{}, errInviteNotUsable()
	}
	var out model.IssuedSession
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		now := s.d.Now().UTC()
		inv, err := q.LockUsableInviteBySecret(ctx, store.LockUsableInviteBySecretParams{SecretHash: hash, Now: now})
		if errors.Is(err, pgx.ErrNoRows) {
			return errInviteNotUsable()
		}
		if err != nil {
			return err
		}
		token, tokenHash, err := newSecret()
		if err != nil {
			return err
		}
		absolute := minTime(now.Add(model.SessionAbsoluteTTL), inv.ExpiresAt)
		idle := minTime(now.Add(model.SessionIdleTTL), absolute)
		id := s.d.NewID()
		if err := q.InsertSession(ctx, store.InsertSessionParams{
			SessionID: id, TokenHash: tokenHash, InviteID: inv.InviteID, IdleExpiresAt: idle, AbsoluteExpiresAt: absolute, Now: now,
		}); err != nil {
			return err
		}
		if _, err := q.RevokeSessionsOverCap(ctx, store.RevokeSessionsOverCapParams{
			Now: &now, InviteID: inv.InviteID, CurrentSessionID: id, Keep: model.MaxActiveSessions,
		}); err != nil {
			return err
		}
		out = model.IssuedSession{Token: token, InviteID: inv.InviteID, RfqRecipientID: inv.RfqRecipientID, IdleExpiresAt: idle, AbsoluteExpiresAt: absolute}
		return nil
	})
	if err != nil {
		return model.IssuedSession{}, err
	}
	return out, nil
}

func minTime(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}
