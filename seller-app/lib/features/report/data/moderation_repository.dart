import 'package:hawkr_api/gen/hawkr/common/v1alpha1/policy_category.pbenum.dart';
import 'package:hawkr_api/gen/hawkr/moderation/v1alpha1/moderation.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/moderation/v1alpha1/moderation.pb.dart'
    as pb;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/connect_provider.dart';

part 'moderation_repository.g.dart';

@riverpod
ModerationRepository moderationRepository(Ref ref) =>
    ModerationRepository(ref.watch(moderationAppClientProvider));

/// 店家檢舉（`ModerationAppService.SubmitReport`）。聊天訊息是店家唯一能檢舉的對象；檢舉人是呼叫者此刻的店，
/// 由伺服器依訊息所屬的對話推導，請求不帶身分。
class ModerationRepository {
  ModerationRepository(this._moderation);

  final ModerationAppServiceClient _moderation;

  /// 送出對聊天訊息 [messageId] 的檢舉，回檢舉 ID。同一個 [requestId] 重送回第一次的結果，
  /// 所以同一次送出嘗試的重試要帶同一個值。
  Future<String> reportChatMessage({
    required String messageId,
    required PolicyCategory category,
    required String description,
    required String requestId,
  }) async {
    final response = await _moderation.submitReport(
      pb.SubmitReportRequest(
        target: pb.ReportTarget(
          kind: pb.ReportTargetKind.REPORT_TARGET_KIND_CHAT_MESSAGE,
          id: messageId,
        ),
        category: category,
        description: description,
        requestId: requestId,
      ),
    );
    return response.reportId;
  }
}
