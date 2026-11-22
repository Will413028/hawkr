import 'package:hawkr_api/gen/hawkr/moderation/v1alpha1/moderation.pbenum.dart';

/// 檢舉的對象，一律走 `ModerationAppService`，只送種類與 ID 參照。
sealed class ReportTarget {
  const ReportTarget();
}

final class SocialReportTarget extends ReportTarget {
  const SocialReportTarget(this.kind, this.id);

  const SocialReportTarget.post(String id)
    : this(ReportTargetKind.REPORT_TARGET_KIND_POST, id);

  const SocialReportTarget.comment(String id)
    : this(ReportTargetKind.REPORT_TARGET_KIND_COMMENT, id);

  const SocialReportTarget.forumEntry(String id)
    : this(ReportTargetKind.REPORT_TARGET_KIND_FORUM_ENTRY, id);

  /// 買家寫的評價；只送評價 ID 參照。
  const SocialReportTarget.review(String id)
    : this(ReportTargetKind.REPORT_TARGET_KIND_REVIEW, id);

  /// 買家與店家之間的一則聊天訊息；只送訊息 ID 參照（伺服器由訊息的歸屬推導檢舉人與被檢舉的店家）。
  const SocialReportTarget.chatMessage(String id)
    : this(ReportTargetKind.REPORT_TARGET_KIND_CHAT_MESSAGE, id);

  final ReportTargetKind kind;
  final String id;
}
