package grpc

import (
	"context"
	"time"

	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"google.golang.org/grpc"
)

// AIProcurementClient 是 ai-service AIProcurementService 的 gRPC client——比照
// AIAssistantClient（Phase B）的既有 pattern：包裝共用連線，每個 unary 方法帶
// 自己的 deadline，streaming 方法不設總長度上限（由呼叫端的 idle/write timeout
// 控制，見 gateway/internal/proxy/ai_assistant_stream.go）。
type AIProcurementClient struct {
	conn   *grpc.ClientConn
	client aipb.AIProcurementServiceClient
}

// NewAIProcurementClient wraps the shared ai-service connection (see
// ai_service_conn.go) with the AIProcurementService stub — the same
// connection NewAIClient/NewAIAssistantClient use, not a second dial.
func NewAIProcurementClient(shared *aiServiceConn) *AIProcurementClient {
	return &AIProcurementClient{conn: shared.conn, client: aipb.NewAIProcurementServiceClient(shared.conn)}
}

// turnTimeout／readTimeout 的兩層分法對照 ai-service 端實際行為（已逐一讀過
// services/ai-service/internal/procurement/usecase/*.go 驗證，不是照抄 brief）：
//
//   - SubmitTurn（turn.go TurnUseCase.Handle）：同步呼叫 extractor.Extract
//     （LLM 抽取）與 replyComposer.Compose（ADK llmagent 回覆生成，見
//     reply_composer.go）。
//   - SubmitAnswer（answer.go AnswerUseCase.Handle）、Rewind（rewind.go
//     RewindUseCase.Handle）：不呼叫抽取，但仍同步呼叫 replyComposer.Compose——
//     跟 SubmitTurn 用同一個 LLM 回覆生成路徑，因此歸進同一個 30 秒預算，
//     而不是 brief 草稿原本暗示的「只有 SubmitTurn 有 LLM」。
//   - 其餘 RPC（GetSnapshot、候選搜尋觸發／輪詢、勾選、CropDisambiguation、
//     報價批次建立／預覽／撤回、對話管理、報價邀請確認）：讀過對應 usecase，
//     皆為純 DB 讀寫或 CPU-bound 計價，沒有同步 LLM 呼叫，用與 AIAssistantClient
//     的 latestSessionTimeout 相同的 15 秒讀取型預算。StartCandidateSearch 只
//     觸發背景 goroutine（dispatch/run），真正的搜尋不在這個呼叫的關鍵路徑上。
const (
	turnTimeout = 30 * time.Second // 同步呼叫 LLM 抽取或 ADK 回覆生成
	readTimeout = 15 * time.Second // 純讀寫或 CPU-bound 計價，無同步 LLM 呼叫
)

func (c *AIProcurementClient) SubmitTurn(ctx context.Context, req *aipb.TurnRequest) (*aipb.TurnResult, error) {
	ctx, cancel := context.WithTimeout(ctx, turnTimeout)
	defer cancel()
	return c.client.SubmitTurn(ctx, req)
}

// AssistantStream 介面（Recv() (*aipb.StreamEvent, error) + grpc.ClientStream）在
// gateway/internal/grpc/ai_assistant_client.go 已定義——SubmitTurnStream 的回傳型別
// 直接滿足它，不用重新定義一個 procurement 專屬 interface。
func (c *AIProcurementClient) SubmitTurnStream(ctx context.Context, req *aipb.TurnRequest) (AssistantStream, error) {
	return c.client.SubmitTurnStream(ctx, req)
}

func (c *AIProcurementClient) GetSnapshot(ctx context.Context, req *aipb.GetSnapshotRequest) (*aipb.SnapshotResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	return c.client.GetSnapshot(ctx, req)
}

func (c *AIProcurementClient) StartCandidateSearch(ctx context.Context, req *aipb.StartCandidateSearchRequest) (*aipb.StartCandidateSearchResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	return c.client.StartCandidateSearch(ctx, req)
}

func (c *AIProcurementClient) GetCandidateSearch(ctx context.Context, req *aipb.GetCandidateSearchRequest) (*aipb.CandidateSetResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	return c.client.GetCandidateSearch(ctx, req)
}

func (c *AIProcurementClient) ToggleSelection(ctx context.Context, req *aipb.ToggleSelectionRequest) (*aipb.ToggleSelectionResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	return c.client.ToggleSelection(ctx, req)
}

func (c *AIProcurementClient) SubmitAnswer(ctx context.Context, req *aipb.SubmitAnswerRequest) (*aipb.TurnResult, error) {
	ctx, cancel := context.WithTimeout(ctx, turnTimeout)
	defer cancel()
	return c.client.SubmitAnswer(ctx, req)
}

func (c *AIProcurementClient) Rewind(ctx context.Context, req *aipb.RewindRequest) (*aipb.TurnResult, error) {
	ctx, cancel := context.WithTimeout(ctx, turnTimeout)
	defer cancel()
	return c.client.Rewind(ctx, req)
}

