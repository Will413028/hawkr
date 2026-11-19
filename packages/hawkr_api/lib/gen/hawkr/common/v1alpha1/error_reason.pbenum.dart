// This is a generated file - do not edit.
//
// Generated from hawkr/common/v1alpha1/error_reason.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

/// ErrorReason 是錯誤 details 裡 google.rpc.ErrorInfo.reason 可以用的值，domain 是 hawkr
/// （ADR 2026-09-25-api-error-model）。reason 放的是值名，所以值名不加 ERROR_REASON_ 前綴，與 googleapis 的
/// google/api/error_reason.proto 相同（api/buf.yaml 只對本檔排除 ENUM_VALUE_PREFIX）。
///
/// 只為 client 會分支的情境新增，而且只搭配 4xx 的錯誤碼。client 遇到不認得的 reason 時照錯誤碼處理。
/// 上線（v1）之後值名就是契約：可以新增，不得改名、刪除或改變搭配的錯誤碼。
class ErrorReason extends $pb.ProtobufEnum {
  static const ErrorReason ERROR_REASON_UNSPECIFIED =
      ErrorReason._(0, _omitEnumNames ? '' : 'ERROR_REASON_UNSPECIFIED');

  /// 依據的版本已經過時：修改時帶的 etag 與目前的不符（搭配 aborted），或以 key 建立時那筆已經存在
  /// （搭配 already_exists）。client 保留使用者的輸入，並顯示最新的版本讓使用者對照後再送出
  /// （ADR 2026-09-25-optimistic-concurrency-etag）。ErrorInfo.metadata 的 conflictTarget 可以說是哪一個物件過時，值由各 API 定義、
  /// client 不認得時照一般版本衝突處理。
  /// 商家治理（SellerGovernanceAdminService）：`report` 是請求帶的通報 etag 過時，`case` 是案件指令帶的案件 etag 過時，
  /// `target_case` 是分案或移動要加入的案件不成立（版本前進或已關閉），`category_case_opened` 是分案或移動要開新案件、而這個類別剛有了進行中的案件。`authority_order` 是主管機關來函的指令（完成、延長保存、記下告知）帶的來函 etag 過時。
  /// 採購（BuyerProcurementAppService、SellerProcurementAppService）：`request_revision` 是需求已被改版（expected_revision_id 過時），`rfq_snapshot` 是詢價確認快照過時，`candidate_set` 是候選的依據早於目前的需求版本，`approval` 是發布核准或價格預覽已過時，`publication` 是賣家看到的詢價已被改版，`seller_response` 是賣家回價的版本已不是目前的（改價、撤回，或帶「尚無正式回價」卻已有一版；client 必須明確告知使用者報價已變動，不得靜默改用最新版重送），`run` 是取消用的 run 版本過時。
  static const ErrorReason VERSION_CONFLICT =
      ErrorReason._(1, _omitEnumNames ? '' : 'VERSION_CONFLICT');

  /// 賣家帳號停權中，這個操作不在停權時仍可做的範圍（搭配 permission_denied）。client 轉成賣家的停權狀態：
  /// 顯示停權通知，並以 GetMyStore 讀停權的來源與原因。
  static const ErrorReason SELLER_SUSPENDED =
      ErrorReason._(2, _omitEnumNames ? '' : 'SELLER_SUSPENDED');

  /// 商家名單的聯絡方式與其他未封存（進行中或已入駐）的業者相同、而且沒有標記為不是同一家（搭配 failed_precondition）。ErrorInfo.metadata 的
  /// prospectIds 是相符的業者 ID（十進位，由小到大以逗號連接）。client 顯示相符的業者，讓管理員選擇合併，或確認不是同一家後
  /// 在請求帶上 confirmed_distinct_prospect_ids 再送一次（ADR 2026-10-04-prospect-contact-points-and-matching）。
  static const ErrorReason PROSPECT_MATCH_FOUND =
      ErrorReason._(3, _omitEnumNames ? '' : 'PROSPECT_MATCH_FOUND');

  /// 商家名單的統一編號已由另一筆未封存（進行中或已入駐）的業者持有：建立搭配 already_exists，修改與還原搭配
  /// failed_precondition。ErrorInfo.metadata 的 prospectIds 是持有者的 ID（十進位，只有一筆，格式同
  /// PROSPECT_MATCH_FOUND）。修改時持有者是進行中的，client 可以讓管理員與它合併；還原時是同一家就不需要還原
  /// （ADR 2026-10-04-prospect-contact-points-and-matching）。
  static const ErrorReason PROSPECT_TAX_ID_HELD =
      ErrorReason._(4, _omitEnumNames ? '' : 'PROSPECT_TAX_ID_HELD');

