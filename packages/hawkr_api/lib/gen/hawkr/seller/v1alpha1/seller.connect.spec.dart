//
//  Generated code. Do not modify.
//  source: hawkr/seller/v1alpha1/seller.proto
//

import "package:connectrpc/connect.dart" as connect;
import "seller.pb.dart" as hawkrsellerv1alpha1seller;

/// SellerAppService 是登入的帳號的方法。除了 ApplyStore 與措施、申覆的方法之外，都以呼叫者目前的店家操作，店家是
/// 呼叫者的現任店家，不接受指定；沒有店家時回 not_found。措施與申覆（ListMyMeasures、GetMyMeasure、SubmitAppeal）
/// 以帳號為範圍：措施的效果以帳號為單位，所有店家都已關閉的帳號照樣受處分、必須還能申覆，所以這三個方法不要求賣家角色
/// 或店家。每個方法需要的賣家能力（SellerCapability）宣告在 internal/httpapi/seller，賣家停權時不在白名單內的方法回
/// permission_denied，ErrorInfo.reason 是 SELLER_SUSPENDED；其他前置條件沒有滿足（例如身分認證未核准、店家正在隨帳號刪除）
/// 回 failed_precondition，ErrorInfo.reason 是 SELLER_REQUIREMENT_MISSING，metadata 帶缺的條件。
abstract final class SellerAppService {
  /// Fully-qualified name of the SellerAppService service.
  static const name = 'hawkr.seller.v1alpha1.SellerAppService';

  /// ApplyStore 申請開店：建立店家並授予賣家角色。前置條件依優先序檢查，第一個不符的決定回應：有生效中的終止合作回
  /// failed_precondition 與 STORE_OPENING_TERMINATED（解除停權也開不了店）；其次帳號停權中回 failed_precondition 與
  /// STORE_OPENING_SUSPENDED；其次帳號正在刪除回 failed_precondition（沒有 reason）；最後帳號已經有店家才回 already_exists。
  /// 所以被終止、停權或正在刪除的帳號即使還有店家，回的也不是 already_exists；任何一個欄位不合法都不會建立店家。官網、logo 與封面開店後以 UpdateMyStore 設定，食品業者登錄以 SubmitFoodRegistration 送審。
  static const applyStore = connect.Spec(
    '/$name/ApplyStore',
    connect.StreamType.unary,
    hawkrsellerv1alpha1seller.ApplyStoreRequest.new,
    hawkrsellerv1alpha1seller.ApplyStoreResponse.new,
  );