// PreviewRfqPrice：委派給既有 PreviewRfqPriceUseCase，讀 Pricebook 快照＋DB
// 冪等鍵查詢，無同步 LLM 呼叫，因此歸進 readTimeout。
func (c *AIProcurementClient) PreviewRfqPrice(ctx context.Context, req *aipb.RfqPricePreviewRequest) (*aipb.RfqPricePreviewResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	return c.client.PreviewRfqPrice(ctx, req)
}

// ConfirmRfqSnapshot：委派給既有 ConfirmRfqSnapshotUseCase，讀 DB 冪等鍵＋
// price preview 查詢並寫入一筆確認快照，無同步 LLM 呼叫——同 PreviewRfqPrice
// 歸進 readTimeout。
func (c *AIProcurementClient) ConfirmRfqSnapshot(ctx context.Context, req *aipb.RfqConfirmationRequest) (*aipb.RfqConfirmationResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	return c.client.ConfirmRfqSnapshot(ctx, req)
}

// PreviewRfqPublication：委派給 PublicationPreviewUseCase，讀 DB（revision／
// confirmed snapshot／price preview／candidate set）並寫入一筆 approval，
// 無同步 LLM 呼叫也無對外通知——同 ConfirmRfqSnapshot 歸進 readTimeout。
func (c *AIProcurementClient) PreviewRfqPublication(ctx context.Context, req *aipb.RfqPublicationPreviewRequest) (*aipb.RfqPublicationPreviewResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	return c.client.PreviewRfqPublication(ctx, req)
}

// ExecuteRfqPublication：對 preview 留下的 approval 做 CAS 比對後寫入一筆
// publication，無同步 LLM 呼叫，本 slice 也無對外投遞（無 invite／outbox／
// projection fan-out）——同 PreviewRfqPublication 歸進 readTimeout。
func (c *AIProcurementClient) ExecuteRfqPublication(ctx context.Context, req *aipb.RfqPublicationExecuteRequest) (*aipb.RfqPublicationExecuteResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	return c.client.ExecuteRfqPublication(ctx, req)
}

func (c *AIProcurementClient) ListConversations(ctx context.Context, req *aipb.ListConversationsRequest) (*aipb.ListConversationsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	return c.client.ListConversations(ctx, req)
}

func (c *AIProcurementClient) GetConversationMessages(ctx context.Context, req *aipb.GetConversationMessagesRequest) (*aipb.ConversationMessagesResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	return c.client.GetConversationMessages(ctx, req)
}

func (c *AIProcurementClient) GetRuntimeSnapshot(ctx context.Context, req *aipb.GetRuntimeSnapshotRequest) (*aipb.RuntimeSnapshotResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	return c.client.GetRuntimeSnapshot(ctx, req)
}

// #2538 八支命令 ＋ 兩支賣家讀取：全部是 DB 讀寫，無同步 LLM 呼叫，歸 readTimeout。
func (c *AIProcurementClient) ListSellerPublications(ctx context.Context, req *aipb.ListSellerPublicationsRequest) (*aipb.ListSellerPublicationsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	return c.client.ListSellerPublications(ctx, req)
}

func (c *AIProcurementClient) GetSellerPublication(ctx context.Context, req *aipb.GetSellerPublicationRequest) (*aipb.SellerPublicationDetailResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	return c.client.GetSellerPublication(ctx, req)
}

func (c *AIProcurementClient) UpsertSellerResponseDraft(ctx context.Context, req *aipb.UpsertSellerResponseDraftRequest) (*aipb.SellerResponseDraftResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	return c.client.UpsertSellerResponseDraft(ctx, req)
}

func (c *AIProcurementClient) GetVendorPortalResponseDraft(ctx context.Context, req *aipb.GetVendorPortalResponseDraftRequest) (*aipb.SellerResponseDraftResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	return c.client.GetVendorPortalResponseDraft(ctx, req)
}

func (c *AIProcurementClient) UpsertVendorPortalResponseDraft(ctx context.Context, req *aipb.UpsertVendorPortalResponseDraftRequest) (*aipb.SellerResponseDraftResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	return c.client.UpsertVendorPortalResponseDraft(ctx, req)
}

// 站外業者（vendor portal）的三支正式寫入。與賣家版共用同一個 usecase，差別只有
// 身分鍵：這裡帶 rfq_recipient_id，publication 由 ai-service 從 invite 解出。
func (c *AIProcurementClient) SubmitVendorPortalResponse(ctx context.Context, req *aipb.SubmitVendorPortalResponseRequest) (*aipb.SellerResponseVersionResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	return c.client.SubmitVendorPortalResponse(ctx, req)
}

func (c *AIProcurementClient) WithdrawVendorPortalResponse(ctx context.Context, req *aipb.WithdrawVendorPortalResponseRequest) (*aipb.SellerResponseClosureResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	return c.client.WithdrawVendorPortalResponse(ctx, req)
}

