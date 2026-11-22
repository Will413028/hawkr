//
//  Generated code. Do not modify.
//  source: hawkr/moderation/v1alpha1/moderation.proto
//

import "package:connectrpc/connect.dart" as connect;
import "moderation.pb.dart" as hawkrmoderationv1alpha1moderation;

/// ModerationAppService 是登入的買家檢舉與讀自己的檢舉；對聊天訊息的檢舉，檢舉人是買家或店家：由訊息所屬的對話推導呼叫者是哪一方（店家由呼叫者此刻的店推導，請求不帶身分）。
/// 管理員等非顧客帳號的身分不能呼叫（permission_denied）。
abstract final class ModerationAppService {
  /// Fully-qualified name of the ModerationAppService service.
  static const name = 'hawkr.moderation.v1alpha1.ModerationAppService';

  /// SubmitReport 送出一筆檢舉。檢舉人是買家；對象是聊天訊息時，呼叫者是該訊息所屬對話的任一方（買家，或呼叫者此刻的店）都可以送，不是對話的一方回 not_found，與訊息不存在相同。
  /// 對象不存在、已刪除或不是公開可讀回 not_found；檢舉自己的內容或自己回 invalid_argument；每位檢舉人每個台北日最多新增 20 筆，
  /// 超過回 resource_exhausted。同一位檢舉人對同一個對象在案件進行中時再送，回原本那一筆（不吃額度）；案件結案後再送是新的檢舉。
  /// 帶 request_id 時，同一個 request_id 重送一律回第一次的結果，不看案件現在的狀態；內容不同回 invalid_argument。
  static const submitReport = connect.Spec(
    '/$name/SubmitReport',
    connect.StreamType.unary,
    hawkrmoderationv1alpha1moderation.SubmitReportRequest.new,
    hawkrmoderationv1alpha1moderation.SubmitReportResponse.new,
  );

  /// ListMyReports 列出呼叫者自己送出的全部檢舉（社群內容與買家的，以及店家的），新到舊（建立時間，相同時社群在前，再依 id 新到舊）。
  /// 每筆帶共用欄位與依種類的 subject：社群檢舉帶對象與結果（處理中、違規、不違規），店家檢舉帶 SellerReportAppService 的 StoreReport
  /// （含結果與結果說明；補充與佐證由 GetMyReport 讀）。page_token 不透明，換了查詢就不能沿用。停權的帳號也可以呼叫。
  static const listMyReports = connect.Spec(
    '/$name/ListMyReports',
    connect.StreamType.unary,
    hawkrmoderationv1alpha1moderation.ListMyReportsRequest.new,
    hawkrmoderationv1alpha1moderation.ListMyReportsResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// ListMyStoreReports 列出呼叫者此刻的店以店家身分送出的檢舉（對象只有聊天訊息），新到舊（建立時間，再依 id 新到舊）；請求不帶 store_id。
  /// 每筆的 subject 是 community（對象與結果）。page_token 不透明，換了查詢就不能沿用。停權的帳號也可以呼叫；沒有店回 not_found，店家退場後沒有此刻的店，所以讀不到。
  static const listMyStoreReports = connect.Spec(
    '/$name/ListMyStoreReports',
    connect.StreamType.unary,
    hawkrmoderationv1alpha1moderation.ListMyStoreReportsRequest.new,
    hawkrmoderationv1alpha1moderation.ListMyStoreReportsResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );
}
