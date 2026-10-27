import 'package:freezed_annotation/freezed_annotation.dart';

part 'order_summary.freezed.dart';

@freezed
abstract class OrderSummary with _$OrderSummary {
  const factory OrderSummary({
    required String id,
    @Default('') String orderSn,
    @Default('') String statusCode,
    @Default('') String title,
    @Default('') String subtitle,
    @Default('') String dateTime,
    @Default('') String productName,
    @Default('') String price,
    String? imageUrl,
    String? mainSkuId,
    @Default([]) List<OrderItem> items,
    @Default('') String merchantId,
    @Default('') String merchantName,
    @Default('') String merchantAvatar,
    @Default('') String receiverName,
    @Default('') String receiverPhone,
    @Default('') String receiverAddress,
    @Default(0.0) double shippingFee,
    @Default(0.0) double discount,
    @Default(0.0) double totalAmount,
    @Default('') String paymentMethod,
    @Default('') String paymentTime,
    @Default('') String paymentStatus,
    @Default('') String atmBankCode,
    @Default('') String atmVirtualAccount,
    @Default('') String atmExpireDate,
    @Default('') String totalActualAmount,
    @Default('') String shippingMethod,
    @Default('') String deliveryTemp,
    @Default('') String pickupStoreName,
    @Default('') String shippingStatus,
    @Default('') String statusTime,
    @Default('') String statusSummary,
  }) = _OrderSummary;
}

@freezed
abstract class OrderItem with _$OrderItem {
  const factory OrderItem({
    @Default('') String name,
    @Default(1) int quantity,
    @Default('') String price,
    String? imageUrl,
    String? skuId,
    String? spuId,
    String? merchantId,
  }) = _OrderItem;
}
