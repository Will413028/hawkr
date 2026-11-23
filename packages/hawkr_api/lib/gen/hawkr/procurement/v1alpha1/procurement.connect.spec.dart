//
//  Generated code. Do not modify.
//  source: hawkr/procurement/v1alpha1/procurement.proto
//

import "package:connectrpc/connect.dart" as connect;
import "procurement.pb.dart" as hawkrprocurementv1alpha1procurement;

/// BuyerProcurementAppService 是登入的買家的採購方法。身分取自 session，帳號沒有買家角色時回 permission_denied。
/// 不屬於呼叫者的需求、對話與 run 一律回 not_found。
abstract final class BuyerProcurementAppService {
  /// Fully-qualified name of the BuyerProcurementAppService service.
  static const name = 'hawkr.procurement.v1alpha1.BuyerProcurementAppService';

  /// SubmitTurn 送出買家的一則訊息，開始一輪釐清。request_id 空字串表示新建需求。伺服器受理後立刻回 run ID，
  /// 這一輪的過程與結果（turn_result 事件）從 WatchRun 取得。這個需求已有進行中的 run 回 FAILED_PRECONDITION，
  /// reason 是 PROCUREMENT_RUN_IN_PROGRESS。買家同時進行的 run 已達上限或每分鐘送出已達上限回 resource_exhausted（沒有 reason，帶 RetryInfo，稍後可再試）；
  /// 本期額度用完回 resource_exhausted，reason 是 AGENT_MONTHLY_QUOTA_EXCEEDED。
  static const submitTurn = connect.Spec(
    '/$name/SubmitTurn',
    connect.StreamType.unary,
    hawkrprocurementv1alpha1procurement.SubmitTurnRequest.new,
    hawkrprocurementv1alpha1procurement.SubmitTurnResponse.new,
  );

