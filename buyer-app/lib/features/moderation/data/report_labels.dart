import 'package:hawkr_api/gen/hawkr/moderation/v1alpha1/moderation.pbenum.dart';

/// 我的檢舉上的對象種類。目錄沒有這個 enum（只有這支 App 用），文字屬於呈現層；UNSPECIFIED 與不認得的值回 null，不顯示。
String? reportTargetKindLabel(ReportTargetKind kind) => switch (kind) {
  ReportTargetKind.REPORT_TARGET_KIND_POST => '貼文',
  ReportTargetKind.REPORT_TARGET_KIND_COMMENT => '留言',
  ReportTargetKind.REPORT_TARGET_KIND_FORUM_ENTRY => '討論區留言',
  ReportTargetKind.REPORT_TARGET_KIND_BUYER => '使用者',
  _ => null,
};

/// 檢舉人視角的處理結果：處理中，或案件結案後的違規、不違規。UNSPECIFIED 與不認得的值回 null，不顯示。
String? reportOutcomeLabel(ReportOutcome outcome) => switch (outcome) {
  ReportOutcome.REPORT_OUTCOME_PENDING => '審核中',
  ReportOutcome.REPORT_OUTCOME_VIOLATION => '已處理（違規）',
  ReportOutcome.REPORT_OUTCOME_NO_VIOLATION => '已處理（不違規）',
  _ => null,
};
