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

  /// ListMyReports 列出呼叫者的檢舉，新到舊。
  Future<hawkrsellerv1alpha1report.ListMyReportsResponse> listMyReports(
    hawkrsellerv1alpha1report.ListMyReportsRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.SellerReportAppService.listMyReports,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }
}
/// SellerReportPublicService 是不需要登入的檢舉入口。
extension type SellerReportPublicServiceClient (connect.Transport _transport) {
  /// SubmitFoodSafetyReport 匿名檢舉食品安全事件，必須留下電話或 email（主管機關可能需要聯絡檢舉人），不收佐證圖片（要附圖請
  /// 登入後以 CreateReport 檢舉）。店家的規則與 CreateReport 相同；呼叫者登入而且店家屬於他的帳號時回 invalid_argument（store_id）。
  /// 同一支電話或同一個 email 對同一家店，一個台北 ISO 週內、還沒有結果的那一筆存在時，這次的內容補充到它（每筆最多 10 則，超過時
  /// 內容不存）；新的檢舉、補充與超過上限的送出回應完全相同（回應沒有內容），不會透露這支電話或 email 用過。每個來源 IP
  /// （登入的呼叫者以帳號計）10 分鐘最多 5 次、1 小時最多 10 次，被拒絕的送出不計；超過回 resource_exhausted（帶 RetryInfo）。
  /// 聯絡方式在「結果設定後 30 天、且不早於送出後 90 天」清除；還沒有結果的檢舉不清除。
  Future<hawkrsellerv1alpha1report.SubmitFoodSafetyReportResponse> submitFoodSafetyReport(
    hawkrsellerv1alpha1report.SubmitFoodSafetyReportRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.SellerReportPublicService.submitFoodSafetyReport,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }
}
