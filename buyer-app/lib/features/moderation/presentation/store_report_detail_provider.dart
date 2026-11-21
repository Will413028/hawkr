import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/report.pb.dart' as pb;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../data/store_report_repository.dart';

part 'store_report_detail_provider.g.dart';

/// 我的一筆店家檢舉的詳情（`GetMyReport`）；補充成功後 `ref.invalidate` 重讀。
@riverpod
Future<pb.GetMyReportResponse> storeReportDetail(Ref ref, String reportId) =>
    ref.watch(storeReportRepositoryProvider).get(reportId);
