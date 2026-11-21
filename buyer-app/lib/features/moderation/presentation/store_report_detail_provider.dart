import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/report.pb.dart' as pb;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../data/store_report_repository.dart';

part 'store_report_detail_provider.g.dart';

Duration? _noAutomaticRetry(int _, Object __) => null;

/// 我的一筆店家檢舉的詳情（`GetMyReport`）；補充成功後以回應的完整詳情取代狀態，不重讀。
@Riverpod(retry: _noAutomaticRetry)
class StoreReportDetail extends _$StoreReportDetail {
  @override
  Future<pb.GetMyReportResponse> build(String reportId) =>
      ref.watch(storeReportRepositoryProvider).get(reportId);

  void applySupplement(pb.AddReportSupplementResponse response) {
    state = AsyncData(
      pb.GetMyReportResponse(
        report: response.report,
        supplements: response.supplements,
        evidence: response.evidence,
      ),
    );
  }
}