func (c *AIProcurementClient) DeclineVendorPortalRfqPublication(ctx context.Context, req *aipb.DeclineVendorPortalRfqPublicationRequest) (*aipb.SellerResponseClosureResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	return c.client.DeclineVendorPortalRfqPublication(ctx, req)
}

func (c *AIProcurementClient) SubmitSellerResponse(ctx context.Context, req *aipb.SubmitSellerResponseRequest) (*aipb.SellerResponseVersionResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	return c.client.SubmitSellerResponse(ctx, req)
}

func (c *AIProcurementClient) WithdrawSellerResponse(ctx context.Context, req *aipb.WithdrawSellerResponseRequest) (*aipb.SellerResponseClosureResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	return c.client.WithdrawSellerResponse(ctx, req)
}

func (c *AIProcurementClient) DeclineRfqPublication(ctx context.Context, req *aipb.DeclineRfqPublicationRequest) (*aipb.SellerResponseClosureResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	return c.client.DeclineRfqPublication(ctx, req)
}

func (c *AIProcurementClient) ListSellerResponses(ctx context.Context, req *aipb.ListSellerResponsesRequest) (*aipb.ListSellerResponsesResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	return c.client.ListSellerResponses(ctx, req)
}

func (c *AIProcurementClient) PutShortlistEntry(ctx context.Context, req *aipb.ShortlistEntryRequest) (*aipb.ShortlistEntryResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	return c.client.PutShortlistEntry(ctx, req)
}

func (c *AIProcurementClient) DeleteShortlistEntry(ctx context.Context, req *aipb.ShortlistEntryRequest) (*aipb.ShortlistEntryResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	return c.client.DeleteShortlistEntry(ctx, req)
}

func (c *AIProcurementClient) AcceptSellerQuote(ctx context.Context, req *aipb.BuyerQuoteAcceptanceRequest) (*aipb.BuyerQuoteAcceptanceResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	return c.client.AcceptSellerQuote(ctx, req)
}

func (c *AIProcurementClient) RenameConversation(ctx context.Context, req *aipb.RenameConversationRequest) error {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	_, err := c.client.RenameConversation(ctx, req)
	return err
}

func (c *AIProcurementClient) DeleteConversation(ctx context.Context, req *aipb.DeleteConversationRequest) error {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	_, err := c.client.DeleteConversation(ctx, req)
	return err
}

// GetRfqPublicationInviteSellerView 供 Vendor Portal E1 讀取站外業者可見的 RFQ 摘要。
func (c *AIProcurementClient) GetRfqPublicationInviteSellerView(ctx context.Context, req *aipb.GetRfqPublicationInviteSellerViewRequest) (*aipb.RfqPublicationInviteSellerViewResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	return c.client.GetRfqPublicationInviteSellerView(ctx, req)
}

// WatchRunEvents opens Task 11's run-ledger replay/live-subscribe stream. Same
// no-timeout-here reasoning as SubmitTurnStream above: this stream's lifetime
// is owned by the caller (gateway/internal/proxy/ai_run_events_stream.go),
// which derives its own bounded context — a run can legitimately outlive one
// turn (SubmitAnswer／Rewind also produce TURN runs against the same run_id),
// so this method must not impose its own shorter budget on top of the
// caller's.
//
// Return type is the generated client-stream type, not
// proxy.RunEventStream — package proxy already imports this package (for
// AssistantStream), so declaring the return type here as proxy.RunEventStream
// would be a circular import. proxy.RunEventStream's method set is a subset of
// aipb.AIProcurementService_WatchRunEventsClient's, so the conversion happens
// implicitly at the handler's call site (gateway/internal/handler/ai.go),
// which already imports both packages.
func (c *AIProcurementClient) WatchRunEvents(ctx context.Context, req *aipb.WatchRunEventsRequest) (aipb.AIProcurementService_WatchRunEventsClient, error) {
	return c.client.WatchRunEvents(ctx, req)
}

// CancelRun (Task 14, doc57 §5.4): CAS on the run's version plus, when it
// succeeds, one status update + one ledger event append — no synchronous LLM
// call, same readTimeout bucket as ToggleSelection/StartCandidateSearch.
func (c *AIProcurementClient) CancelRun(ctx context.Context, req *aipb.CancelRunRequest) (*aipb.CancelRunResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	return c.client.CancelRun(ctx, req)
}

// ResumeRun (Task 14, doc57 §5.5): resolves an interrupt and creates one
// child run row (QUEUED) — pure DB writes, no synchronous LLM call in this
// RPC itself, same readTimeout bucket as CancelRun above.
func (c *AIProcurementClient) ResumeRun(ctx context.Context, req *aipb.ResumeRunRequest) (*aipb.ResumeRunResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	return c.client.ResumeRun(ctx, req)
}
