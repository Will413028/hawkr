package kyc

import (
	"context"
	"fmt"
	"strings"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/seal"
)

// 加密欄位的名稱，也是 AAD 的一部分：密文不能在欄位之間、申請之間搬動。
const (
	colIDCardNo    = "id_card_no"
	colRepIDCardNo = "rep_id_card_no"
	colBankAccount = "bank_account"
)

// protector 加密個資並計算 blind index。
type protector struct {
	keys *seal.Keyring
	mac  *seal.MAC
}

// aad 是密文認證的脈絡：申請 ID 與欄位名。
func aad(applicationID uuid.UUID, column string) []byte {
	return []byte(applicationID.String() + "|" + column)
}

// protected 是一個要寫入的識別欄位：密文、blind index 與算出它的金鑰 ID。
type protected struct {
	Enc    []byte
	Hash   []byte
	HashID *string
}

// protect 加密 plaintext，並以 hashInput 計算 blind index（hashInput 已經是帶標籤的輸入）。
func (p protector) protect(applicationID uuid.UUID, column, plaintext string, hashInput []byte) (protected, error) {
	enc, err := p.keys.Seal([]byte(plaintext), aad(applicationID, column))
	if err != nil {
		return protected{}, fmt.Errorf("seller：加密 KYC 欄位：%w", err)
	}
	h := p.mac.Sum(hashInput)
	return protected{Enc: enc, Hash: h.Sum, HashID: &h.KeyID}, nil
}

// open 解密一個識別欄位；沒有值時回 nil。
func (p protector) open(applicationID uuid.UUID, column string, enc []byte) (*string, error) {
	if enc == nil {
		return nil, nil
	}
	plain, err := p.keys.Open(enc, aad(applicationID, column))
	if err != nil {
		return nil, fmt.Errorf("seller：解密 KYC 欄位 %s：%w", column, err)
	}
	s := string(plain)
	return &s, nil
}

// hashes 以金鑰環裡的每一把金鑰計算 value 的 blind index，查詢時比對全部。
func (p protector) hashes(input []byte) [][]byte {
	all := p.mac.SumAll(input)
	out := make([][]byte, 0, len(all))
	for _, h := range all {
		out = append(out, h.Sum)
	}
	return out
}

func idInput(id string) []byte { return model.IdentityInput(model.IdentityIDCard, id) }

func taxInput(id string) []byte { return model.IdentityInput(model.IdentityTaxID, id) }

func bankInput(bankCode, account string) []byte {
	return model.IdentityInput(model.IdentityBankAccount, model.BankIdentityValue(bankCode, account))
}

// CheckKeys 檢查資料庫參照的每一把金鑰 ID 都在設定的金鑰環裡；缺的 ID 一併列在錯誤裡（金鑰 ID 不是機密，值不會出現）。
func (s *Service) CheckKeys(ctx context.Context) error {
	rows, err := store.New(s.d.DB.Conn(ctx)).ReferencedKeyIDs(ctx)
	if err != nil {
		return fmt.Errorf("seller：讀取資料庫參照的金鑰：%w", err)
	}
	var missing []string
	for _, r := range rows {
		has := s.p.keys.Has(r.KeyID)
		if r.Kind == "HASH" {
			has = s.p.mac.Has(r.KeyID)
		}
		if !has {
			missing = append(missing, r.Kind+":"+r.KeyID)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("seller：資料庫參照的金鑰不在設定的金鑰環裡（KYC_ENCRYPTION_KEYS、KYC_HASH_KEYS）：%s", strings.Join(missing, ", "))
	}
	return nil
}
