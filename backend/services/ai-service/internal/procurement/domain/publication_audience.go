package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
)

// WishPoolPolicyVersionV1 是 codemeta.ProcurementWishPoolPolicyAllActiveSellersV1
// 的版號。它不在 CodeMeta 裡，因為 generate.sh 產出的 CodeMeta struct 只有
// Code／Value／Label／Description 四欄，放不下數字版號；版號釘在這裡——這裡就是
// 唯一把它寫進 audienceHash preimage 與 audience_policy_version 欄的地方。
//
// 政策演進一律是**新增值**（`..._V2` ＋ 新常數），不得改這個數字或改 v1 的語意：
// 已核准的 publication 依它自己記下的 policy id ＋ version 解讀受眾。
const WishPoolPolicyVersionV1 = 1

// PublicationRecipient 是 DIRECT_RECIPIENTS 受眾裡的一位收件人。身分凍結在核准
// 當下，不是即時查 seller-service。
//
// ⛔ **兩條 lane 的身分住在不同的欄位，不可互換**（procurement_candidates 的既有
// 形狀，#2377／#3182）：
//
//	PLATFORM → StoreID    （平台賣家；此時 ProspectID 恆為 0）
//	EXTERNAL → ProspectID （seller-service 的站外業者主檔；此時 StoreID 恆為 0）
//
// 這不是「兩個欄位挑一個填」的方便寫法，而是本型別存在的理由：EXTERNAL 候選的
// store_id 在 DB 裡恆為 0，把它當身分寫進 audienceHash 會讓**所有**站外業者塌成
// 同一個值——買家核准發給業者 A，execute 送給業者 B 也能通過 CAS。用 Lane 當
// 判別式、各自帶各自的 id，才擋得住。
//
// 一律用 PlatformRecipient／ExternalRecipient 建構，不要自己填 struct literal。
type PublicationRecipient struct {
	StoreID    int64
	Lane       QuoteLane
	ProspectID int64
}

// PlatformRecipient／ExternalRecipient 是唯二的建構入口。分成兩支具名函式而不是
// 一支帶 lane 參數的，是為了讓「lane 與 id 欄位配錯」在編譯期就寫不出來。
func PlatformRecipient(storeID int64) PublicationRecipient {
	return PublicationRecipient{Lane: QuoteLanePlatform, StoreID: storeID}
}

func ExternalRecipient(prospectID int64) PublicationRecipient {
	return PublicationRecipient{Lane: QuoteLaneExternal, ProspectID: prospectID}
}

// ErrRecipientIdentityMismatch 是 recipient 的 lane 與它帶的 id 欄位對不上時的
// 哨兵錯誤（例如 EXTERNAL 卻只填了 StoreID）。Hash() 遇到它一律失敗而不是算出一個
// 看起來合法的值——一個身分錯誤的 preimage 產生的 hash 會靜默通過 execute 的 CAS，
// 那正是這個型別要防的事。
var ErrRecipientIdentityMismatch = errors.New("procurement: publication recipient identity doesn't match its lane")

// identity 回傳這位收件人在 preimage 裡的身分欄位；lane 與 id 對不上時回錯誤。
func (r PublicationRecipient) identity() (storeID, prospectID string, err error) {
	switch r.Lane {
	case QuoteLanePlatform:
		if r.StoreID == 0 || r.ProspectID != 0 {
			return "", "", fmt.Errorf("%w: lane=PLATFORM storeId=%d prospectId=%d", ErrRecipientIdentityMismatch, r.StoreID, r.ProspectID)
		}
		return strconv.FormatInt(r.StoreID, 10), "", nil
	case QuoteLaneExternal:
		if r.ProspectID == 0 || r.StoreID != 0 {
			return "", "", fmt.Errorf("%w: lane=EXTERNAL storeId=%d prospectId=%d", ErrRecipientIdentityMismatch, r.StoreID, r.ProspectID)
		}
		return "", strconv.FormatInt(r.ProspectID, 10), nil
	default:
		return "", "", fmt.Errorf("%w: unknown lane %q", ErrRecipientIdentityMismatch, r.Lane)
	}
}

// PublicationAudience 是 publication 的受眾，兩支 pipeline 共用型別但 preimage 不同。
type PublicationAudience struct {
	Mode                      string
	PolicyID                  string
	PolicyVersion             int
	CandidateSetID            int64
	CandidateSetPolicyVersion int
	Recipients                []PublicationRecipient
}

