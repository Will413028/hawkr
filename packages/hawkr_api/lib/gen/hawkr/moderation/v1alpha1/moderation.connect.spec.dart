//
//  Generated code. Do not modify.
//  source: hawkr/moderation/v1alpha1/moderation.proto
//

import "package:connectrpc/connect.dart" as connect;
import "moderation.pb.dart" as hawkrmoderationv1alpha1moderation;

/// ModerationAppService 是登入的買家檢舉與讀自己的檢舉；管理員等非顧客帳號的身分不能呼叫（permission_denied）。
abstract final class ModerationAppService {
  /// Fully-qualified name of the ModerationAppService service.
  static const name = 'hawkr.moderation.v1alpha1.ModerationAppService';

  /// SubmitReport 送出一筆檢舉。對象不存在、已刪除或不是公開可讀回 not_found；檢舉自己的內容或自己回 invalid_argument；每位檢舉人每個台北日最多新增 20 筆，
  /// 超過回 resource_exhausted。同一位檢舉人對同一個對象在案件進行中時再送，回原本那一筆（不吃額度）；案件結案後再送是新的檢舉。
  /// 帶 request_id 時，同一個 request_id 重送一律回第一次的結果，不看案件現在的狀態；內容不同回 invalid_argument。
  static const submitReport = connect.Spec(
    '/$name/SubmitReport',
    connect.StreamType.unary,
    hawkrmoderationv1alpha1moderation.SubmitReportRequest.new,
    hawkrmoderationv1alpha1moderation.SubmitReportResponse.new,
  );

  /// ListMyReports 列出呼叫者自己送出的檢舉，新到舊。每筆帶對象、原因、說明與結果（處理中、違規、不違規）。停權的帳號也可以呼叫。
  static const listMyReports = connect.Spec(
    '/$name/ListMyReports',
    connect.StreamType.unary,
    hawkrmoderationv1alpha1moderation.ListMyReportsRequest.new,
    hawkrmoderationv1alpha1moderation.ListMyReportsResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );
}