  /// 賣家同意的入駐文件版本不是現行版本（搭配 failed_precondition）。client 重讀
  /// seller.v1alpha1.SellerPublicService.GetSellerAgreements，顯示新版本，請賣家重新勾選後再送出。
  static const ErrorReason AGREEMENT_OUTDATED =
      ErrorReason._(5, _omitEnumNames ? '' : 'AGREEMENT_OUTDATED');

  /// 賣家做這個操作需要的前置條件沒有滿足（搭配 failed_precondition）：身分認證未核准，或店家正在隨帳號刪除。停權不用它，
  /// 停權是 SELLER_SUSPENDED。ErrorInfo.metadata 的 missingRequirements 是缺的條件，值是
  /// seller.v1alpha1.SellerRequirement 的值名（不加 SELLER_REQUIREMENT_ 前綴，例如 ONBOARDING_COMPLETE），依列舉的數字由小到大以逗號連接。client 先看 GetMyStore 的能力狀態，這個 reason 只是
  /// 伺服器逐方法檢查的後備：重讀 GetMyStore，依缺的條件顯示。
  static const ErrorReason SELLER_REQUIREMENT_MISSING =
      ErrorReason._(6, _omitEnumNames ? '' : 'SELLER_REQUIREMENT_MISSING');

  /// 帳號停權中，不能申請開店（搭配 failed_precondition）；解除停權後可以再申請。client 顯示 LocalizedMessage，並導向措施與申覆。帳號同時有生效中的終止合作時是
  /// STORE_OPENING_TERMINATED。
  static const ErrorReason STORE_OPENING_SUSPENDED =
      ErrorReason._(7, _omitEnumNames ? '' : 'STORE_OPENING_SUSPENDED');

  /// 帳號有生效中的終止合作（條款第 21.3 條；終止撤銷之前同一個帳號不能再開店），不能申請開店（搭配 failed_precondition）。client 顯示 LocalizedMessage，
  /// 並導向措施與申覆。換身分認證被另一個帳號的終止擋下的情形不用它（隱私：不透露命中的帳號或欄位），那條路徑維持通用的 failed_precondition 與文字。
  static const ErrorReason STORE_OPENING_TERMINATED =
      ErrorReason._(8, _omitEnumNames ? '' : 'STORE_OPENING_TERMINATED');

  /// 帳號擁有商店，不能被停權（搭配 failed_precondition）：停權對店家、未結訂單與撥款的效果由賣家條款決定，條款定出之前有店的帳號不停權。
  /// 只出現在後台與內部的處分路徑；client 顯示 LocalizedMessage，請營運先處理店家（措施或關店），再停權帳號。
  static const ErrorReason ACCOUNT_HAS_STORE =
      ErrorReason._(9, _omitEnumNames ? '' : 'ACCOUNT_HAS_STORE');

  /// 帳號停權中，這個操作是寫入，不在停權時仍可做的範圍（搭配 permission_denied）。停權的帳號仍能登入、讀取與刪除帳號：
  /// client 轉成帳號的受限狀態，顯示停權通知，並以 identity.v1alpha1.IdentityAppService.GetSession 的 suspension 讀停權的狀態。
  /// 舊 gateway 的 REST 路徑（/api/v1）沒有 ErrorInfo，停權的非 GET／HEAD 請求只回 403。
  static const ErrorReason ACCOUNT_SUSPENDED =
      ErrorReason._(10, _omitEnumNames ? '' : 'ACCOUNT_SUSPENDED');

  /// 要連結的登入方式（Google、LINE 的身分）已經連結到另一個帳號，包含已刪除帳號留下的身分（搭配 already_exists）；什麼都沒有改。
  /// client 顯示 LocalizedMessage，請使用者改用那個帳號登入，或換一個登入方式。
  /// IdentityAppService.LinkIdentity（web 的連結走 /api/auth，不在 Connect）。
  static const ErrorReason IDENTITY_ALREADY_LINKED =
      ErrorReason._(11, _omitEnumNames ? '' : 'IDENTITY_ALREADY_LINKED');

  /// 這個操作要求最近重新驗證過，而目前的 session 沒有（搭配 failed_precondition）：IdentityAppService.RevokeSession 要求目前的 session
  /// 在 15 分鐘內以 provider 驗證過。client 引導使用者重新走一次登入（web 的 /api/auth/{provider}/start、App 的 ExchangeIdToken，
  /// 登入會換發新 session），再重送。
  static const ErrorReason REAUTHENTICATION_REQUIRED =
      ErrorReason._(12, _omitEnumNames ? '' : 'REAUTHENTICATION_REQUIRED');

