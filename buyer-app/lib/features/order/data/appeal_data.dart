import 'package:freezed_annotation/freezed_annotation.dart';

part 'appeal_data.freezed.dart';
part 'appeal_data.g.dart';

/// 仲裁時間軸事件
///
/// 對齊 backend `GET /app/after_sales/{caseNo}/arbitration` timeline 條目。
/// eventType：`APPEAL_SUBMITTED`（買家提交申訴）、`DECISION_MADE`（平台裁決）
@freezed
abstract class AppealEvent with _$AppealEvent {
  const factory AppealEvent({
    @Default('') String eventType,
    @Default('') String timestamp,
    @Default('') String label,
    @Default('') String description,
  }) = _AppealEvent;

  factory AppealEvent.fromJson(Map<String, dynamic> json) =>
      _$AppealEventFromJson(json);
}

/// 平台裁決
///
/// outcome：`FULL_REFUND`（全額退款）/ `PARTIAL_REFUND`（部分退款，含 refundAmount）
/// / `REJECT_APPEAL`（駁回申訴）
@freezed
abstract class PlatformDecision with _$PlatformDecision {
  const factory PlatformDecision({
    @Default('') String outcome,
    @Default('') String refundAmount,
    @Default('') String reason,
    @Default('') String decidedAt,
  }) = _PlatformDecision;

  factory PlatformDecision.fromJson(Map<String, dynamic> json) =>
      _$PlatformDecisionFromJson(json);
}

/// 仲裁狀態資料（對齊 backend `GET /app/after_sales/{caseNo}/arbitration`）
///
/// state：CodeMeta `AfterSaleAppealState` 的 **Value**（`'10'` 申訴中，等待平台裁決 /
/// `'20'` 平台調查中 / `'30'` 已裁決，decision 不為 null）。後端直接把
/// `after_sale_cases.appeal_state` 欄位值放進 `state`，不做 Name 轉換。
@freezed
abstract class ArbitrationData with _$ArbitrationData {
  const factory ArbitrationData({
    @Default('') String state,
    @Default([]) List<AppealEvent> timeline,
    PlatformDecision? decision,
  }) = _ArbitrationData;

  factory ArbitrationData.fromJson(Map<String, dynamic> json) =>
      _$ArbitrationDataFromJson(json);
}

/// 仲裁狀態 enum — 值為 CodeMeta `AfterSaleAppealState` 的 Value。
/// 出處：backend-payuni/docs/代碼管理（CodeMeta）/業務狀態碼.md#AfterSaleAppealState
///
/// 後端 `ArbitrationData.state` 吐的是 Value（`aftersales.proto` 的
/// `string state = 1; // AfterSaleAppealState value`），Name 不會出現在 wire 上。
class ArbitrationState {
  ArbitrationState._();
  static const String appealed = '10';
  static const String investigating = '20';
  static const String resolved = '30';

  static String label(String state) => switch (state) {
    appealed => '申訴中，等待平台裁決',
    investigating => '平台調查中',
    resolved => '已裁決',
    _ => '未知狀態',
  };
}

/// 平台裁決結果 enum
class DecisionOutcome {
  DecisionOutcome._();
  static const String fullRefund = 'FULL_REFUND';
  static const String partialRefund = 'PARTIAL_REFUND';
  static const String rejectAppeal = 'REJECT_APPEAL';

  static String label(String outcome) => switch (outcome) {
    fullRefund => '全額退款',
    partialRefund => '部分退款',
    rejectAppeal => '駁回申訴',
    _ => outcome,
  };
}

/// 申訴 timeline 事件類型
class AppealEventType {
  AppealEventType._();
  static const String appealSubmitted = 'APPEAL_SUBMITTED';
  static const String decisionMade = 'DECISION_MADE';
}
