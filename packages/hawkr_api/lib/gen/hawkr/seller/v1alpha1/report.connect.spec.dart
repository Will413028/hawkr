//
//  Generated code. Do not modify.
//  source: hawkr/seller/v1alpha1/report.proto
//

import "package:connectrpc/connect.dart" as connect;
import "report.pb.dart" as hawkrsellerv1alpha1report;

/// SellerReportAppService 是登入的買家的檢舉。
abstract final class SellerReportAppService {
  /// Fully-qualified name of the SellerReportAppService service.
  static const name = 'hawkr.seller.v1alpha1.SellerReportAppService';

  /// CreateReport 以呼叫者的買家身分檢舉一家店。店家必須是曾經公開過的店（BatchGetStores 回得到的；已關閉的也可以），否則回
  /// not_found；店家屬於呼叫者的帳號時回 invalid_argument（store_id）。同一個買家對同一家店、同一個類別，一個台北日曆日內、
  /// 還沒有結果的那一筆存在時，這次的內容補充到它（supplemented 為 true，report 是原本那一筆，notice 說明發生了什麼）；原本那一筆
  /// 已經有結果時，這次是新的檢舉。原本那一筆已有 10 則補充時回 failed_precondition。每個帳號 10 分鐘最多 5 次、1 小時最多 10 次，
  /// 超過回 resource_exhausted（帶 RetryInfo）。
  static const createReport = connect.Spec(
    '/$name/CreateReport',
    connect.StreamType.unary,
    hawkrsellerv1alpha1report.CreateReportRequest.new,
    hawkrsellerv1alpha1report.CreateReportResponse.new,
  );

  /// ListMyReports 列出呼叫者的檢舉，新到舊。
  static const listMyReports = connect.Spec(
    '/$name/ListMyReports',
    connect.StreamType.unary,
    hawkrsellerv1alpha1report.ListMyReportsRequest.new,
    hawkrsellerv1alpha1report.ListMyReportsResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );
}
