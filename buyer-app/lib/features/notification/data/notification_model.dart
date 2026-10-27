import 'package:freezed_annotation/freezed_annotation.dart';

part 'notification_model.freezed.dart';
part 'notification_model.g.dart';

/// 後端 total / page 回傳為字串（e.g. "50"），統一轉 int
int _parseInt(dynamic value) =>
    value is int ? value : int.tryParse(value?.toString() ?? '') ?? 0;

// ============================================================
// 單個通知項目
// ============================================================

@freezed
abstract class NotificationItem with _$NotificationItem {
  const factory NotificationItem({
    @Default(0) @JsonKey(fromJson: _parseInt) int id,
    @Default('') String type,
    @Default('') String title,
    @Default('') @JsonKey(name: 'body') String content,
    String? refType,
    String? refId,
    @Default(false) bool isRead,
    DateTime? readAt,
    required DateTime createdAt,

    /// 各 type 附帶的額外資料（結構依 type 而異）
    Map<String, dynamic>? data,
  }) = _NotificationItem;

  factory NotificationItem.fromJson(Map<String, dynamic> json) =>
      _$NotificationItemFromJson(json);
}

// ============================================================
// 通知列表分頁結果
// ============================================================

@freezed
abstract class NotificationListResult with _$NotificationListResult {
  const factory NotificationListResult({
    @Default([]) @JsonKey(name: 'records') List<NotificationItem> records,
    @Default(0) @JsonKey(fromJson: _parseInt) int total,
    @Default(20) @JsonKey(name: 'pageSize', fromJson: _parseInt) int pageSize,
    @Default(false) bool hasMore,
  }) = _NotificationListResult;

  factory NotificationListResult.fromJson(Map<String, dynamic> json) =>
      _$NotificationListResultFromJson(json);
}

// ============================================================
// 未讀數量
// ============================================================

@freezed
abstract class UnreadCount with _$UnreadCount {
  const factory UnreadCount({
    @Default(0) @JsonKey(name: 'count', fromJson: _parseInt) int count,
  }) = _UnreadCount;

  factory UnreadCount.fromJson(Map<String, dynamic> json) =>
      _$UnreadCountFromJson(json);
}

// ============================================================
// 通知偏好設定（對應後端 6 個 category toggle）
// ============================================================

@freezed
abstract class NotificationPreferenceItem with _$NotificationPreferenceItem {
  const factory NotificationPreferenceItem({
    required String category,
    @Default(true) bool enabled,
    @Default('') String title,
    @Default('') String description,
  }) = _NotificationPreferenceItem;

  factory NotificationPreferenceItem.fromJson(Map<String, dynamic> json) =>
      _$NotificationPreferenceItemFromJson(json);
}