  /// 帳號沒有聯絡信箱（沒有 provider 驗證過的 email，也沒有自填的），不能送出 KYC 申請（搭配 failed_precondition）：SellerAppService.SubmitKYC。
  /// client 引導使用者在 IdentityAppService.SetContactEmail 填一個，再重送。
  static const ErrorReason CONTACT_EMAIL_REQUIRED =
      ErrorReason._(13, _omitEnumNames ? '' : 'CONTACT_EMAIL_REQUIRED');

  /// 要建立內容下架措施的內容已經有生效中的內容下架措施（搭配 failed_precondition）：一則內容同時只有一個生效中的下架措施，恢復時才不會撤銷一個、
  /// 另一個仍讓內容維持下架。client 顯示 LocalizedMessage 並重新讀取列表（列表會顯示既有的下架）。
  /// AdminSellerService.CreateMeasure 的內容下架；moderation 的 DecideCase TAKEDOWN 遇到時不報錯，改綁定既有措施。
  static const ErrorReason CONTENT_TAKEDOWN_ACTIVE =
      ErrorReason._(14, _omitEnumNames ? '' : 'CONTENT_TAKEDOWN_ACTIVE');

  /// 串流的 cursor 早於伺服器保留的事件範圍（搭配 failed_precondition）：assistant 與 procurement 的 server streaming
  /// （WatchAssistantTurn、WatchRun）。client 放棄接續，改讀 GetRuntimeSnapshot（助理則重新送出）重建畫面。
  static const ErrorReason REPLAY_CURSOR_EXPIRED =
      ErrorReason._(15, _omitEnumNames ? '' : 'REPLAY_CURSOR_EXPIRED');

  /// 串流的 cursor 超前於最新的事件（搭配 failed_precondition）：client 帶了不是伺服器發出的 cursor。處理同 REPLAY_CURSOR_EXPIRED。
  static const ErrorReason REPLAY_CURSOR_AHEAD =
      ErrorReason._(16, _omitEnumNames ? '' : 'REPLAY_CURSOR_AHEAD');

  /// 採購需求的必填欄位還沒有填滿，不能搜尋候選（搭配 failed_precondition）：StartCandidateSearch。client 回到釐清問答。
  static const ErrorReason REQUIREMENT_NOT_READY =
      ErrorReason._(17, _omitEnumNames ? '' : 'REQUIREMENT_NOT_READY');

  /// 需求有一個待回覆的中斷（run 停在等待輸入）（搭配 failed_precondition）：SubmitAnswer、Rewind 在這期間不能寫入，
  /// 否則中斷會被孤立。client 改用 ResumeRun 回覆，或 CancelRun 取消。
  static const ErrorReason APPROVAL_PENDING =
      ErrorReason._(18, _omitEnumNames ? '' : 'APPROVAL_PENDING');

  /// 要回覆的中斷已經被回覆過（搭配 failed_precondition）：ResumeRun。client 重讀 GetSnapshot。
  static const ErrorReason APPROVAL_ALREADY_RESOLVED =
      ErrorReason._(19, _omitEnumNames ? '' : 'APPROVAL_ALREADY_RESOLVED');

  /// run 的狀態不允許這個命令，因為它已不在進行中（搭配 failed_precondition）：ResumeRun。client 重讀 GetSnapshot。
  static const ErrorReason RUN_NOT_ACTIVE =
      ErrorReason._(20, _omitEnumNames ? '' : 'RUN_NOT_ACTIVE');

  /// 被接受的賣家回價版本已過有效期（搭配 failed_precondition）：AcceptSellerQuote。不可接受，也不自動改接受別版。
  static const ErrorReason RFQ_RESPONSE_EXPIRED =
      ErrorReason._(21, _omitEnumNames ? '' : 'RFQ_RESPONSE_EXPIRED');

  /// 賣家對這筆詢價已經不能再寫入（搭配 failed_precondition）：賣家已婉拒，或詢價已結案、他人已成交。
  /// SellerProcurementAppService 的 SubmitSellerResponse、WithdrawSellerResponse、UpsertSellerResponseDraft。
  static const ErrorReason RFQ_RECIPIENT_CLOSED =
      ErrorReason._(22, _omitEnumNames ? '' : 'RFQ_RECIPIENT_CLOSED');

