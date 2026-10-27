// ignore_for_file: invalid_annotation_target

import 'package:freezed_annotation/freezed_annotation.dart';

part 'coupon_model.freezed.dart';
part 'coupon_model.g.dart';

/// 優惠券模板（對應 CouponTemplateResponse）
@freezed
abstract class Coupon with _$Coupon {
  const factory Coupon({
    /// templateId from API (used as primary key)
    required String templateId,

    /// instanceId for customer-specific coupon instances
    @Default('') String instanceId,
    @Default('') String title,

    /// CASH | PERCENT
    @Default('CASH') String type,
    @Default('') String status,
    String? discountAmount,
    String? discountPercent,

    /// 門檻金額
    String? thresholdAmount,
    String? maxDiscountAmount,
    String? startAt,
    String? endAt,
    String? issuedCount,
    String? totalIssueLimit,
    String? usageLimitPerAccount,
    String? issuerAccountId,
    String? issuerStoreId,
    String? issuerType,
    String? applicableStoreId,

    /// 賣家名稱 (from customer coupons API)
    String? sellerName,
    String? createdAt,
  }) = _Coupon;

  factory Coupon.fromJson(Map<String, dynamic> json) => _$CouponFromJson(json);
}

/// 優惠券模板詳情（對應 CouponTemplateDetailResponse）
///
/// 在列表的 [Coupon] 之外，額外提供已領取 / 已使用統計，供唯讀詳情頁顯示。
class CouponTemplateDetail {
  const CouponTemplateDetail({
    required this.template,
    this.reservedCount = 0,
    this.usedCount = 0,
  });

  /// 券模板本體
  final Coupon template;

  /// 已領取（保留）數量
  final int reservedCount;

  /// 已使用數量
  final int usedCount;
}

/// 客戶查詢結果（指定會員發券用）
@freezed
abstract class CustomerLookupResult with _$CustomerLookupResult {
  const factory CustomerLookupResult({
    required String uid,
    @Default('') String buyerName,
    @Default('') String phone,
    String? avatarUrl,
  }) = _CustomerLookupResult;

  factory CustomerLookupResult.fromJson(Map<String, dynamic> json) =>
      _$CustomerLookupResultFromJson(json);
}

/// 優惠券列表頁面狀態
@freezed
abstract class CouponListState with _$CouponListState {
  const factory CouponListState({
    @Default([]) List<Coupon> coupons,
    @Default(false) bool isLoading,
    String? error,
  }) = _CouponListState;
}
