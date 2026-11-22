import 'package:hawkr_api/gen/hawkr/moderation/v1alpha1/moderation.pbenum.dart';

import 'report_target_type.dart' as legacy;

/// 檢舉的對象。社群內容（貼文、留言、討論區留言）與評價走 `ModerationAppService`；聊天訊息仍走舊
/// `/app/reports`（A6 前的允許殘留）。兩者以型別分開，呼叫端不用字串判斷走哪條。
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

  final ReportTargetKind kind;
  final String id;
}

final class LegacyReportTarget extends ReportTarget {
  const LegacyReportTarget(this.type, this.id);

  final legacy.ReportTargetType type;
  final String id;
}