  /// GetSnapshot 回傳一張需求目前的內容與狀態。
  static const getSnapshot = connect.Spec(
    '/$name/GetSnapshot',
    connect.StreamType.unary,
    hawkrprocurementv1alpha1procurement.GetSnapshotRequest.new,
    hawkrprocurementv1alpha1procurement.GetSnapshotResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// StartCandidateSearch 開始為需求搜尋候選賣家（背景執行，以 GetCandidateSearch 取得結果）。
  /// 需求還沒齊備回 FAILED_PRECONDITION，reason 是 REQUIREMENT_NOT_READY；這個需求已有進行中的 run 回 FAILED_PRECONDITION，
  /// reason 是 PROCUREMENT_RUN_IN_PROGRESS。買家同時進行的 run 已達上限或每分鐘送出已達上限回 resource_exhausted（沒有 reason，帶 RetryInfo，稍後可再試）。
  static const startCandidateSearch = connect.Spec(
    '/$name/StartCandidateSearch',
    connect.StreamType.unary,
    hawkrprocurementv1alpha1procurement.StartCandidateSearchRequest.new,
    hawkrprocurementv1alpha1procurement.StartCandidateSearchResponse.new,
  );

  /// GetCandidateSearch 回傳一次候選搜尋的狀態與目前的候選。
  static const getCandidateSearch = connect.Spec(
    '/$name/GetCandidateSearch',
    connect.StreamType.unary,
    hawkrprocurementv1alpha1procurement.GetCandidateSearchRequest.new,
    hawkrprocurementv1alpha1procurement.GetCandidateSearchResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// ToggleSelection 勾選或取消勾選一個候選。候選所屬的候選集不是有效的候選集（需求已不在 MATCHING，或有更新的搜尋成功）回 ABORTED，conflictTarget 是 candidate_set。
  static const toggleSelection = connect.Spec(
    '/$name/ToggleSelection',
    connect.StreamType.unary,
    hawkrprocurementv1alpha1procurement.ToggleSelectionRequest.new,
    hawkrprocurementv1alpha1procurement.ToggleSelectionResponse.new,
  );

  /// SubmitAnswer 回答一個釐清問題。需求有待回覆的中斷時改用 ResumeRun，回 FAILED_PRECONDITION，
  /// reason 是 APPROVAL_PENDING。
  static const submitAnswer = connect.Spec(
    '/$name/SubmitAnswer',
    connect.StreamType.unary,
    hawkrprocurementv1alpha1procurement.SubmitAnswerRequest.new,
    hawkrprocurementv1alpha1procurement.SubmitAnswerResponse.new,
  );

  /// Rewind 讓需求退回某個欄位重新回答。有待回覆的中斷時規則同 SubmitAnswer。
  static const rewind = connect.Spec(
    '/$name/Rewind',
    connect.StreamType.unary,
    hawkrprocurementv1alpha1procurement.RewindRequest.new,
    hawkrprocurementv1alpha1procurement.RewindResponse.new,
  );

  /// PreviewRfqPrice 依買家預期的單價範圍與需求數量，算出詢價總額區間並存成預覽（有效 15 分鐘）。
  /// 買家沒有給合法的範圍（下界與上界是正整數、下界不大於上界）回 INVALID_ARGUMENT；
  /// 需求已有數量與單位卻算不出總額回 FAILED_PRECONDITION。
  static const previewRfqPrice = connect.Spec(
    '/$name/PreviewRfqPrice',
    connect.StreamType.unary,
    hawkrprocurementv1alpha1procurement.PreviewRfqPriceRequest.new,
    hawkrprocurementv1alpha1procurement.PreviewRfqPriceResponse.new,
  );

  /// ConfirmRfqSnapshot 對一份價格預覽產生不可變的詢價確認快照。預覽或需求版本過時回 ABORTED。
  static const confirmRfqSnapshot = connect.Spec(
    '/$name/ConfirmRfqSnapshot',
    connect.StreamType.unary,
    hawkrprocurementv1alpha1procurement.ConfirmRfqSnapshotRequest.new,
    hawkrprocurementv1alpha1procurement.ConfirmRfqSnapshotResponse.new,
  );

  /// PreviewRfqPublication 依確認快照編出要發給賣家的內容並建立不可變的發布核准，不發布、不通知。
  static const previewRfqPublication = connect.Spec(
    '/$name/PreviewRfqPublication',
    connect.StreamType.unary,
    hawkrprocurementv1alpha1procurement.PreviewRfqPublicationRequest.new,
    hawkrprocurementv1alpha1procurement.PreviewRfqPublicationResponse.new,
  );

  /// ExecuteRfqPublication 以發布核准的版本執行發布。成功只代表伺服器已建立發布，不代表已投遞或賣家已看到。
  static const executeRfqPublication = connect.Spec(
    '/$name/ExecuteRfqPublication',
    connect.StreamType.unary,
    hawkrprocurementv1alpha1procurement.ExecuteRfqPublicationRequest.new,
    hawkrprocurementv1alpha1procurement.ExecuteRfqPublicationResponse.new,
  );

  /// ListConversations 列出買家的對話，最近更新的在前。
  static const listConversations = connect.Spec(
    '/$name/ListConversations',
    connect.StreamType.unary,
    hawkrprocurementv1alpha1procurement.ListConversationsRequest.new,
    hawkrprocurementv1alpha1procurement.ListConversationsResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  static const getConversationMessages = connect.Spec(
    '/$name/GetConversationMessages',
    connect.StreamType.unary,
    hawkrprocurementv1alpha1procurement.GetConversationMessagesRequest.new,
    hawkrprocurementv1alpha1procurement.GetConversationMessagesResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// GetRuntimeSnapshot 回傳重建一個對話畫面需要的全部事實：訊息、run、待回覆的中斷與發布結果。
  static const getRuntimeSnapshot = connect.Spec(
    '/$name/GetRuntimeSnapshot',
    connect.StreamType.unary,
    hawkrprocurementv1alpha1procurement.GetRuntimeSnapshotRequest.new,
    hawkrprocurementv1alpha1procurement.GetRuntimeSnapshotResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// ListSellerResponses 列出一張需求已發布的詢價收到的賣家回價（每位賣家的目前版本）。
  static const listSellerResponses = connect.Spec(
    '/$name/ListSellerResponses',
    connect.StreamType.unary,
    hawkrprocurementv1alpha1procurement.ListSellerResponsesRequest.new,
    hawkrprocurementv1alpha1procurement.ListSellerResponsesResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// PutShortlistEntry 把賣家回價的一個版本加入入圍清單；已在清單時回目前的狀態。
  static const putShortlistEntry = connect.Spec(
    '/$name/PutShortlistEntry',
    connect.StreamType.unary,
    hawkrprocurementv1alpha1procurement.PutShortlistEntryRequest.new,
    hawkrprocurementv1alpha1procurement.PutShortlistEntryResponse.new,
  );

  /// DeleteShortlistEntry 把賣家回價從入圍清單移除；不在清單時回目前的狀態。
  static const deleteShortlistEntry = connect.Spec(
    '/$name/DeleteShortlistEntry',
    connect.StreamType.unary,
    hawkrprocurementv1alpha1procurement.DeleteShortlistEntryRequest.new,
    hawkrprocurementv1alpha1procurement.DeleteShortlistEntryResponse.new,
  );

  /// AcceptSellerQuote 接受賣家回價的指定版本並凍結成交價，隨後建立待付款訂單。報價已改版、已撤回回 ABORTED，
  /// conflictTarget 是 seller_response；報價已過有效期回 FAILED_PRECONDITION，reason 是 RFQ_RESPONSE_EXPIRED；
  /// 同一張需求已有進行中的成交回 FAILED_PRECONDITION，reason 是 RFQ_CHECKOUT_ALREADY_ACTIVE（metadata 的 orderNo 是
  /// 既有訂單，買家去付款，不是重新接受）；已經成交回 FAILED_PRECONDITION，reason 是 RFQ_ALREADY_AWARDED。
  static const acceptSellerQuote = connect.Spec(
    '/$name/AcceptSellerQuote',
    connect.StreamType.unary,
    hawkrprocurementv1alpha1procurement.AcceptSellerQuoteRequest.new,
    hawkrprocurementv1alpha1procurement.AcceptSellerQuoteResponse.new,
  );

  static const renameConversation = connect.Spec(
    '/$name/RenameConversation',
    connect.StreamType.unary,
    hawkrprocurementv1alpha1procurement.RenameConversationRequest.new,
    hawkrprocurementv1alpha1procurement.RenameConversationResponse.new,
  );

  static const deleteConversation = connect.Spec(
    '/$name/DeleteConversation',
    connect.StreamType.unary,
    hawkrprocurementv1alpha1procurement.DeleteConversationRequest.new,
    hawkrprocurementv1alpha1procurement.DeleteConversationResponse.new,
  );

  /// WatchRun 訂閱一個 run 的事件，從 cursor 之後開始（空字串表示從頭）。只讀；伺服器在 run 結束（finished 或 error 事件之後）
  /// 或滿 100 秒時結束串流。每位買家同時訂閱的串流有上限，超過回 resource_exhausted。
  static const watchRun = connect.Spec(
    '/$name/WatchRun',
    connect.StreamType.server,
    hawkrprocurementv1alpha1procurement.WatchRunRequest.new,
    hawkrprocurementv1alpha1procurement.WatchRunResponse.new,
  );

  /// CancelRun 取消一個進行中的 run。run 已不在進行中不會回錯，cancel_disposition 說明結果。
  static const cancelRun = connect.Spec(
    '/$name/CancelRun',
    connect.StreamType.unary,
    hawkrprocurementv1alpha1procurement.CancelRunRequest.new,
    hawkrprocurementv1alpha1procurement.CancelRunResponse.new,
  );

  /// ResumeRun 回覆 run 停下來等待的中斷，從中斷處接續，排入回答 run 後立即回傳 run_id。回覆格式不合
  /// （option_id 與 free_text 要恰一個、文字不可只有空白、選項要在待回答的卡上）回 INVALID_ARGUMENT 並帶欄位違規。中斷已被回覆回 FAILED_PRECONDITION，
  /// reason 是 APPROVAL_ALREADY_RESOLVED；run 不在等待輸入的狀態回 FAILED_PRECONDITION，reason 是 RUN_NOT_ACTIVE；
  /// 這個需求已有進行中的 run（包含另一個正在回答的 run）回 FAILED_PRECONDITION，reason 是 PROCUREMENT_RUN_IN_PROGRESS。
  /// 買家同時進行的 run 已達上限或每分鐘送出已達上限回 resource_exhausted（沒有 reason，帶 RetryInfo，稍後可再試）。
  static const resumeRun = connect.Spec(
    '/$name/ResumeRun',
    connect.StreamType.unary,
    hawkrprocurementv1alpha1procurement.ResumeRunRequest.new,
    hawkrprocurementv1alpha1procurement.ResumeRunResponse.new,
  );
}
/// SellerProcurementAppService 是登入的賣家的回價方法。店家取自呼叫者目前的店家，需要賣家能力 CAP_MARKETING。
/// 只能看到發給自己的詢價，也只能處理自己的回價；其他的一律回 not_found。
abstract final class SellerProcurementAppService {
  /// Fully-qualified name of the SellerProcurementAppService service.
  static const name = 'hawkr.procurement.v1alpha1.SellerProcurementAppService';

  /// ListSellerPublications 列出發給呼叫者的詢價，最新的在前。
  static const listSellerPublications = connect.Spec(
    '/$name/ListSellerPublications',
    connect.StreamType.unary,
    hawkrprocurementv1alpha1procurement.ListSellerPublicationsRequest.new,
    hawkrprocurementv1alpha1procurement.ListSellerPublicationsResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// GetSellerPublication 回傳一筆詢價的內容，加上呼叫者自己的草稿與目前的正式回價。只含呼叫者自己的回價。
  static const getSellerPublication = connect.Spec(
    '/$name/GetSellerPublication',
    connect.StreamType.unary,
    hawkrprocurementv1alpha1procurement.GetSellerPublicationRequest.new,
    hawkrprocurementv1alpha1procurement.GetSellerPublicationResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// UpsertSellerResponseDraft 建立或更新回價草稿。草稿不驗金額恆等式，買家讀不到。
  static const upsertSellerResponseDraft = connect.Spec(
    '/$name/UpsertSellerResponseDraft',
    connect.StreamType.unary,
    hawkrprocurementv1alpha1procurement.UpsertSellerResponseDraftRequest.new,
    hawkrprocurementv1alpha1procurement.UpsertSellerResponseDraftResponse.new,
  );

  /// SubmitSellerResponse 送出正式回價，產生不可變的新版本。詢價已被改版回 ABORTED，conflictTarget 是 publication；
  /// 帶的 expected_seller_response_version 不是目前的版本（賣家已改價或撤回，或空字串卻已有正式回價）回 ABORTED，
  /// conflictTarget 是 seller_response，client 必須明確告知使用者報價已變動，不得改用最新版靜默重送。
  /// 沒有填實際品相、金額恆等式不成立回 INVALID_ARGUMENT。已停止寫入（賣家已婉拒、詢價已結案或他人成交）回
  /// FAILED_PRECONDITION，reason 是 RFQ_RECIPIENT_CLOSED。
  static const submitSellerResponse = connect.Spec(
    '/$name/SubmitSellerResponse',
    connect.StreamType.unary,
    hawkrprocurementv1alpha1procurement.SubmitSellerResponseRequest.new,
    hawkrprocurementv1alpha1procurement.SubmitSellerResponseResponse.new,
  );

  /// WithdrawSellerResponse 撤回正式回價。
  static const withdrawSellerResponse = connect.Spec(
    '/$name/WithdrawSellerResponse',
    connect.StreamType.unary,
    hawkrprocurementv1alpha1procurement.WithdrawSellerResponseRequest.new,
    hawkrprocurementv1alpha1procurement.WithdrawSellerResponseResponse.new,
  );

  /// DeclineRfqPublication 婉拒一筆詢價，之後不能再回價。
  static const declineRfqPublication = connect.Spec(
    '/$name/DeclineRfqPublication',
    connect.StreamType.unary,
    hawkrprocurementv1alpha1procurement.DeclineRfqPublicationRequest.new,
    hawkrprocurementv1alpha1procurement.DeclineRfqPublicationResponse.new,
  );
}