  /// 同一張需求已經有進行中的成交（搭配 failed_precondition）：AcceptSellerQuote。ErrorInfo.metadata 的 orderNo 是既有訂單，
  /// client 引導買家去付款，不是叫他重新接受。
  static const ErrorReason RFQ_CHECKOUT_ALREADY_ACTIVE =
      ErrorReason._(23, _omitEnumNames ? '' : 'RFQ_CHECKOUT_ALREADY_ACTIVE');

  /// 同一張需求已經成交（搭配 failed_precondition）：AcceptSellerQuote。這是終局，與 RFQ_CHECKOUT_ALREADY_ACTIVE 不同，不能再付款。
  static const ErrorReason RFQ_ALREADY_AWARDED =
      ErrorReason._(24, _omitEnumNames ? '' : 'RFQ_ALREADY_AWARDED');

  /// 本期的採購額度已用完（搭配 resource_exhausted）：BuyerProcurementAppService.SubmitTurn，以及串流裡 run 失敗的 error 事件。
  /// ErrorInfo.metadata 的 remaining 是剩餘次數（此時是 0），resetTime 是下一期開始的時間（RFC 3339，UTC）。
  static const ErrorReason AGENT_MONTHLY_QUOTA_EXCEEDED =
      ErrorReason._(25, _omitEnumNames ? '' : 'AGENT_MONTHLY_QUOTA_EXCEEDED');

  /// 本週的 AI 修圖額度已用完（搭配 resource_exhausted）：ImageAiAppService.TransformImage。client 顯示額度用完、
  /// 不提供重試；同時進行的修圖超過上限也回 resource_exhausted，但沒有這個 reason，稍後可再試。
  static const ErrorReason IMAGE_QUOTA_EXCEEDED =
      ErrorReason._(26, _omitEnumNames ? '' : 'IMAGE_QUOTA_EXCEEDED');

  /// 圖片或提示詞被安全性檢查擋下（搭配 failed_precondition）：TransformImage。同一份輸入重送結果相同，
  /// client 請使用者換一張圖或改提示詞，不自動重試。額度退回。
  static const ErrorReason IMAGE_SAFETY_BLOCKED =
      ErrorReason._(27, _omitEnumNames ? '' : 'IMAGE_SAFETY_BLOCKED');

  /// 圖片涉及著作權複誦而被擋下（搭配 failed_precondition）：TransformImage。同一份輸入重送結果相同，
  /// client 請使用者換一張圖，不自動重試。額度退回。
  static const ErrorReason IMAGE_RECITATION_BLOCKED =
      ErrorReason._(28, _omitEnumNames ? '' : 'IMAGE_RECITATION_BLOCKED');

  /// 模型沒有產出圖片（搭配 failed_precondition）：TransformImage。這是模型的非決定性行為，同一份輸入重送可能成功，
  /// client 可以讓使用者直接重試。額度退回。
  static const ErrorReason IMAGE_NOT_PRODUCED =
      ErrorReason._(29, _omitEnumNames ? '' : 'IMAGE_NOT_PRODUCED');

  static const $core.List<ErrorReason> values = <ErrorReason>[
    ERROR_REASON_UNSPECIFIED,
    VERSION_CONFLICT,
    SELLER_SUSPENDED,
    PROSPECT_MATCH_FOUND,
    PROSPECT_TAX_ID_HELD,
    AGREEMENT_OUTDATED,
    SELLER_REQUIREMENT_MISSING,
    STORE_OPENING_SUSPENDED,
    STORE_OPENING_TERMINATED,
    ACCOUNT_HAS_STORE,
    ACCOUNT_SUSPENDED,
    IDENTITY_ALREADY_LINKED,
    REAUTHENTICATION_REQUIRED,
    CONTACT_EMAIL_REQUIRED,
    CONTENT_TAKEDOWN_ACTIVE,
    REPLAY_CURSOR_EXPIRED,
    REPLAY_CURSOR_AHEAD,
    REQUIREMENT_NOT_READY,
    APPROVAL_PENDING,
    APPROVAL_ALREADY_RESOLVED,
    RUN_NOT_ACTIVE,
    RFQ_RESPONSE_EXPIRED,
    RFQ_RECIPIENT_CLOSED,
    RFQ_CHECKOUT_ALREADY_ACTIVE,
    RFQ_ALREADY_AWARDED,
    AGENT_MONTHLY_QUOTA_EXCEEDED,
    IMAGE_QUOTA_EXCEEDED,
    IMAGE_SAFETY_BLOCKED,
    IMAGE_RECITATION_BLOCKED,
    IMAGE_NOT_PRODUCED,
  ];

  static final $core.List<ErrorReason?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 29);
  static ErrorReason? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const ErrorReason._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
