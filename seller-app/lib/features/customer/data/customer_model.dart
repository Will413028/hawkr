import 'package:freezed_annotation/freezed_annotation.dart';

part 'customer_model.freezed.dart';
part 'customer_model.g.dart';

/// 標籤（商家標籤 / 顧客標籤共用）
@freezed
abstract class CustomerTag with _$CustomerTag {
  const factory CustomerTag({required int id, required String name}) =
      _CustomerTag;

  factory CustomerTag.fromJson(Map<String, dynamic> json) =>
      _$CustomerTagFromJson(json);
}

@freezed
abstract class Customer with _$Customer {
  const factory Customer({
    /// 客戶識別碼，值語意是 buyer_id（API `buyerId`）。
    required String id,
    required String name,
    required String phone,
    @Default(0) int totalSpent,

    /// 訂單數（API `order_count`）
    @Default(0) int visitCount,
    @Default(0) int couponCount,
    @Default(false) bool isVip,
    @Default(false) bool isBlacklisted,

    /// API 原始值：'normal' / 'vip' / 'blocklist'
    @Default('normal') String customerType,
    String? avatarUrl,
    String? address,
    String? email,
    @Default([]) List<String> frequentItems,
    @Default([]) List<CustomerTag> tags,
  }) = _Customer;

  factory Customer.fromJson(Map<String, dynamic> json) =>
      _$CustomerFromJson(json);
}

@freezed
abstract class SpendingRecord with _$SpendingRecord {
  const factory SpendingRecord({
    required String date,
    required String orderId,
    required int amount,
    required int refund,

    /// API `discountAmount` — 折扣金額
    @Default(0) int discountAmount,
  }) = _SpendingRecord;

  factory SpendingRecord.fromJson(Map<String, dynamic> json) =>
      _$SpendingRecordFromJson(json);
}

enum OrderStatus { delivering, completed, cancelled, issue }

OrderStatus _parseOrderStatus(dynamic value) {
  if (value is int) {
    switch (value) {
      case 0:
        return OrderStatus.delivering;
      case 1:
        return OrderStatus.completed;
      case 2:
        return OrderStatus.cancelled;
      case 3:
        return OrderStatus.issue;
      default:
        return OrderStatus.completed;
    }
  } else if (value is String) {
    final v = value.toLowerCase();
    if (v.contains('deliver') || v.contains('ship')) {
      return OrderStatus.delivering;
    }
    if (v.contains('cancel')) return OrderStatus.cancelled;
    if (v.contains('issue') || v.contains('problem')) return OrderStatus.issue;
  }
  return OrderStatus.completed;
}

@freezed
abstract class OrderRecord with _$OrderRecord {
  const factory OrderRecord({
    required String orderId,
    required String date,
    required String items,
    required int amount,
    // ignore: invalid_annotation_target
    @JsonKey(fromJson: _parseOrderStatus)
    @Default(OrderStatus.completed)
    OrderStatus status,
    @Default(0) int rating,
    @Default('') String review,
  }) = _OrderRecord;

  factory OrderRecord.fromJson(Map<String, dynamic> json) =>
      _$OrderRecordFromJson(json);
}