  /// GetMyStoreOpening 回答呼叫者的帳號此刻能不能申請開店，以及不能的原因（依優先序）。開店入口在送出表單之前讀它，不必等 ApplyStore 失敗才知道。
  /// 與 ApplyStore 的前置條件是同一份規則（伺服器推導）：can_apply_store 為 true 時 ApplyStore 不會被前置條件擋下；ApplyStore 的 ErrorReason
  /// （STORE_OPENING_SUSPENDED、STORE_OPENING_TERMINATED）仍是兩次呼叫之間狀態改變時的後備。不要求賣家角色，帳號還沒有店家也可以呼叫。
  static const getMyStoreOpening = connect.Spec(
    '/$name/GetMyStoreOpening',
    connect.StreamType.unary,
    hawkrsellerv1alpha1seller.GetMyStoreOpeningRequest.new,
    hawkrsellerv1alpha1seller.GetMyStoreOpeningResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// GetMyStore 回傳呼叫者的店家，連同停權與每個能力的狀態（含缺的條件）。停權與等待刪除的店家照樣回傳。
  static const getMyStore = connect.Spec(
    '/$name/GetMyStore',
    connect.StreamType.unary,
    hawkrsellerv1alpha1seller.GetMyStoreRequest.new,
    hawkrsellerv1alpha1seller.GetMyStoreResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// UpdateMyStore 修改店家資料的欄位（update_mask），含 logo 與封面（logo_staged_token、
  /// cover_staged_token）。食品業者登錄不在這裡（SubmitFoodRegistration）。換圖與文字欄位在同一個版本條件下一起寫入，任何一項失敗都不會寫入；沒有任何欄位改變時
  /// 不寫入，etag 不變。
  static const updateMyStore = connect.Spec(
    '/$name/UpdateMyStore',
    connect.StreamType.unary,
    hawkrsellerv1alpha1seller.UpdateMyStoreRequest.new,
    hawkrsellerv1alpha1seller.UpdateMyStoreResponse.new,
  );

  /// CompleteMyStoreOnboarding 標記新手引導完成；已經完成時不改變任何東西（etag 仍要相符）。
  static const completeMyStoreOnboarding = connect.Spec(
    '/$name/CompleteMyStoreOnboarding',
    connect.StreamType.unary,
    hawkrsellerv1alpha1seller.CompleteMyStoreOnboardingRequest.new,
    hawkrsellerv1alpha1seller.CompleteMyStoreOnboardingResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// ListMyFollowers 列出追蹤呼叫者店家的買家，新到舊。
  static const listMyFollowers = connect.Spec(
    '/$name/ListMyFollowers',
    connect.StreamType.unary,
    hawkrsellerv1alpha1seller.ListMyFollowersRequest.new,
    hawkrsellerv1alpha1seller.ListMyFollowersResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// GetMyDashboard 回傳呼叫者店家的首頁儀表板：最近 7 個完整的台北日曆日（不含今天）與前一個 7 日的銷售、粉絲、商品
  /// 瀏覽，加上訂單狀態的目前快照。數字都是原始值，不計算成長率（client 自己比較 current 與 previous）。四個區塊來自
  /// 不同的模組，各自讀取、各自逾時：讀不到的區塊不設，不會是 0，其他區塊照樣回傳，方法本身不因為單一區塊失敗而失敗。
  /// 需要行銷經營的能力（CAP_MARKETING），所以停權或等待刪除的店家回 permission_denied（SELLER_SUSPENDED）或
  /// failed_precondition。
  static const getMyDashboard = connect.Spec(
    '/$name/GetMyDashboard',
    connect.StreamType.unary,
    hawkrsellerv1alpha1seller.GetMyDashboardRequest.new,
    hawkrsellerv1alpha1seller.GetMyDashboardResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// SubmitFoodRegistration 送出食品業者登錄供平台核對（食藥署的登錄資料，人工核對）；通過後才會成為店面與商品頁公開的字號，並開放上架
  /// 加工食品。店家要在營業中，開店前後的 KYC 狀態不影響送出，平台在實名認證（KYC）核准後才能核准。已有待審的登錄時：字號相同就沿用
  /// （不改變排隊的時間），不同就撤回舊的、換成新的，所以同時最多一筆待審。審核期間目前核准的登錄照常有效。字號去掉前後空白後 1 到 50 字，
  /// 不檢查格式（沒有官方規格，交給人工核對），不合法回 invalid_argument。回傳送出之後的店家（pending_food_registration 是這筆待審）。
  static const submitFoodRegistration = connect.Spec(
    '/$name/SubmitFoodRegistration',
    connect.StreamType.unary,
    hawkrsellerv1alpha1seller.SubmitFoodRegistrationRequest.new,
    hawkrsellerv1alpha1seller.SubmitFoodRegistrationResponse.new,
  );

  /// WithdrawFoodRegistration 撤回待審的登錄（狀態成為已撤回，賣家不再看到），目前核准的那一筆保留。沒有待審的登錄回 failed_precondition。
  static const withdrawFoodRegistration = connect.Spec(
    '/$name/WithdrawFoodRegistration',
    connect.StreamType.unary,
    hawkrsellerv1alpha1seller.WithdrawFoodRegistrationRequest.new,
    hawkrsellerv1alpha1seller.WithdrawFoodRegistrationResponse.new,
  );

  /// ClearFoodRegistration 清除食品業者登錄（不再賣加工食品）：結束目前核准的那一筆、同時撤回待審的，立即生效，店面與商品頁不再顯示
  /// 字號。核准的與待審的都沒有時回 failed_precondition。
  static const clearFoodRegistration = connect.Spec(
    '/$name/ClearFoodRegistration',
    connect.StreamType.unary,
    hawkrsellerv1alpha1seller.ClearFoodRegistrationRequest.new,
    hawkrsellerv1alpha1seller.ClearFoodRegistrationResponse.new,
  );

  /// CloseMyStore 關閉店家（帳號與買家身分留著，賣家角色移除）。店家有未結訂單時回 failed_precondition；
  /// 停權中也可以關店；正在隨帳號刪除的店家由刪除流程關閉，回 failed_precondition。
  static const closeMyStore = connect.Spec(
    '/$name/CloseMyStore',
    connect.StreamType.unary,
    hawkrsellerv1alpha1seller.CloseMyStoreRequest.new,
    hawkrsellerv1alpha1seller.CloseMyStoreResponse.new,
  );

  /// SaveKYCDraft 部分更新呼叫者店家的 KYC 草稿（update_mask）。申請是店家的單例資源（沒有 Create，也沒有申請 ID）：
  /// 店家還沒有申請時，不帶 etag 的呼叫建立它（subject 必須設定其中一支）；已經有申請時 etag 必填，沒帶 etag 回
  /// already_exists、與目前的版本不符回 aborted，兩者的 ErrorInfo.reason 都是 VERSION_CONFLICT（client 的處理相同：
  /// 保留輸入、顯示最新版本）。被退回的申請在這裡回到草稿重新編輯。全部欄位先驗證才寫入，任何一欄不合法都不會寫入。
  /// 送出（SubmitKYC）才檢查必填欄位與證件是否齊全。流程狀態 can_resubmit 為 false 時回 failed_precondition，訊息是
  /// KYCFlow.resubmit_unavailabilities 的第一句。
  static const saveKYCDraft = connect.Spec(
    '/$name/SaveKYCDraft',
    connect.StreamType.unary,
    hawkrsellerv1alpha1seller.SaveKYCDraftRequest.new,
    hawkrsellerv1alpha1seller.SaveKYCDraftResponse.new,
  );

  /// GetMyKYC 回傳呼叫者店家目前的 KYC 申請（身分證號與銀行帳號只有遮蔽後的值）與流程狀態。還沒有申請（或申請已取消）
  /// 時回 not_found。
  static const getMyKYC = connect.Spec(
    '/$name/GetMyKYC',
    connect.StreamType.unary,
    hawkrsellerv1alpha1seller.GetMyKYCRequest.new,
    hawkrsellerv1alpha1seller.GetMyKYCResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// SubmitKYC 送出申請給管理員審核。草稿，以及流程狀態 can_resubmit 為 true 的申請（被退回的、PayUni 拒絕或綁定失敗而且賣家能自己修正的）都能直接送出，
  /// 不必先儲存；can_resubmit 為 false 的狀態回 failed_precondition，訊息是 KYCFlow.resubmit_unavailabilities 的第一句。必填欄位與證件齊全、身分沒有命中別的帳號生效中的處分。缺少的欄位回
  /// invalid_argument，BadRequest 的每個 field 是 KYCApplication 的欄位路徑（real_name、id_card_front、business.tax_id……，與 KYCField 一對一）。申請本身齊全之後，
  /// PayUni 綁定需要的店家資料（地址、聯絡人姓名與手機、客服電子信箱）還有缺漏時回 failed_precondition（店家的狀態不在請求裡），
  /// client 讀 GetMyKYC 的 missing_store_fields 取得缺哪些，以 UpdateMyStore 補齊。 身分比對沒有通過時回
  /// failed_precondition，訊息固定，不透露是哪個欄位或哪個帳號。每個帳號每小時最多送出 5 次，超過回
  /// resource_exhausted（帶 RetryInfo）。
  static const submitKYC = connect.Spec(
    '/$name/SubmitKYC',
    connect.StreamType.unary,
    hawkrsellerv1alpha1seller.SubmitKYCRequest.new,
    hawkrsellerv1alpha1seller.SubmitKYCResponse.new,
  );

  /// CancelKYC 取消草稿或審核中的申請；取消之後可以重新建立。
  static const cancelKYC = connect.Spec(
    '/$name/CancelKYC',
    connect.StreamType.unary,
    hawkrsellerv1alpha1seller.CancelKYCRequest.new,
    hawkrsellerv1alpha1seller.CancelKYCResponse.new,
  );

  /// LookupBusinessRegistration 以統一編號查經濟部商業司的公開登記資料，給 KYC 表單帶入公司資料。查無這個統編不是錯誤：
  /// 回應沒有 registration；資料來源暫時無法使用回 unavailable，畫面退回手動填寫。每個帳號每分鐘最多查 10 次，超過回
  /// resource_exhausted（帶 RetryInfo）。
  static const lookupBusinessRegistration = connect.Spec(
    '/$name/LookupBusinessRegistration',
    connect.StreamType.unary,
    hawkrsellerv1alpha1seller.LookupBusinessRegistrationRequest.new,
    hawkrsellerv1alpha1seller.LookupBusinessRegistrationResponse.new,
  );

  /// ListMyMeasures 列出呼叫者帳號所有店家上的措施（警告與停權），新到舊。以帳號為範圍，不要求賣家角色。state 篩選措施此刻的
  /// 狀態（例如 ACTIVE 是生效中的停權，給停權公告用）；UNSPECIFIED 不篩。
  static const listMyMeasures = connect.Spec(
    '/$name/ListMyMeasures',
    connect.StreamType.unary,
    hawkrsellerv1alpha1seller.ListMyMeasuresRequest.new,
    hawkrsellerv1alpha1seller.ListMyMeasuresResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// GetMyMeasure 回傳呼叫者帳號的一筆措施。不屬於這個帳號的措施與不存在的措施都回 not_found。
  static const getMyMeasure = connect.Spec(
    '/$name/GetMyMeasure',
    connect.StreamType.unary,
    hawkrsellerv1alpha1seller.GetMyMeasureRequest.new,
    hawkrsellerv1alpha1seller.GetMyMeasureResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// SubmitAppeal 對自己的措施提出申覆，每筆措施一次。etag 必填（措施的內容在申覆之前被管理員改過時，賣家要看到新的內容再決定）。
  /// etag 不符（管理員在讀取之後改過內容）回 aborted（VERSION_CONFLICT），不寫入。提出申覆不改變 etag，所以申覆已經生效後
  /// 再送一次（包含重送同一個請求）回 failed_precondition，不寫入；措施已經撤銷時也回 failed_precondition。已結束的措施也
  /// 可以申覆；措施已結束而仍有待處理的解除請求時，解除請求在同一個 transaction 以 CLOSED（ENDED）結案後再提出申覆。終止合作到 Measure.appeal_deadline_time
  /// 為止可以申覆（含預告期間，措施還是 SCHEDULED 時），期限已過回 failed_precondition（Measure.appeal_unavailabilities 含
  /// APPEAL_WINDOW_ENDED）。不要求賣家角色。
  static const submitAppeal = connect.Spec(
    '/$name/SubmitAppeal',
    connect.StreamType.unary,
    hawkrsellerv1alpha1seller.SubmitAppealRequest.new,
    hawkrsellerv1alpha1seller.SubmitAppealResponse.new,
  );

  /// RequestRemediation 對依據風險類型（RISK，含條款第 17.5 條的外部要求）的措施，在風險消除或外部要求解除後檢具事證請求解除
  /// （條款第 17.4 條）。可以重複提出，但同一措施同時只能有一筆待處理的請求（申覆與解除請求合計），累計被駁回的請求多了會有冷卻期
  /// （Measure.next_remediation_time）；VIOLATION 依據的措施沒有這個入口。etag 必填，規則同 SubmitAppeal：不符回 aborted
  /// （VERSION_CONFLICT）；提出請求不改變 etag，所以請求已經生效後再送一次（包含重送同一個請求）回 failed_precondition，不寫入。
  /// 措施不是生效中、不是 RISK 依據、在冷卻期內、已有待處理的請求，或已撤銷時回 failed_precondition。不要求賣家角色。
  static const requestRemediation = connect.Spec(
    '/$name/RequestRemediation',
    connect.StreamType.unary,
    hawkrsellerv1alpha1seller.RequestRemediationRequest.new,
    hawkrsellerv1alpha1seller.RequestRemediationResponse.new,
  );
}
/// SellerPublicService 是任何人都能讀的店家資料，不需要登入（ADR 2026-10-03-public-store-identity-and-visibility）。
/// 公開的店家以 store_code 識別、以 id 參照：店面頁網址用 store_code（亂數、不可列舉），各處的店家卡片用 id；
/// 兩個方法都只接受自己的鍵，不接受另一個。
abstract final class SellerPublicService {
  /// Fully-qualified name of the SellerPublicService service.
  static const name = 'hawkr.seller.v1alpha1.SellerPublicService';

  /// GetStore 以 store_code 讀取一家店的店面，連同粉絲數與評價統計。只回可以公開曝光的店家（營運中、沒有停權、身分認證
  /// 通過、店名與 logo 與所在地齊全）；店家不存在與不公開都回 not_found，兩者的回應相同。賣家預覽自己還沒公開的店面讀
  /// SellerAppService.GetMyStore。粉絲數與評價統計來自別的模組：讀取失敗時 stats 的對應欄位不設，不會是 0（見 StoreStats），
  /// 店面本身照樣回傳。
  static const getStore = connect.Spec(
    '/$name/GetStore',
    connect.StreamType.unary,
    hawkrsellerv1alpha1seller.GetStoreRequest.new,
    hawkrsellerv1alpha1seller.GetStoreResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// ListStores 列出可以公開曝光的店家，依 id 由小到大，給 sitemap 與之後的店家目錄使用。清單依最後一次公告的可曝光狀態
  /// 產生，每一頁再以 GetStore 的規則複核：店家剛變得可曝光，要等公告之後才出現；公告落後現況而此刻不可曝光的店不會列出。
  /// 一頁可以少於 page_size（甚至是空的），結束只看 next_page_token 是不是空字串，不可用「少於 page_size」判斷；翻頁途中
  /// 新增或失去可曝光的店不會讓已讀過的店重複出現。沒有可曝光的店時回空清單。
  static const listStores = connect.Spec(
    '/$name/ListStores',
    connect.StreamType.unary,
    hawkrsellerv1alpha1seller.ListStoresRequest.new,
    hawkrsellerv1alpha1seller.ListStoresResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// BatchGetStores 以 id 批次讀取店家卡片，給收藏、追蹤、聊天、訂單、貼文這類只需要店名與 logo 的地方。曾經公開過的
  /// 店家都回傳，包括停權與已關閉的（status 區分，個資清除之後 name 不設）；不存在與從沒公開過的店家不在回應裡。
  /// 回應依請求的順序，重複的 id 只回一次。一次最多 100 個 id。這是刻意的部分結果（不是 AIP-231 的整批失敗）：找不到與
  /// 從沒公開過的 id 直接略過，不回錯誤，client 以回應裡有沒有該 id 判斷（ADR 2026-10-03-public-store-identity-and-visibility）。
  static const batchGetStores = connect.Spec(
    '/$name/BatchGetStores',
    connect.StreamType.unary,
    hawkrsellerv1alpha1seller.BatchGetStoresRequest.new,
    hawkrsellerv1alpha1seller.BatchGetStoresResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// GetSellerAgreements 讀取賣家入駐時要同意的文件的現行版本與網址，全部種類都回，依 AgreementKind 的數字順序。內容是公開的，
  /// 開店前就要能讀，所以不需要登入。client 把這裡的版本顯示給賣家勾選，再原樣帶回 ApplyStore、SubmitKYC 與 SubmitFoodRegistration 的
  /// accepted_agreements。
  static const getSellerAgreements = connect.Spec(
    '/$name/GetSellerAgreements',
    connect.StreamType.unary,
    hawkrsellerv1alpha1seller.GetSellerAgreementsRequest.new,
    hawkrsellerv1alpha1seller.GetSellerAgreementsResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// RecordProductView 記錄一次商品頁的瀏覽，只計訪客：登入的買家由買家的瀏覽紀錄計入，登入的呼叫者收到成功的回應但
  /// 不計數（不會重複計算，也不必讓 client 判斷登入狀態）。商品所屬的店家由背景程序解析，不存在的商品不回錯。
  /// 每個來源 IP 每分鐘最多 120 次，超過回 resource_exhausted（帶 RetryInfo）。
  static const recordProductView = connect.Spec(
    '/$name/RecordProductView',
    connect.StreamType.unary,
    hawkrsellerv1alpha1seller.RecordProductViewRequest.new,
    hawkrsellerv1alpha1seller.RecordProductViewResponse.new,
  );
}
