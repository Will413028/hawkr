package kyc

import (
	"bytes"
	"context"
	"fmt"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
)

// 比對 hash 的欄位名（kyc_identity_hashes.field）：身分證號、企業負責人身分證號、統編、銀行帳號。
const (
	fieldIDCard    = "ID_CARD_NO"
	fieldRepIDCard = "REP_ID_CARD_NO"
	fieldTaxID     = "TAX_ID"
	fieldBank      = "BANK_ACCOUNT"
)

var hashFields = []string{fieldIDCard, fieldRepIDCard, fieldTaxID, fieldBank}

// LinkedField 把比對 hash 的欄位對到關聯帳號命中的身分種類。
func linkedIdentityOf(field string) (model.LinkedIdentity, bool) {
	switch field {
	case fieldIDCard:
		return model.LinkedNationalID, true
	case fieldRepIDCard:
		return model.LinkedResponsiblePersonID, true
	case fieldTaxID:
		return model.LinkedUBN, true
	}
	return "", false
}

// storedHash 是申請上這個欄位寫入當下第一把金鑰的 hash 與金鑰 ID，沒有值時是 nil。
func storedHash(row store.SellerKycApplication, field string) ([]byte, *string) {
	switch field {
	case fieldIDCard:
		return row.IDCardHash, row.IDCardHashKey
	case fieldRepIDCard:
		return row.RepIDCardHash, row.RepIDCardHashKey
	case fieldTaxID:
		return row.TaxIDHash, row.TaxIDHashKey
	default:
		return row.BankAccountHash, row.BankAccountHashKey
	}
}

func sameStoredHash(a, b store.SellerKycApplication, field string) bool {
	ha, ka := storedHash(a, field)
	hb, kb := storedHash(b, field)
	if (ka == nil) != (kb == nil) {
		return false
	}
	return bytes.Equal(ha, hb) && (ka == nil || *ka == *kb)
}

// hashInput 是這個欄位的 blind index 輸入（需要解密的欄位在這裡解密）；欄位沒有值時 ok 為 false。
func (p protector) hashInput(row store.SellerKycApplication, field string) (input []byte, ok bool, err error) {
	switch field {
	case fieldIDCard:
		id, err := p.open(row.ApplicationID, colIDCardNo, row.IDCardNoEnc)
		if err != nil || id == nil {
			return nil, false, err
		}
		return idInput(*id), true, nil
	case fieldRepIDCard:
		id, err := p.open(row.ApplicationID, colRepIDCardNo, row.RepIDCardNoEnc)
		if err != nil || id == nil {
			return nil, false, err
		}
		return idInput(*id), true, nil
	case fieldTaxID:
		if row.TaxID == nil {
			return nil, false, nil
		}
		return taxInput(*row.TaxID), true, nil
	default:
		if row.BankCode == nil {
			return nil, false, nil
		}
		account, err := p.open(row.ApplicationID, colBankAccount, row.BankAccountEnc)
		if err != nil || account == nil {
			return nil, false, err
		}
		return bankInput(*row.BankCode, *account), true, nil
	}
}

// writeHashes 把這個欄位金鑰環每一把金鑰算的 blind index 寫進 kyc_identity_hashes（已存在的略過）。
func (p protector) writeHashes(ctx context.Context, q *store.Queries, row store.SellerKycApplication, field string) error {
	input, ok, err := p.hashInput(row, field)
	if err != nil || !ok {
		return err
	}
	for _, h := range p.mac.SumAll(input) {
		if err := q.InsertKYCIdentityHash(ctx, store.InsertKYCIdentityHashParams{
			ApplicationID: row.ApplicationID, Field: field, KeyID: h.KeyID, Hash: h.Sum,
		}); err != nil {
			return err
		}
	}
	return nil
}

// syncHashes 在寫入申請的同一個 transaction 內讓 kyc_identity_hashes 與申請的識別欄位一致：跟寫入前的列（before，新建時是 nil）比，
// 欄位的值有變（hash 不同）才刪掉重寫，沒動的欄位不解密也不寫。
func (p protector) syncHashes(ctx context.Context, q *store.Queries, before *store.SellerKycApplication, after store.SellerKycApplication) error {
	for _, field := range hashFields {
		if before != nil && sameStoredHash(*before, after, field) {
			continue
		}
		if err := q.DeleteKYCIdentityHashes(ctx, store.DeleteKYCIdentityHashesParams{ApplicationID: after.ApplicationID, Field: field}); err != nil {
			return err
		}
		if err := p.writeHashes(ctx, q, after, field); err != nil {
			return err
		}
	}
	return nil
}

// rehashBatch 是補算一批的申請數。
const rehashBatch = 100

// RehashIdentities 補算金鑰環每一把金鑰的比對 hash（輪替之後新增的金鑰，既有申請還沒有它的列）：只解密缺列的申請，鎖住申請列（與草稿寫入互斥，
// 不會把剛被改掉的欄位的舊值寫回去）。已經齊全的申請不會被讀到，所以沒有輪替時什麼都不做。回傳補算的申請數；啟動時呼叫，可重複執行。
func (s *Service) RehashIdentities(ctx context.Context) (int, error) {
	total := 0
	for _, keyID := range s.p.mac.IDs() {
		var after *uuid.UUID
		for {
			n, last, err := s.rehashBatchFor(ctx, keyID, after)
			if err != nil {
				return total, err
			}
			total += n
			if n < rehashBatch {
				break
			}
			after = &last
		}
	}
	return total, nil
}

func (s *Service) rehashBatchFor(ctx context.Context, keyID string, after *uuid.UUID) (int, uuid.UUID, error) {
	var count int
	var last uuid.UUID
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		params := store.KYCApplicationsMissingHashKeyParams{KeyID: keyID, AfterID: after, BatchLimit: rehashBatch}
		rows, err := q.KYCApplicationsMissingHashKey(ctx, params)
		if err != nil {
			return err
		}
		for _, row := range rows {
			for _, field := range hashFields {
				if err := s.p.writeHashes(ctx, q, row, field); err != nil {
					return fmt.Errorf("seller：補算申請 %s 的 %s 比對 hash：%w", row.ApplicationID, field, err)
				}
			}
			last = row.ApplicationID
		}
		count = len(rows)
		return nil
	})
	return count, last, err
}
