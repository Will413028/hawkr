import 'package:hawkr_api/gen/hawkr/common/v1alpha1/policy_category.pbenum.dart';
import 'package:hawkr_api/gen/hawkr/moderation/v1alpha1/moderation.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/moderation/v1alpha1/moderation.pb.dart'
    as pb;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/connect_provider.dart';
import 'report_target.dart';

part 'moderation_repository.g.dart';

@riverpod
ModerationRepository moderationRepository(Ref ref) =>
    ModerationRepository(ref.watch(moderationAppClientProvider));

/// 社群檢舉（`ModerationAppService`）。聊天的檢舉在 `LegacyReportRepository`。
class ModerationRepository {
  ModerationRepository(this._moderation);

  /// 我的檢舉每頁筆數（API 預設 20，上限 50）。
  static const pageSize = 20;

  final ModerationAppServiceClient _moderation;

  /// 送出檢舉，回檢舉 ID。同一個 [requestId] 重送回第一次的結果，所以同一次送出嘗試的重試要帶同一個值。
  Future<String> submitReport({
    required SocialReportTarget target,
    required PolicyCategory category,
    required String description,
    required String requestId,
  }) async {
    final response = await _moderation.submitReport(
      pb.SubmitReportRequest(
        target: pb.ReportTarget(kind: target.kind, id: target.id),
        category: category,
        description: description,
        requestId: requestId,
      ),
    );
    return response.reportId;
  }

  Future<pb.ListMyReportsResponse> listMyReports({String pageToken = ''}) =>
      _moderation.listMyReports(
        pb.ListMyReportsRequest(pageSize: pageSize, pageToken: pageToken),
      );
}
