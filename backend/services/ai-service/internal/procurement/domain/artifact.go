package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
)

// ArtifactType 是 Hawkr Agent typed artifact 的種類（doc 57 §4.5；CodeMeta
// ProcurementArtifactType，業務類型碼.md）。值域凍結於 2026-08-20「V6
// artifact/event registry delta」留言（issue #2446），依 doc 57 §14 D3 為工作版本，
// 未經二次確認；之後若有更新版本，以 additive 方式處理。
type ArtifactType string

// 綁到 CodeMeta 生成的 var（非 const——ProcurementArtifactType 是 struct literal），
// 手法同 steps.go 的 stepStatusSucceeded：編譯期綁死，避免第二份字面值。
var (
	ArtifactRequestSnapshot        = ArtifactType(codemeta.ProcurementArtifactTypeRequestSnapshot.Value)
	ArtifactClarification          = ArtifactType(codemeta.ProcurementArtifactTypeClarification.Value)
	ArtifactCandidateResults       = ArtifactType(codemeta.ProcurementArtifactTypeCandidateResults.Value)
	ArtifactPricingPreview         = ArtifactType(codemeta.ProcurementArtifactTypePricingPreview.Value)
	ArtifactConfirmedRFQ           = ArtifactType(codemeta.ProcurementArtifactTypeConfirmedRfq.Value)
	ArtifactRFQPublicationApproval = ArtifactType(codemeta.ProcurementArtifactTypeRfqPublicationApproval.Value)
	ArtifactRFQPublicationResult   = ArtifactType(codemeta.ProcurementArtifactTypeRfqPublicationResult.Value)
	ArtifactSellerQuoteResult      = ArtifactType(codemeta.ProcurementArtifactTypeSellerQuoteResult.Value)
	ArtifactErrorRecovery          = ArtifactType(codemeta.ProcurementArtifactTypeErrorRecovery.Value)
	ArtifactPurchasePlan           = ArtifactType(codemeta.ProcurementArtifactTypePurchasePlan.Value)
	ArtifactCartApproval           = ArtifactType(codemeta.ProcurementArtifactTypeCartApproval.Value)
	ArtifactCartResult             = ArtifactType(codemeta.ProcurementArtifactTypeCartResult.Value)
)

// artifactTrackA／artifactTrackB 見 doc 57 §4.5：「兩個 Track 不得共用一個 generic
// APPROVAL data schema」——ArtifactRFQPublicationApproval 與 ArtifactCartApproval
// 概念上都是「核准」，但屬不同 Track，資料形狀必須各自獨立定義，不得抽出共用型別。
var artifactTrackA = map[ArtifactType]bool{
	ArtifactRequestSnapshot:        true,
	ArtifactClarification:          true,
	ArtifactCandidateResults:       true,
	ArtifactPricingPreview:         true,
	ArtifactConfirmedRFQ:           true,
	ArtifactRFQPublicationApproval: true,
	ArtifactRFQPublicationResult:   true,
	ArtifactSellerQuoteResult:      true,
	ArtifactErrorRecovery:          true,
}

var artifactTrackB = map[ArtifactType]bool{
	ArtifactPurchasePlan: true,
	ArtifactCartApproval: true,
	ArtifactCartResult:   true,
}

// IsTrackA／IsTrackB 供 schema 驗證與未來 projector 使用，避免 Track A／B 誤用同一
// helper（doc 57 §4.5）。
func (t ArtifactType) IsTrackA() bool { return artifactTrackA[t] }
func (t ArtifactType) IsTrackB() bool { return artifactTrackB[t] }

// artifactStatusReady／artifactStatusFailed 綁到 CodeMeta 生成常數（同 steps.go 的
// stepStatusSucceeded 手法：編譯期綁死，避免第二份字面值）。
var (
	artifactStatusDraft = codemeta.ProcurementArtifactStatusDraft.Value
)

// ArtifactDraftStatus 是新建立、尚未完整產製的 artifact 應使用的初始狀態
// （ProcurementArtifactStatus.DRAFT，doc 57 §4.3）。
func ArtifactDraftStatus() string { return artifactStatusDraft }

// Artifact 是 doc 57 §4.5 的公開 typed artifact 共同 envelope（`PublicAgentArtifactV1`
// 的落地，見 doc 57 §4.1）。DB 落地（`procurement_agent_run_artifacts`）尚未建立
// （doc 57 §6.2，Slice F），本結構是 Slice B 契約凍結的產物，供 golden fixtures 與
// 後續 BE-01B serializer／runtime-snapshot DTO（doc 57 §5.6）共用。
//
// Version 是這個 artifact_id 的「第幾次產製」（instance revision，對齊
// `UNIQUE (artifact_id, version)`），不是 data schema 版本——data 的形狀演進採
// additive-only（同本檔案其餘值域的 open registry 原則），不另外在 payload 內放
// schema version 欄位。
type Artifact struct {
	ArtifactID int64        `json:"artifactId,string"`
	Type       ArtifactType `json:"type"`
	Version    int          `json:"version"`
	Checksum   string       `json:"checksum"`
	// Status 是 ProcurementArtifactStatus 值域（DRAFT/READY/STALE/CONSUMED/FAILED）。
	Status string `json:"status"`
	// AsOf 是本 artifact 據以產製的事實時間戳（freshness；doc 57 §4.5），不是 DB
	// audit time——依 doc 57 §21，事件/業務時間與 DB audit time 不可互相冒充。
	AsOf time.Time `json:"asOf"`
	// DomainRef 指向本 artifact 據以產製的來源 domain 物件（例如
	// candidateSetId／requestRevisionId），供 owner restore 與 §6.4-6 的 CAS
	// （`expectedRfqSnapshotHash`）使用。Allowlisted：只能是已核准會對外曝光的
	// domain 識別值，不得塞入未經核准的內部參照（doc 57 §4.5、2026-08-20 delta）。
	DomainRef string `json:"domainRef,omitempty"`
	// Data 依 Type 而定形狀。四個已可從既有 domain 結構推導的型別見本檔下方
	// RequestSnapshotData／ClarificationData／CandidateResultsData／
	// ErrorRecoveryData；其餘九型（Track A 六型＋Track B 三型）data 形狀尚未展開
	// （doc 57 §5.7、業務類型碼.md ProcurementArtifactType note），刻意不在此
	// 預先發明未經業務設計驗證的欄位——projector 對 unknown/未展開 schema 必須
	// fail-closed（2026-08-20 delta），不得臆測渲染。
	Data json.RawMessage `json:"data"`
}