// WishPoolAudience 回傳 v1 的公開池受眾：全站所有未停權的賣家。
// 沒有 recipient 名單是本模式的定義，不是資料缺漏。
func WishPoolAudience() PublicationAudience {
	return PublicationAudience{
		Mode:          codemeta.ProcurementPublicationAudienceModeWishPool.Value,
		PolicyID:      codemeta.ProcurementWishPoolPolicyAllActiveSellersV1.Value,
		PolicyVersion: WishPoolPolicyVersionV1,
	}
}

// DirectRecipientsAudience 回傳指定對象受眾。recipients 的順序由呼叫端決定並且有意義。
func DirectRecipientsAudience(candidateSetID int64, candidateSetPolicyVersion int, recipients []PublicationRecipient) PublicationAudience {
	return PublicationAudience{
		Mode:                      codemeta.ProcurementPublicationAudienceModeDirectRecipients.Value,
		CandidateSetID:            candidateSetID,
		CandidateSetPolicyVersion: candidateSetPolicyVersion,
		Recipients:                recipients,
	}
}

// audiencePreimage 是進 hash 的白名單投影。⛔ 不得加入任何會隨外部事件變動的狀態
// （賣家停權、庫存、額度）——那類狀態改變時該作廢的不是這一筆核准，它們屬於讀取端
// recheck。加欄位前先問「這個值改變時，買家的核准該不該失效」。
//
// 數字一律以字串進 preimage，與 proto 的過線慣例一致（CLAUDE.md §4），也避免
// JSON number 的精度／格式差異讓同一組受眾算出兩個 hash。
type audiencePreimage struct {
	Mode                      string                `json:"mode"`
	PolicyID                  string                `json:"policyId,omitempty"`
	PolicyVersion             int                   `json:"policyVersion,omitempty"`
	CandidateSetID            string                `json:"candidateSetId,omitempty"`
	CandidateSetPolicyVersion int                   `json:"candidateSetPolicyVersion,omitempty"`
	Recipients                []audienceRecipientPI `json:"recipients,omitempty"`
}

// ⛔ **欄位順序與 omitempty 是契約的一部分，不要「整理」它。** encoding/json 依
// struct 宣告順序輸出，所以：
//
//   - StoreID 排在 Lane 前面，且 PLATFORM 一定有 store id ⇒ PLATFORM 收件人序列化
//     出來是 `{"storeId":"11","lane":"PLATFORM"}`，與**加入 ProspectID 之前**逐位元組
//     相同。這讓「支援 EXTERNAL」對既有的 PLATFORM 受眾不是破壞性變更，既有
//     approval 的 audience_hash 依然成立。
//   - ProspectID 排在最後並帶 omitempty ⇒ PLATFORM 從不輸出這個鍵。
//   - StoreID 也帶 omitempty ⇒ EXTERNAL 從不輸出 storeId（尤其不會輸出 "0"）。
//
// 同樣的道理，日後若真的出現第三條 lane，把它的 id 欄位加在最後並帶 omitempty，
// 既有兩條 lane 的 preimage 位元組不動、hash 不動。
type audienceRecipientPI struct {
	StoreID    string `json:"storeId,omitempty"`
	Lane       string `json:"lane"`
	ProspectID string `json:"prospectId,omitempty"`
}

// Hash 產生 server-owned audienceHash。呼叫端傳入的 hash 一律不可信（#2450 Scope 2）。
func (a PublicationAudience) Hash() (string, error) {
	pre := audiencePreimage{
		Mode:                      a.Mode,
		PolicyID:                  a.PolicyID,
		PolicyVersion:             a.PolicyVersion,
		CandidateSetPolicyVersion: a.CandidateSetPolicyVersion,
	}
	if a.CandidateSetID != 0 {
		pre.CandidateSetID = strconv.FormatInt(a.CandidateSetID, 10)
	}
	for _, r := range a.Recipients {
		storeID, prospectID, idErr := r.identity()
		if idErr != nil {
			return "", idErr
		}
		pre.Recipients = append(pre.Recipients, audienceRecipientPI{
			StoreID: storeID, Lane: string(r.Lane), ProspectID: prospectID,
		})
	}
	b, err := json.Marshal(pre)
	if err != nil {
		return "", fmt.Errorf("marshal audience preimage: %w", err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
