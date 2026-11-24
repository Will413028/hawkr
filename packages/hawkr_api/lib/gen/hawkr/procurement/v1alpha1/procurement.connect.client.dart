//
//  Generated code. Do not modify.
//  source: hawkr/procurement/v1alpha1/procurement.proto
//

import "package:connectrpc/connect.dart" as connect;
import "procurement.pb.dart" as hawkrprocurementv1alpha1procurement;
import "procurement.connect.spec.dart" as specs;

/// BuyerProcurementAppService 是登入的買家的採購方法。身分取自 session，帳號沒有買家角色時回 permission_denied。
/// 不屬於呼叫者的需求、對話與 run 一律回 not_found。
extension type BuyerProcurementAppServiceClient (connect.Transport _transport) {
  /// SubmitTurn 送出買家的一則訊息，開始一輪釐清。request_id 空字串表示新建需求。伺服器受理後立刻回 run ID，
  /// 這一輪的過程與結果（turn_result 事件）從 WatchRun 取得。這個需求已有進行中的 run 回 FAILED_PRECONDITION，
  /// reason 是 PROCUREMENT_RUN_IN_PROGRESS。買家同時進行的 run 已達上限或每分鐘送出已達上限回 resource_exhausted（沒有 reason，帶 RetryInfo，稍後可再試）；
  /// 本期額度用完回 resource_exhausted，reason 是 AGENT_MONTHLY_QUOTA_EXCEEDED。
  Future<hawkrprocurementv1alpha1procurement.SubmitTurnResponse> submitTurn(
    hawkrprocurementv1alpha1procurement.SubmitTurnRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerProcurementAppService.submitTurn,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// GetSnapshot 回傳一張需求目前的內容與狀態。
  Future<hawkrprocurementv1alpha1procurement.GetSnapshotResponse> getSnapshot(
    hawkrprocurementv1alpha1procurement.GetSnapshotRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerProcurementAppService.getSnapshot,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// StartCandidateSearch 開始為需求搜尋候選賣家（背景執行，以 GetCandidateSearch 取得結果）。
  /// 需求還沒齊備回 FAILED_PRECONDITION，reason 是 REQUIREMENT_NOT_READY；這個需求已有進行中的 run 回 FAILED_PRECONDITION，
  /// reason 是 PROCUREMENT_RUN_IN_PROGRESS。買家同時進行的 run 已達上限或每分鐘送出已達上限回 resource_exhausted（沒有 reason，帶 RetryInfo，稍後可再試）。
  Future<hawkrprocurementv1alpha1procurement.StartCandidateSearchResponse> startCandidateSearch(
    hawkrprocurementv1alpha1procurement.StartCandidateSearchRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerProcurementAppService.startCandidateSearch,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// GetCandidateSearch 回傳一次候選搜尋的狀態與目前的候選。
  Future<hawkrprocurementv1alpha1procurement.GetCandidateSearchResponse> getCandidateSearch(
    hawkrprocurementv1alpha1procurement.GetCandidateSearchRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerProcurementAppService.getCandidateSearch,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// ToggleSelection 勾選或取消勾選一個候選。候選所屬的候選集不是有效的候選集（需求已不在 MATCHING，或有更新的搜尋成功）回 ABORTED，conflictTarget 是 candidate_set。
  Future<hawkrprocurementv1alpha1procurement.ToggleSelectionResponse> toggleSelection(
    hawkrprocurementv1alpha1procurement.ToggleSelectionRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerProcurementAppService.toggleSelection,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// SubmitAnswer 回答一個釐清問題。需求有待回覆的中斷時改用 ResumeRun，回 FAILED_PRECONDITION，
  /// reason 是 APPROVAL_PENDING。只收確定性的回答（選項、數量、略過；品項文字須唯一對到一個末層分類），
  /// 品項文字對到多個或零個分類回 INVALID_ARGUMENT（field 是 free_text），請改用 SubmitTurn。
  Future<hawkrprocurementv1alpha1procurement.SubmitAnswerResponse> submitAnswer(
    hawkrprocurementv1alpha1procurement.SubmitAnswerRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerProcurementAppService.submitAnswer,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// Rewind 讓需求退回某個欄位重新回答。有待回覆的中斷時規則同 SubmitAnswer。
  Future<hawkrprocurementv1alpha1procurement.RewindResponse> rewind(
    hawkrprocurementv1alpha1procurement.RewindRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerProcurementAppService.rewind,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// PreviewRfqPrice 依買家預期的單價範圍與需求數量算出詢價總額區間；純計算，不寫入任何資料。
  /// 買家沒有給合法的範圍（下界與上界是正整數、下界不大於上界）回 INVALID_ARGUMENT；
  /// 需求狀態不是 REQUIREMENT_READY 或 MATCHING，或還沒有單位（算不出計價基準）回 FAILED_PRECONDITION，reason 是 REQUIREMENT_NOT_READY；
  /// 需求已有數量與單位卻算不出總額（溢位）回 FAILED_PRECONDITION，沒有 reason。
  /// 讀到的需求版本不是現行版本回 ABORTED，conflictTarget 是 request_revision；需求不屬於呼叫者回 NOT_FOUND。
  Future<hawkrprocurementv1alpha1procurement.PreviewRfqPriceResponse> previewRfqPrice(
    hawkrprocurementv1alpha1procurement.PreviewRfqPriceRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerProcurementAppService.previewRfqPrice,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// PreviewRfqPublication 依需求現行版本、買家的單價範圍與補充說明編出要發給賣家的內容，並建立不可變、只能執行一次的發布核准（有效 24 小時），
  /// 不發布、不通知。核准存單價範圍、補充說明、回價期限、受眾與候選集。
  /// 需求版本過時回 ABORTED，conflictTarget 是 request_revision；候選集不是有效的候選集（見 ToggleSelection）回 ABORTED，conflictTarget 是 candidate_set。
  /// 需求狀態不是 REQUIREMENT_READY 或 MATCHING，或還沒有單位，回 FAILED_PRECONDITION，reason 是 REQUIREMENT_NOT_READY。
  /// 候選集不屬於呼叫者回 NOT_FOUND；單價範圍不合法、buyer_note 超過 500 字、受眾欄位不合（沒帶候選集或候選、候選重複、候選不在候選集裡）、
  /// 回價期限不是合法日期或早於今天（台北時區）回 INVALID_ARGUMENT。
  Future<hawkrprocurementv1alpha1procurement.PreviewRfqPublicationResponse> previewRfqPublication(
    hawkrprocurementv1alpha1procurement.PreviewRfqPublicationRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerProcurementAppService.previewRfqPublication,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// ExecuteRfqPublication 以發布核准的版本執行發布。成功只代表伺服器已建立發布，不代表已投遞或賣家已看到；不建 run。
  /// 需求版本過時回 ABORTED，conflictTarget 是 request_revision；核准不是這個需求的、版本不符、已執行過、已過期、凍結的需求版本已不是現行版本、
  /// 核准的回價期限已早於今天（台北時區），或 payload_hash 與核准的內容不符回 ABORTED，conflictTarget 是 publication_approval；
  /// 候選集已不是有效的候選集回 ABORTED，conflictTarget 是 candidate_set；核准不屬於呼叫者回 NOT_FOUND。
  /// 需求狀態不是 REQUIREMENT_READY 或 MATCHING 回 FAILED_PRECONDITION，reason 是 REQUIREMENT_NOT_READY。
  /// 重送同一個 client_request_id 回第一次的發布（replayed 為 true），內容不同回 INVALID_ARGUMENT。
  Future<hawkrprocurementv1alpha1procurement.ExecuteRfqPublicationResponse> executeRfqPublication(
    hawkrprocurementv1alpha1procurement.ExecuteRfqPublicationRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerProcurementAppService.executeRfqPublication,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// ListConversations 列出買家的對話，最近更新的在前。
  Future<hawkrprocurementv1alpha1procurement.ListConversationsResponse> listConversations(
    hawkrprocurementv1alpha1procurement.ListConversationsRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerProcurementAppService.listConversations,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  Future<hawkrprocurementv1alpha1procurement.GetConversationMessagesResponse> getConversationMessages(
    hawkrprocurementv1alpha1procurement.GetConversationMessagesRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerProcurementAppService.getConversationMessages,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// GetRuntimeSnapshot 回傳重建一個對話畫面需要的全部事實：訊息、run、待回覆的中斷與發布結果。
  Future<hawkrprocurementv1alpha1procurement.GetRuntimeSnapshotResponse> getRuntimeSnapshot(
    hawkrprocurementv1alpha1procurement.GetRuntimeSnapshotRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerProcurementAppService.getRuntimeSnapshot,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// ListSellerResponses 列出一張需求已發布的詢價收到的賣家回價（每位賣家的目前版本）。
  Future<hawkrprocurementv1alpha1procurement.ListSellerResponsesResponse> listSellerResponses(
    hawkrprocurementv1alpha1procurement.ListSellerResponsesRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerProcurementAppService.listSellerResponses,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// PutShortlistEntry 把賣家回價的一個版本加入入圍清單；同一則回價已在清單時換成請求帶的版本（一則回價只有一個版本在清單），回清單上掛的版本。
  Future<hawkrprocurementv1alpha1procurement.PutShortlistEntryResponse> putShortlistEntry(
    hawkrprocurementv1alpha1procurement.PutShortlistEntryRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerProcurementAppService.putShortlistEntry,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// DeleteShortlistEntry 把賣家回價從入圍清單移除（以回價為單位，不看請求帶的版本）；回被移除那一筆掛的版本，不在清單時回請求帶的版本，present 都是 false。
  Future<hawkrprocurementv1alpha1procurement.DeleteShortlistEntryResponse> deleteShortlistEntry(
    hawkrprocurementv1alpha1procurement.DeleteShortlistEntryRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerProcurementAppService.deleteShortlistEntry,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// AcceptSellerQuote 接受賣家回價的指定版本並凍結成交價，隨後建立待付款訂單。報價已改版、已撤回回 ABORTED，
  /// conflictTarget 是 seller_response；報價已過有效期回 FAILED_PRECONDITION，reason 是 RFQ_RESPONSE_EXPIRED；
  /// 同一張需求已有進行中的成交回 FAILED_PRECONDITION，reason 是 RFQ_CHECKOUT_ALREADY_ACTIVE（metadata 的 orderNo 是
  /// 既有訂單，買家去付款，不是重新接受）。
  Future<hawkrprocurementv1alpha1procurement.AcceptSellerQuoteResponse> acceptSellerQuote(
    hawkrprocurementv1alpha1procurement.AcceptSellerQuoteRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerProcurementAppService.acceptSellerQuote,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  Future<hawkrprocurementv1alpha1procurement.RenameConversationResponse> renameConversation(
    hawkrprocurementv1alpha1procurement.RenameConversationRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerProcurementAppService.renameConversation,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  Future<hawkrprocurementv1alpha1procurement.DeleteConversationResponse> deleteConversation(
    hawkrprocurementv1alpha1procurement.DeleteConversationRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerProcurementAppService.deleteConversation,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// WatchRun 訂閱一個 run 的事件，從 cursor 之後開始（空字串表示從頭）。只讀；伺服器在 run 結束（finished 或 error 事件之後）
  /// 或滿 100 秒時結束串流。每位買家同時訂閱的串流有上限，超過回 resource_exhausted。
  Stream<hawkrprocurementv1alpha1procurement.WatchRunResponse> watchRun(
    hawkrprocurementv1alpha1procurement.WatchRunRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).server(
      specs.BuyerProcurementAppService.watchRun,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// CancelRun 取消一個進行中的 run。run 已不在進行中不會回錯，cancel_disposition 說明結果。
  Future<hawkrprocurementv1alpha1procurement.CancelRunResponse> cancelRun(
    hawkrprocurementv1alpha1procurement.CancelRunRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerProcurementAppService.cancelRun,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// ResumeRun 回覆 run 停下來等待的中斷，從中斷處接續，排入回答 run 後立即回傳 run_id。回覆格式不合
  /// （option_id 與 free_text 要恰一個、文字不可只有空白、選項要在待回答的卡上）回 INVALID_ARGUMENT 並帶欄位違規。中斷已被回覆回 FAILED_PRECONDITION，
  /// reason 是 APPROVAL_ALREADY_RESOLVED；run 不在等待輸入的狀態回 FAILED_PRECONDITION，reason 是 RUN_NOT_ACTIVE；
  /// 這個需求已有進行中的 run（包含另一個正在回答的 run）回 FAILED_PRECONDITION，reason 是 PROCUREMENT_RUN_IN_PROGRESS。
  /// 買家同時進行的 run 已達上限或每分鐘送出已達上限回 resource_exhausted（沒有 reason，帶 RetryInfo，稍後可再試）。
  Future<hawkrprocurementv1alpha1procurement.ResumeRunResponse> resumeRun(
    hawkrprocurementv1alpha1procurement.ResumeRunRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerProcurementAppService.resumeRun,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }
}
/// SellerProcurementAppService 是登入的賣家的回價方法。店家取自呼叫者目前的店家，需要賣家能力 CAP_MARKETING。
/// 只能看到發給自己的詢價，也只能處理自己的回價；其他的一律回 not_found（包含詢價的買家已刪除：被邀請的店照常讀得到）。
/// 寫入（UpsertSellerResponseDraft、SubmitSellerResponse、WithdrawSellerResponse）在下列任一情況回 FAILED_PRECONDITION，
/// reason 是 RFQ_RECIPIENT_CLOSED：賣家已婉拒這筆詢價、這張需求已有進行中的成交（買家已接受某家報價且尚未因建單失敗而解除）、
/// 買家已刪除帳號（沒有人能再接受報價）、詢價的回價期限（response_deadline，台北時區當日結束）已過。讀取不受影響。
/// DeclineRfqPublication 不受這些條件限制（婉拒隨時可以，重複婉拒回第一次的結果）。
extension type SellerProcurementAppServiceClient (connect.Transport _transport) {
  /// ListSellerPublications 列出發給呼叫者的詢價，最新的在前。
  Future<hawkrprocurementv1alpha1procurement.ListSellerPublicationsResponse> listSellerPublications(
    hawkrprocurementv1alpha1procurement.ListSellerPublicationsRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.SellerProcurementAppService.listSellerPublications,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// GetSellerPublication 回傳一筆詢價的內容，加上呼叫者自己的草稿與目前的正式回價。只含呼叫者自己的回價。
  Future<hawkrprocurementv1alpha1procurement.GetSellerPublicationResponse> getSellerPublication(
    hawkrprocurementv1alpha1procurement.GetSellerPublicationRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.SellerProcurementAppService.getSellerPublication,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// UpsertSellerResponseDraft 建立或更新回價草稿。草稿不驗金額恆等式與必填欄位，買家讀不到；只驗形狀與長度：
  /// 文字欄不得超過正式回價的上限，整份草稿序列化後不得超過 16 KiB，超過回 INVALID_ARGUMENT 並指出欄位。
  Future<hawkrprocurementv1alpha1procurement.UpsertSellerResponseDraftResponse> upsertSellerResponseDraft(
    hawkrprocurementv1alpha1procurement.UpsertSellerResponseDraftRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.SellerProcurementAppService.upsertSellerResponseDraft,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// SubmitSellerResponse 送出正式回價，產生不可變的新版本。詢價已被改版回 ABORTED，conflictTarget 是 publication；
  /// 帶的 expected_seller_response_version 不是目前的版本（賣家已改價或撤回，或空字串卻已有正式回價）回 ABORTED，
  /// conflictTarget 是 seller_response，client 必須明確告知使用者報價已變動，不得改用最新版靜默重送。
  /// spec_note 超過 1000 字、金額恆等式不成立、必填欄位缺漏或格式不合回 INVALID_ARGUMENT。已停止寫入（見檔頭）回
  /// FAILED_PRECONDITION，reason 是 RFQ_RECIPIENT_CLOSED。重送同一個 client_request_id：內容相同回第一次的版本（replayed 為 true、不建新版本，
  /// 先於版本與停止寫入的檢查），內容不同回 INVALID_ARGUMENT。幣別一律是 TWD，伺服器補上。
  Future<hawkrprocurementv1alpha1procurement.SubmitSellerResponseResponse> submitSellerResponse(
    hawkrprocurementv1alpha1procurement.SubmitSellerResponseRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.SellerProcurementAppService.submitSellerResponse,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// WithdrawSellerResponse 撤回指定版本的正式回價（目標狀態的方法）：該版已撤回時回目前的結案狀態，不改動。
  /// expected_seller_response_version 不是目前的版本回 ABORTED，conflictTarget 是 seller_response。已停止寫入回
  /// FAILED_PRECONDITION，reason 是 RFQ_RECIPIENT_CLOSED。回價不是呼叫者的回 NOT_FOUND。
  Future<hawkrprocurementv1alpha1procurement.WithdrawSellerResponseResponse> withdrawSellerResponse(
    hawkrprocurementv1alpha1procurement.WithdrawSellerResponseRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.SellerProcurementAppService.withdrawSellerResponse,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// DeclineRfqPublication 婉拒一筆詢價，之後不能再回價（目標狀態的方法）：已婉拒時回第一次的結案狀態，不改動。已提交的正式回價保留，
  /// 買家端不能再接受它。
  Future<hawkrprocurementv1alpha1procurement.DeclineRfqPublicationResponse> declineRfqPublication(
    hawkrprocurementv1alpha1procurement.DeclineRfqPublicationRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.SellerProcurementAppService.declineRfqPublication,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }
}
