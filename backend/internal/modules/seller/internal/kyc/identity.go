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

// match 比對申請的身分與「別的帳號、處分生效中」的身分快照（ADR 2026-10-19-seller-kyc-payuni-consolidated）：
//
//   - blocked：身分證號（含企業負責人）或統編命中，不能通過認證；
//   - bankShared：銀行帳號命中，只標記不擋（家人或配偶共用帳戶是常態）。
//
// 比對用金鑰環的每一把金鑰重算（輪替前寫入的快照才找得到），所以要解密身分證號與銀行帳號。快照只回報「哪些帳號的快照
// 有相同的 hash」，措施是否生效一律經 stint.Derive（生效中的定義只有 EffectiveMeasures 那一支查詢，這裡不再寫第二份）；
// 處分解除或到期之後快照被刪掉，沒刪之前 Derive 也會擋下，所以不會誤擋。
func (s *Service) match(ctx context.Context, q *store.Queries, row store.SellerKycApplication, accountID int64, now time.Time) (blocked, bankShared bool, err error) {
	var hashes [][]byte
	for _, field := range []struct {
		column string
		enc    []byte
	}{{colIDCardNo, row.IDCardNoEnc}, {colRepIDCardNo, row.RepIDCardNoEnc}} {
		id, err := s.p.open(row.ApplicationID, field.column, field.enc)
		if err != nil {
			return false, false, err
		}
		if id != nil {
			hashes = append(hashes, s.p.hashes(idInput(*id))...)
		}
	}
	if row.TaxID != nil {
		hashes = append(hashes, s.p.hashes(taxInput(*row.TaxID))...)
	}
	if row.BankCode != nil {
		account, err := s.p.open(row.ApplicationID, colBankAccount, row.BankAccountEnc)
		if err != nil {
			return false, false, err
		}
		if account != nil {
			hashes = append(hashes, s.p.hashes(bankInput(*row.BankCode, *account))...)
		}
	}
	if len(hashes) == 0 {
		return false, false, nil
	}
	matches, err := q.MeasureIdentityMatches(ctx, store.MeasureIdentityMatchesParams{Hashes: hashes, AccountID: accountID})
	if err != nil {
		return false, false, err
	}
	sanctioned := map[int64]bool{}
	for _, m := range matches {
		active, seen := sanctioned[m.AccountID]
		if !seen {
			suspended, err := stint.Derive(ctx, q, m.AccountID, now)
			if err != nil {
				return false, false, err
			}
			active = suspended
			sanctioned[m.AccountID] = active
		}
		if !active {
			continue
		}
		switch model.SanctionIdentityKind(m.Kind) {
		case model.IdentityIDCard, model.IdentityTaxID:
			blocked = true
		case model.IdentityBankAccount:
			bankShared = true
		}
	}
	return blocked, bankShared, nil
}

// LinkedAccountHits 反查與一筆措施的身分快照相同的其他帳號（W9）：別的帳號的申請（跨 stint、含已取消的）裡，身分證號（本人與企業負責人）對快照的
// ID_CARD、統一編號對 TAX_ID，銀行帳號不算。快照與申請都存金鑰環每一把金鑰算的 (key_id, hash)（kyc_identity_hashes、measure_identities），一支以索引等值連接的查詢
// 找出命中的帳號，不解密任何申請；輪替之後兩邊總有共同的金鑰（舊金鑰的 hash 留著、新金鑰由 RehashIdentities 補算）。accountID 不為 nil 時只看那個帳號。
// 個資清理過的申請沒有這些列，不命中。
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
