package kyc

import (
	"context"
	"fmt"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
)

// match 比對申請的統編與「別的帳號、處分生效中」的身分快照（ADR 2026-10-19-seller-kyc-payuni-consolidated）：命中就是 blocked，不能通過認證。
//
// 比對用金鑰環的每一把金鑰重算（輪替前寫入的快照才找得到）。快照只回報「哪些帳號的快照有相同的 hash」，措施是否生效一律經 stint.Derive
// （生效中的定義只有 EffectiveMeasures 那一支查詢，這裡不再寫第二份）；處分解除或到期之後快照被刪掉，沒刪之前 Derive 也會擋下，所以不會誤擋。
// 平台不再收銀行帳號與身分證號（Stripe 計畫 D22、D45），所以沒有「銀行帳號相同」的標記。
func (s *Service) match(ctx context.Context, q *store.Queries, row store.SellerKycApplication, accountID int64, now time.Time) (blocked bool, err error) {
	if row.TaxID == nil {
		return false, nil
	}
	matches, err := q.MeasureIdentityMatches(ctx, store.MeasureIdentityMatchesParams{Hashes: s.p.hashes(taxInput(*row.TaxID)), AccountID: accountID})
	if err != nil {
		return false, err
	}
	sanctioned := map[int64]bool{}
	for _, m := range matches {
		active, seen := sanctioned[m.AccountID]
		if !seen {
			suspended, err := stint.Derive(ctx, q, m.AccountID, now)
			if err != nil {
				return false, err
			}
			active = suspended
			sanctioned[m.AccountID] = active
		}
		if active && model.SanctionIdentityKind(m.Kind) == model.IdentityTaxID {
			blocked = true
		}
	}
	return blocked, nil
}

// LinkedAccountHits 反查與一筆措施的身分快照相同的其他帳號（W9）：防詐保存的 account_identity_hashes（跨 stint，個資清理之後仍在）裡，統編對快照的
// TAX_ID、provider 回報的銀行帳號 fingerprint 對快照的 BANK_ACCOUNT（銀行帳號的命中只列出，是否能當依據由 LinkedIdentity.CountsAsGround 決定）。快照與保存表都存
// 金鑰環每一把金鑰算的 (key_id, hash)，一支以索引等值連接的查詢找出命中的帳號，不解密；輪替之後兩邊總有共同的金鑰（舊金鑰的 hash 留著、新金鑰由
// RehashIdentities 補算）。accountID 不為 nil 時只看那個帳號。
func (s *Service) LinkedAccountHits(ctx context.Context, q *store.Queries, measureID uuid.UUID, exceptAccountID int64, accountID *int64) ([]model.LinkedHit, error) {
	rows, err := q.LinkedAccountHits(ctx, store.LinkedAccountHitsParams{MeasureID: measureID, ExceptAccountID: exceptAccountID, AccountID: accountID})
	if err != nil {
		return nil, err
	}
	hits := make([]model.LinkedHit, 0, len(rows))
	for _, r := range rows {
		identity, ok := linkedIdentityOf(r.Field)
		if !ok {
			return nil, fmt.Errorf("seller：關聯帳號比對命中不認得的欄位 %s", r.Field)
		}
		hits = append(hits, model.LinkedHit{AccountID: r.AccountID, Identity: identity})
	}
	return hits, nil
}
