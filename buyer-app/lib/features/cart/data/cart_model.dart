import 'package:freezed_annotation/freezed_annotation.dart';

part 'cart_model.freezed.dart';
part 'cart_model.g.dart';

@freezed
abstract class CartItem with _$CartItem {
  const factory CartItem({
    required int id,
    @JsonKey(name: 'skuId') required int skuId,
    required int count,
    required int price,
    // 加入購物車時的原價快照（刪除線用），缺欄位時為 null。
    @JsonKey(name: 'listPrice') int? listPrice,
    String? title,
    String? image,
    @JsonKey(name: 'sellerId') int? sellerId,
    @JsonKey(name: 'merchantName') String? merchantName,
    @JsonKey(name: 'specName') String? specName,
    bool? check,
    @JsonKey(name: 'userId') String? userId,
    @JsonKey(name: 'userKey') String? userKey,
    // 每個商品列的可購買狀態與庫存由後端回傳。
    @JsonKey(name: 'stockQty') String? stockQty,
    @JsonKey(name: 'purchasable') @Default(true) bool purchasable,
    @JsonKey(name: 'unavailableReason') String? unavailableReason,
  }) = _CartItem;

  factory CartItem.fromJson(Map<String, dynamic> json) =>
      _$CartItemFromJson(json);
}

@freezed
abstract class CartSummary with _$CartSummary {
  const factory CartSummary({
    @Default([]) List<CartItem> items,
    @JsonKey(name: 'totalAmount') @Default(0) int totalAmount,
  }) = _CartSummary;

  factory CartSummary.fromJson(Map<String, dynamic> json) =>
      _$CartSummaryFromJson(json);
}