// RequestSnapshotData 是 ArtifactRequestSnapshot 的 data payload。對應
// procurement_request_revisions 的公開投影。
type RequestSnapshotData struct {
	RevisionID int64  `json:"revisionId,string"`
	RevisionNo int    `json:"revisionNo"`
	Slots      Slots  `json:"slots"`
	SlotsHash  string `json:"slotsHash"`
}

// ClarificationData 是 ArtifactClarification 的 data payload：承載
// domain.ClarifyQuestion 的公開投影（doc 57 §4.1）。
type ClarificationData struct {
	Question ClarifyQuestion `json:"question"`
}

// CandidateResultsData 是 ArtifactCandidateResults 的 data payload：PLATFORM／
// EXTERNAL 兩條 lane 各自的候選清單。ProcessedCount／TotalCount 只有 authoritative
// 值才回，unknown total 為 nil（doc 57 §6.3，不得顯示假 percent）。
type CandidateResultsData struct {
	PlatformLane CandidateLaneResult `json:"platformLane"`
	ExternalLane CandidateLaneResult `json:"externalLane"`
}

// CandidateLaneResult 是單一 lane 的候選搜尋結果切片。
type CandidateLaneResult struct {
	Status         string             `json:"status"` // ProcurementLaneStatus 值域
	ProcessedCount *int               `json:"processedCount"`
	TotalCount     *int               `json:"totalCount"`
	Candidates     []CandidateSummary `json:"candidates"`
}

// CandidateSummary 是候選的公開安全子集（doc 57 §4.1 PublicCandidateV1 的落地）。
// 欄位形狀對齊 gateway `ProcurementCandidateItemDTO`（gateway/internal/handler/
// procurement_response_mapping.go）——同一個候選概念在兩個 Go module 各自維護一份
// DTO 鏡像是既有慣例（該檔案自己的註解即以此手法對齊 proto）。
//
// 明確禁止（doc 57 §1.3、§4.1）：candidate 的 email、phone、social、聯絡人與 BD
// notes 一律不得出現在此結構——唯一屏障是這層欄位挑選，任何新增欄位前必須先對照
// §1.3 的 data-class 矩陣。PublicWebsite 目前仍可能是社群網址（§1.3 補充第三條，
// 已知缺口，另案處理，不在本次範圍關閉）。
type CandidateSummary struct {
	CandidateID       int64              `json:"candidateId,string"`
	SourceID          string             `json:"sourceId"`
	Lane              Lane               `json:"lane"`
	DisplayName       string             `json:"displayName"`
	Score             float64            `json:"score"`
	MatchedFacts      []MatchFact        `json:"matchedFacts"`
	UnverifiableFacts []UnverifiableFact `json:"unverifiableFacts"`
	FactsAsOf         time.Time          `json:"factsAsOf"`
	Selected          bool               `json:"selected"`

	// 以下是 issue #2377 的賣家身分快照，只在 PLATFORM lane 非空（EXTERNAL lane 沒有
	// 平台賣家，四欄一律省略，不偽造平台 seller identity）。命名與 gateway DTO 一致，
	// 三層同名（DB／proto／此處）避免重開 account_id 命名債。
	StoreID    string `json:"storeId,omitempty"`
	SellerName string `json:"sellerName,omitempty"`
	StoreCode  string `json:"storeCode,omitempty"`
	LogoKey    string `json:"logoKey,omitempty"`
}

// ErrorRecoveryData 是 ArtifactErrorRecovery 的 data payload。欄位形狀沿用既有
// procurement 錯誤 envelope（gateway `procurementErrorBody`），依 doc 57 §4.7
// 「錯誤 envelope 形狀沿用既有實作，不重新設計」。
type ErrorRecoveryData struct {
	// Reason 是 ProcurementErrorReason 或 ProcurementRunFailureReason 值域之一，
	// 依觸發來源而定（run 層失敗用後者，command 層錯誤用前者）。
	Reason  string `json:"reason"`
	Message string `json:"message"`
	// Retryable 由伺服器依 Reason 判斷是否值得重試同一命令，不得由前端猜測。
	Retryable bool `json:"retryable"`
}

// ArtifactChecksum 是 Artifact.Checksum 的唯一算法：對 data payload 的位元組做
// sha256、輸出 hex。與 Slots.Hash 同一套（sha256 hex）——artifact 與 request
// snapshot 之後要互相對帳「是不是同一份事實」，兩處各用一套雜湊只會多一層換算。
//
// 輸入是**已序列化的位元組**不是 Go 值：呼叫端寫進 DB 的就是那串位元組，對它算
// checksum 才保證「checksum 描述的是真正被存下來的內容」。若改成先接 any 再在這裡
// json.Marshal，marshal 兩次的結果不保證逐位元組相同（map 順序、之後可能加的
// encoder 設定），checksum 就會描述一份不存在的內容。
func ArtifactChecksum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
