//
//  Generated code. Do not modify.
//  source: hawkr/seller/v1alpha1/report.proto
//

import "package:connectrpc/connect.dart" as connect;
import "report.pb.dart" as hawkrsellerv1alpha1report;
import "report.connect.spec.dart" as specs;

/// SellerReportAppService 是登入的買家的檢舉。
extension type SellerReportAppServiceClient (connect.Transport _transport) {
  /// CreateReport 以呼叫者的買家身分檢舉一家店。店家必須是曾經公開過的店（BatchGetStores 回得到的；已關閉的也可以），否則回
  /// not_found；店家屬於呼叫者的帳號時回 invalid_argument（store_id）。同一個買家對同一家店、同一個類別，一個台北日曆日內、
  /// 還沒有結果的那一筆存在時，這次的內容補充到它（supplemented 為 true，report 是原本那一筆，notice 說明發生了什麼）；原本那一筆
  /// 已經有結果時，這次是新的檢舉。原本那一筆已有 10 則補充時回 failed_precondition。每個帳號 10 分鐘最多 5 次、1 小時最多 10 次，
  /// 超過回 resource_exhausted（帶 RetryInfo）。
  Future<hawkrsellerv1alpha1report.CreateReportResponse> createReport(
    hawkrsellerv1alpha1report.CreateReportRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.SellerReportAppService.createReport,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// GetMyReport 讀呼叫者自己的一筆店家檢舉：檢舉本身、補充與佐證。別人的或不存在的 report_id 一律回 not_found（不洩漏存在與否）。
  /// 呼叫者的全部檢舉（社群與店家）的清單是 hawkr.moderation.v1alpha1.ModerationAppService.ListMyReports。
  Future<hawkrsellerv1alpha1report.GetMyReportResponse> getMyReport(
    hawkrsellerv1alpha1report.GetMyReportRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.SellerReportAppService.getMyReport,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// AddReportSupplement 為呼叫者自己、還沒有結果的一筆店家檢舉補充內容。與 CreateReport 去重時併入的補充是同一串、同一個上限
  /// （每筆檢舉合計最多 10 則），admin 看到的是同一串補充。別人的或不存在的 report_id 回 not_found；檢舉已經有結果，或補充已
  /// 達 10 則時回 failed_precondition。每個帳號 10 分鐘最多 5 次、1 小時最多 10 次（與 CreateReport 同樣的額度，各自計數），超過回
  /// resource_exhausted（帶 RetryInfo）。
  Future<hawkrsellerv1alpha1report.AddReportSupplementResponse> addReportSupplement(
    hawkrsellerv1alpha1report.AddReportSupplementRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.SellerReportAppService.addReportSupplement,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }
}
