import 'package:freezed_annotation/freezed_annotation.dart';

part 'wallet_model.freezed.dart';
part 'wallet_model.g.dart';

/// `GET /app/coupons` 的一筆券。
///
/// **權威來源**：gateway `templateInfoToMap` / `instanceInfoToMap`
/// （backend-payuni `gateway/internal/handler/coupon.go:1177-1248`）。
/// 兩者都只回 camelCase 單一鍵，沒有 snake 別名，也沒有下列欄位：
/// `id` / `couponId` / `name` / `discount` / `minAmount` / `expiredAt` /
/// `usedAt` / `ruleType` / `conditionType` / `discountRate`
/// —— 這些欄位過去被當成候選 key 讀，全部恆為空：`id` 恆 0（列表 ValueKey 因此
/// 全部相同）、`usedAt` 恆空（`isUsed` 恆 false）、規則說明只剩折抵金額一行。
/// 已全數移除，只留後端真的會吐的鍵。
@freezed
abstract class Coupon with _$Coupon {
  const factory Coupon({
    /// `instanceId`（已使用券）或 `templateId`（可用券）。
    required String id,
    required String name,
    int? status,
    String? createdAt,
    num? discountAmount,
    num? discountPercent,
    num? maxDiscountAmount,
    num? thresholdAmount,
    num? usageLimitPerAccount,
    String? startAt,
    String? endAt,
    String? sellerName,
    String? storeId,

    /// 商家 logo 來源：`logoKey`（裸 objectKey，SSOT）優先、`logoUrl` 退路。
    /// 顯示端一律帶 scene 過 `buildImageUrl`（dual-mode）。
    String? sellerAvatarUrl,
  }) = _Coupon;

  factory Coupon.fromJson(Map<String, dynamic> json) => _$CouponFromJson(json);

  factory Coupon.fromApi(Map<String, dynamic> json) {
    String stringValue(List<String> keys) {
      for (final key in keys) {
        final value = json[key]?.toString().trim() ?? '';
        if (value.isNotEmpty && value != 'null') return value;
      }
      return '';
    }

    num? numberValue(String key) {
      final value = json[key];
      if (value is num) return value;
      return num.tryParse(value?.toString() ?? '');
    }

    final title = stringValue(const ['title']);
    return Coupon(
      id: stringValue(const ['instanceId', 'templateId']),
      name: title.isEmpty ? '優惠券' : title,
      status: numberValue('status')?.toInt(),
      createdAt: stringValue(const ['createdAt']),
      discountAmount: numberValue('discountAmount'),
      discountPercent: numberValue('discountPercent'),
      maxDiscountAmount: numberValue('maxDiscountAmount'),
      thresholdAmount: numberValue('thresholdAmount'),
      usageLimitPerAccount: numberValue('usageLimitPerAccount'),
      startAt: stringValue(const ['startAt']),
      endAt: stringValue(const ['endAt']),
      sellerName: stringValue(const ['sellerName']),
      storeId: stringValue(const ['applicableStoreId']),
      sellerAvatarUrl: stringValue(const ['logoKey', 'logoUrl']),
    );
  }
}
