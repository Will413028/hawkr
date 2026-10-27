// Package repository 是 procurement 域的持久層。刻意與既有 AI 表分離：
// 沒有跨域外鍵，未來抽成獨立服務時整包搬走。
//
// 資料存取走 sqlc（見 db/queries/procurement/、db/sqlc.yaml 的第二個 entry）。這裡的
// Request／Revision／CandidateSet／Candidate／CandidateSelection 是手寫的 plain
// struct，不是 sqlc 產出的 gen.XXX——monorepo 既有慣例（如 buyer-service 的
// toBuyerEntity）是 repository 對外只曝露自己定義的型別，轉換邏輯收在下面的
// toXxxEntity 函式，usecase／handler 層完全不知道 pgtype／gen 的存在。
//
// answer_patch 是例外：它沒有自己的 hand-written entity／toXxxEntity——revision.go
// 直接讀寫 gen.ProcurementAnswerPatch*，因為呼叫端（turn.go）只需要它在冪等重播時
// 讀出的 payload_json，不需要整列的其他欄位。這裡曾經有一個對稱的 AnswerPatch
// struct，兩輪 review 都確認它從未被建構過（沒有 toAnswerPatchEntity，也沒有任何
// 呼叫端持有它的值），已移除。
package repository

import (
	"encoding/json"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository/gen"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

// Request 是這個需求的身分與版本指標。
//
// 品項的**解析結果**不在這個型別上：它是 domain.Slots.ItemResolution，跟 item
// 原話住同一個 revision（ADR 2026-09-04-crop-resolution-into-slots，刀 1）。
// 「現在正在問什麼」（待消歧候選與待處理原因）也不在這裡：它是那個還開著的
// 中斷的 payload（刀 2）。兩者都曾經住在這張單列可變表上，而兩次搬家的理由是
// 同一個——放錯層的欄位讓所有依賴生命週期的機制（diff、rewind、版本鏈、
// interrupt 的建立與解決）都得為它寫特判。
type Request struct {
	ID               int64
	BuyerID          identity.BuyerID
	Status           string
	ActiveRevisionID int64
	Version          int
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func toRequestEntity(row gen.ProcurementRequest) Request {
	return Request{
		ID: row.ID, BuyerID: identity.BuyerID(row.BuyerID), Status: row.Status,
		ActiveRevisionID: int64Value(row.ActiveRevisionID), Version: int(row.Version),
		CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time,
	}
}

// SlotsJSON／PayloadJSON 用 encoding/json.RawMessage：sqlc.yaml 對這兩欄做了
// go_type override（見 Design Decisions #8），本檔案的欄位型別與既有 GORM 版本一致，
// 只是產生方式從 gorm tag 換成 sqlc override。
type Revision struct {
	ID               int64
	RequestID        int64
	RevisionNo       int
	ParentRevisionID int64
	BranchID         int64
	SlotsJSON        json.RawMessage
	SlotsHash        string
	Status           string
	ConversationID   int64
	CreatedAt        time.Time
}

func toRevisionEntity(row gen.ProcurementRequestRevision) Revision {
	return Revision{
		ID: row.ID, RequestID: row.RequestID, RevisionNo: int(row.RevisionNo),
		ParentRevisionID: int64Value(row.ParentRevisionID), BranchID: row.BranchID,
		SlotsJSON: row.SlotsJson, SlotsHash: row.SlotsHash, Status: row.Status,
		CreatedAt: row.CreatedAt.Time,
	}
}

// RevisionWithPatch 是版本鏈裡的一個節點，配對它對應的 answer_patch——root revision
// 沒有 patch（CreateRequest 不建立 answer_patch row），此時 PatchSource 為空字串、
// PatchPayloadJSON 為 nil。
type RevisionWithPatch struct {
	Revision
	PatchSource      string
	PatchPayloadJSON json.RawMessage
}

// CandidateSet 是一次候選搜尋 run 的容器與生命週期紀錄（見 candidate.go 檔頭註解）。
type CandidateSet struct {
	ID int64
	// RunID 是這次執行的身分，與 ID（這批候選的身分）**刻意不同值**——設計書 57 §1.2-3
	// 禁止一個 ID 同時代表 runId 與 candidateSetId。
	//
	// 零值只出現在 20260822061013 這支 migration 之前建立的舊列（DB 欄位為 NULL）。
	// 呼叫端要對外送出 runId 時一律走 PublicRunID()，不要自己判斷零值。
	RunID              int64
	RequestID          int64
	RequestRevisionID  int64
	BuyerID            identity.BuyerID
	PolicyVersion      int
	Status             string
	PlatformLaneStatus string
	ExternalLaneStatus string
	// FailureReason 是 run 進入 FAILED 的機器可讀原因（CodeMeta 的
	// ProcurementRunFailureReason），非 FAILED 時為空字串。
	FailureReason  string
	LeaseOwner     string
	LeaseExpiresAt *time.Time
	FencingToken   int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// PublicRunID 回傳對外契約上的 runId。
//
// expand 階段的相容行為：run_id 欄位是 2026-08-22 才加的，那之前建立的列沒有值，這種列
// 退回舊行為（runId ＝ candidate set id）。**這是刻意保留的過渡路徑，不是契約**——新列
// 一律有獨立的 run_id，golden fixtures 也只照新路徑產生，所以沒有任何消費端能從 fixture
// 觀察到「runId 等於 candidateSetId」並依賴它。contract 階段（run_id 改 NOT NULL）連同
// 這個 fallback 一起移除。
func (s CandidateSet) PublicRunID() int64 {
	if s.RunID != 0 {
		return s.RunID
	}
	return s.ID
}

// isSucceededRunStatus 判斷一個 run 狀態是否為「成功終態」。
//
// 這個判斷決定 GetCandidateSetWithCandidates 要不要去讀候選：判偽會讓那些 run 對外變成
// 「終態但零候選」，判真過寬會讓還在跑的 run 提前吐出不完整的候選清單——兩種都沒有錯誤訊號，
// 所以即使現在只比一個值，仍保留具名函式讓呼叫端讀得出意圖、也讓值域有單一修改點。
func isSucceededRunStatus(status string) bool {
	return status == codemeta.ProcurementRunStatusSucceeded.Value
}

func toCandidateSetEntity(row gen.ProcurementCandidateSet) CandidateSet {
	return CandidateSet{
		ID: row.ID, RunID: int64Value(row.RunID), RequestID: row.RequestID, RequestRevisionID: row.RequestRevisionID,
		BuyerID: identity.BuyerID(row.BuyerID), PolicyVersion: int(row.PolicyVersion),
		Status: row.Status, PlatformLaneStatus: row.PlatformLaneStatus, ExternalLaneStatus: row.ExternalLaneStatus,
		FailureReason: row.FailureReason,
		LeaseOwner:    row.LeaseOwner, LeaseExpiresAt: row.LeaseExpiresAt,
		FencingToken: row.FencingToken, CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time,
	}
}

// Candidate 是 candidate_set 底下的一個候選（PLATFORM 或 EXTERNAL lane）。
//
// StoreID 以下四欄是 #2377 的賣家身分快照（facts_as_of 當下的事實），讓網頁把候選呈現成
// 「找到哪幾家商家」。⛔ StoreID **不是授權判準**——擁有權必須在需要的當下重新問
// product-service，見 usecase/quote_confirm.go 的 ConfirmByID。EXTERNAL lane 四欄一律零值。
type Candidate struct {
	ID                    int64
	CandidateSetID        int64
	Lane                  string
	SourceID              string
	DisplayName           string
	Score                 int
	MatchedFactsJSON      json.RawMessage
	UnverifiableFactsJSON json.RawMessage
	FactsAsOf             time.Time
	Rank                  int
	StoreID               identity.StoreID // 0 = 無平台賣家（EXTERNAL lane）
	SellerName            string
	LogoKey               string // 商家 logo objectKey，不是 URL
	StoreCode             string // 賣家頁 canonical 短碼
	// ProspectID 是 EXTERNAL lane 的業者身分（seller-service.seller_prospects.id）。
	// 與 StoreID 互斥：PLATFORM 填 StoreID、EXTERNAL 填 ProspectID。
	// uq_procurement_candidates_set_prospect 掛在它上面。
	ProspectID *int64
	// BrandLogoKey／BrandCoverKey 是 EXTERNAL lane 的品牌素材快照。與上面的
	// LogoKey 不同軸：那一欄是平台商家的 logo（PLATFORM 專用），這兩欄是站外
	// 業者自己的品牌圖，不表示平台已驗證或已入駐。
	BrandLogoKey  string
	BrandCoverKey string
	CreatedAt     time.Time
}

func toCandidateEntity(row gen.ProcurementCandidate) Candidate {
	return Candidate{
		ID: row.ID, CandidateSetID: row.CandidateSetID, Lane: row.Lane, SourceID: row.SourceID,
		DisplayName: row.DisplayName, Score: int(row.Score),
		MatchedFactsJSON: row.MatchedFactsJson, UnverifiableFactsJSON: row.UnverifiableFactsJson,
		FactsAsOf: row.FactsAsOf.Time, Rank: int(row.Rank),
		StoreID:       identity.StoreID(int64Value(row.StoreID)), // nil → 0，正是 EXTERNAL 的語意
		SellerName:    row.SellerName,
		LogoKey:       row.LogoKey,
		StoreCode:     row.StoreCode,
		BrandLogoKey:  row.BrandLogoKey,
		BrandCoverKey: row.BrandCoverKey,
		ProspectID:    row.ProspectID,
		CreatedAt:     row.CreatedAt.Time,
	}
}

// CandidateSelection 是買家對某個候選的勾選狀態。
type CandidateSelection struct {
	ID                int64
	RequestID         int64
	RequestRevisionID int64
	BuyerID           identity.BuyerID
	CandidateSetID    int64
	Lane              string
	CandidateID       int64
	Selected          bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func toCandidateSelectionEntity(row gen.ProcurementCandidateSelection) CandidateSelection {
	return CandidateSelection{
		ID: row.ID, RequestID: row.RequestID, RequestRevisionID: row.RequestRevisionID,
		BuyerID: identity.BuyerID(row.BuyerID), CandidateSetID: row.CandidateSetID, Lane: row.Lane,
		CandidateID: row.CandidateID, Selected: row.Selected,
		CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time,
	}
}

// Conversation 是一次採購對話的容器（1:1 對應 procurement_requests，
// uq_procurement_conversations_request 保證）。
type Conversation struct {
	ID                 int64
	RequestID          int64
	BuyerID            identity.BuyerID
	Title              string
	TitleSource        string // AUTO | USER
	LastMessagePreview string
	DeletedAt          *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func toConversationEntity(row gen.ProcurementConversation) Conversation {
	return Conversation{
		ID: row.ID, RequestID: row.RequestID, BuyerID: identity.BuyerID(row.BuyerID),
		Title: row.Title, TitleSource: row.TitleSource, LastMessagePreview: row.LastMessagePreview,
		DeletedAt: row.DeletedAt,
		CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time,
	}
}
