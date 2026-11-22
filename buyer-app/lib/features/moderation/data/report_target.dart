import 'package:hawkr_api/gen/hawkr/moderation/v1alpha1/moderation.pbenum.dart';

/// 檢舉的對象：種類加伺服器擁有的 ID，一律送 `ModerationAppService.SubmitReport`。
/// 評價與聊天訊息只送參照（評價 ID、訊息 ID），不送文字或快照。
final class ReportTarget {
  const ReportTarget(this.kind, this.id);

  const ReportTarget.post(String id)
    : this(ReportTargetKind.REPORT_TARGET_KIND_POST, id);

  const ReportTarget.comment(String id)
    : this(ReportTargetKind.REPORT_TARGET_KIND_COMMENT, id);

  const ReportTarget.forumEntry(String id)
    : this(ReportTargetKind.REPORT_TARGET_KIND_FORUM_ENTRY, id);

  /// 買家寫的評價。
  const ReportTarget.review(String id)
    : this(ReportTargetKind.REPORT_TARGET_KIND_REVIEW, id);

  /// 買家與店家之間的一則聊天訊息。
  const ReportTarget.chatMessage(String id)
    : this(ReportTargetKind.REPORT_TARGET_KIND_CHAT_MESSAGE, id);

  final ReportTargetKind kind;
  final String id;
}
