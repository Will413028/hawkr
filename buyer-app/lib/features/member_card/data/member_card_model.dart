import 'package:freezed_annotation/freezed_annotation.dart';

part 'member_card_model.freezed.dart';

@freezed
abstract class MemberCard with _$MemberCard {
  const factory MemberCard({
    required String sellerId,
    required String sellerName,

    /// 賣家 logo 的 objectKey（49 §6.1 成對契約中的 SSOT 那一半）。
    /// 顯示 URL 一律由 OssImage / buildImageUrl 於渲染期組出，不存完整 URL。
    @Default('') String sellerLogoKey,

    /// 賣家封面的 objectKey（同上）。
    @Default('') String sellerCoverKey,
    required String totalSpending,
  }) = _MemberCard;
}

@freezed
abstract class SpendingRecord with _$SpendingRecord {
  const factory SpendingRecord({
    required String orderDate,
    required String orderSn,
    required String amount,
    @Default('0') String refund,
    @Default(false) bool hasCoupon,
  }) = _SpendingRecord;
}
