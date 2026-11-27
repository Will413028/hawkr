import 'package:freezed_annotation/freezed_annotation.dart';

import '../../../config/media_image.dart';

part 'order_model.freezed.dart';

@freezed
abstract class OrderItem with _$OrderItem {
  const OrderItem._();

  const factory OrderItem({
    required String name,
    required String price,
    @Default('') String unitPrice,
    @Default('') String listPrice,
    @Default('') String shareDiscount,
    @Default('') String actualPayAmount,
    @Default(1) int quantity,

    /// 商品圖 **objectKey**（49 §6.1 的 SSOT）。顯示 URL 一律交給
    /// [OptimizedNetworkImage] / [buildImageUrl] 這個唯一組裝入口，模型層不存 URL。
    @Default('') String imageKey,
    @Default('') String skuId,
    @Default('') String spuId,
  }) = _OrderItem;

  static String _str(dynamic v) => v == null ? '' : v.toString();
  static int _int(dynamic v) {
    if (v == null) return 0;
    if (v is int) return v;
    return int.tryParse(v.toString()) ?? 0;
  }

  /// item 的欄位全集（`gateway/internal/handler/order.go` 的
  /// `OrderItemDetailResponse`，2026-08-05 對 dev 兩支端點實打逐鍵確認）：
  /// `id / productId / productName / skuId / skuName / quantity /
  ///  unitPrice / listPrice / actualPayAmount / refundStatus / shareDiscount /
  ///  imageKey / imageUrl`。
  ///
  /// 全部欄位都是 `omitempty`，所以**空值等於整個鍵消失**——列表端點的 item 就
  /// 沒有 `actualPayAmount` / `refundStatus` / `shareDiscount`（詳情才有）。
  /// 這是 `price` 需要兩段而其他欄位只要一段的唯一原因。
  ///
  /// 新增候選鍵前必須先實打端點確認。多候選並讀不會讓錯誤浮現，
  /// 只會讓「後端改欄名」從一個立刻看得見的空值變成沒人知道何時壞的猜謎。
  factory OrderItem.fromMap(Map<String, dynamic> j) {
    String name = OrderItem._str(j['productName']);

    final unitPriceStr = OrderItem._str(j['unitPrice']);
    final listPriceStr = OrderItem._str(j['listPrice']);
    final shareDiscountStr = OrderItem._str(j['shareDiscount']);
    final actualPayAmountStr = OrderItem._str(j['actualPayAmount']);

    final price = OrderItem._str(j['actualPayAmount'] ?? j['unitPrice']);
    final qty = OrderItem._int(j['quantity']);
    final imageKey = mediaObjectKey(OrderItem._str(j['imageKey']));
    final skuId = OrderItem._str(j['skuId']);
    final spuId = OrderItem._str(j['productId']);

    return OrderItem(
      name: name,
      price: price,
      unitPrice: unitPriceStr,
      listPrice: listPriceStr,
      shareDiscount: shareDiscountStr,
      actualPayAmount: actualPayAmountStr,
      quantity: qty > 0 ? qty : 1,
      imageKey: imageKey,
      skuId: skuId,
      spuId: spuId,
    );
  }

  Map<String, dynamic> toJson() => {
    'name': name,
    'price': price,
    'unitPrice': unitPrice,
    'listPrice': listPrice,
    'shareDiscount': shareDiscount,
    'actualPayAmount': actualPayAmount,
    'quantity': quantity,
    'imageKey': imageKey,
    'skuId': skuId,
    'spuId': spuId,
  };

  /// PayUni #758: 成交單價（unitPrice -> originalPrice fallback -> price）
  String get effectiveUnitPrice {
    if (unitPrice.isNotEmpty) return unitPrice;
    return price;
  }

  /// PayUni #758: 原價刪除線值
  String get effectiveListPrice {
    if (listPrice.isNotEmpty) return listPrice;
    return effectiveUnitPrice;
  }

  bool get shouldStrikeListPrice {
    final lp = num.tryParse(effectiveListPrice);
    final up = num.tryParse(effectiveUnitPrice);
    if (lp == null || up == null) return false;
    return lp > up;
  }

  num get shareDiscountValue => num.tryParse(shareDiscount) ?? 0;
}

@freezed
abstract class OrderSummary with _$OrderSummary {
  const OrderSummary._();

  const factory OrderSummary({
    required String id,
    @Default('') String orderSn,
    int? statusCode,
    required String title,
    required String subtitle,
    required String dateTime,
    required String productName,
    required String price,
    @Default('') String sellerName,
    @Default([]) List<OrderItem> items,
    int? buyerId,
    @Default('') String address,
    @Default('') String buyerName,
    @Default(0) num shippingFee,
    @Default(0) num discountAmount,
    @Default(0) num totalAmount,
    @Default('') String phone,
    @Default('') String recipientName,
    @Default('') String shippingAddress,
    @Default('') String shippingMethod,
    @Default('') String buyerAvatar,
    @Default('') String pickupStoreName,
    @Default('') String statusSummary,
    @Default('') String shippingStatus,
    @Default('') String labelUrl,
    @Default('') String odno,
    @Default('') String validationNo,
    @Default('') String shipmentStatus,
    @Default([]) List<String> shipmentPhotoUrls,
    @Default([]) List<String> receiptPhotoUrls,
    @Default('') String labelExpireAt,
    @Default('') String shipStatusDesc,
    @Default('') String autoCancelAt,
    @Default('') String cancelReason,
    @Default('') String cancelRemark,
    @Default('') String cancelByRole,
  }) = _OrderSummary;

  static String _str(dynamic v) => v == null ? '' : v.toString();

  /// 只餵兩種回應：`GET /app/orders?view=seller`（列表 `data.records[]`）與
  /// `GET /app/orders/{orderNo}?view=seller`（詳情 `data`）。
  /// 兩支的欄位全集＝`gateway/internal/handler/order.go` 的
  /// `OrderDetailResponse`，2026-08-05 對 dev 實打逐鍵確認。
  ///
  /// 路由 `extra` 走的是另一條路（`core/router/route_payload.dart` 的
  /// `orderSummaryFromRoutePayload`），不經過這裡——所以這個方法只需要認得
  /// 後端契約，不必兼容自己 `toJson` 出去的形狀。
  static OrderSummary fromJson(Map<String, dynamic> j) {
    final id = OrderSummary._str(j['orderNo'] ?? '');

    final status = OrderSummary._str(j['status'] ?? '');

    String itemName = '';
    String itemPrice = '';
    Map<String, dynamic>? firstItem;
    List<OrderItem> items = const [];

    final rawItems = j['items'];
    final List<dynamic>? foundList = rawItems is List && rawItems.isNotEmpty
        ? rawItems
        : null;

    if (foundList != null && foundList.isNotEmpty) {
      final mapped = <OrderItem>[];
      for (final e in foundList) {
        if (e is Map) {
          mapped.add(OrderItem.fromMap(e.cast<String, dynamic>()));
        }
      }
      items = mapped;
      final fi = foundList.first;
      if (fi is Map) {
        firstItem = fi.cast<String, dynamic>();
      }
    }

    if (firstItem != null) {
      itemName = OrderSummary._str(firstItem['productName']);
      itemPrice = OrderSummary._str(firstItem['unitPrice']);
    }

    final String productName = itemName;
    final dateTimeStr = OrderSummary._str(j['createTime']);
    final title = productName.isNotEmpty
        ? productName
        : (id.isNotEmpty ? 'Order $id' : 'Order');
    final finalPrice = itemPrice;

    final int? buyerId = int.tryParse(OrderSummary._str(j['buyerId']));

    final String sellerName = OrderSummary._str(j['sellerName']);

    final String address = OrderSummary._str(j['shippingAddress']);

    final String buyerName = OrderSummary._str(j['buyerNickname']);
    final String buyerAvatar = OrderSummary._str(j['avatarUrl']);

    final String phone = OrderSummary._str(j['recipientPhone']);

    num toNum(dynamic v) {
      if (v == null) return 0;
      if (v is num) return v;
      return num.tryParse(v.toString()) ?? 0;
    }

    final num shippingFee = toNum(j['shippingFee']);
    final num discountAmount = toNum(j['discountAmount']);

    final num totalAmount = toNum(j['totalActualAmount']);

    final recipientName = OrderSummary._str(j['recipientName']);
    final shippingMethod = OrderSummary._str(j['shippingMethod']);
    final pickupStoreName = OrderSummary._str(j['pickupStoreName']);
    final statusSummary = OrderSummary._str(j['statusSummary']);
    final cancelReason = OrderSummary._str(j['cancelReason']);
    final cancelRemark = OrderSummary._str(j['cancelRemark']);
    final cancelByRole = OrderSummary._str(j['cancelByRole']);
    final labelUrl = OrderSummary._str(j['labelUrl']);
    final odno = OrderSummary._str(j['odno']);
    final validationNo = OrderSummary._str(j['validationNo']);

    final shippingStatus = OrderSummary._str(j['shippingStatus']);

    List<String> shipmentPhotoUrls = const [];
    final rawPhotos = j['shipmentPhotoUrls'];
    if (rawPhotos is List && rawPhotos.isNotEmpty) {
      shipmentPhotoUrls = rawPhotos
          .whereType<String>()
          .where((s) => s.isNotEmpty)
          .toList();
    }

    List<String> receiptPhotoUrls = const [];
    final rawReceiptPhotos = j['receiptPhotoUrls'];
    if (rawReceiptPhotos is List && rawReceiptPhotos.isNotEmpty) {
      receiptPhotoUrls = rawReceiptPhotos
          .whereType<String>()
          .where((s) => s.isNotEmpty)
          .toList();
    }

    return OrderSummary(
      id: id.isNotEmpty ? id : title,
      orderSn: OrderSummary._str(j['orderNo'] ?? id),
      statusCode: (j['status'] is int)
          ? j['status'] as int
          : int.tryParse(j['status']?.toString() ?? ''),
      title: title,
      subtitle: status,
      dateTime: dateTimeStr,
      productName: productName,
      price: finalPrice,
      sellerName: sellerName,
      items: items,
      buyerId: buyerId,
      address: address,
      buyerName: buyerName.isNotEmpty ? buyerName : recipientName,
      shippingFee: shippingFee,
      discountAmount: discountAmount,
      totalAmount: totalAmount,
      phone: phone,
      recipientName: recipientName,
      shippingAddress: address,
      shippingMethod: shippingMethod,
      pickupStoreName: pickupStoreName,
      buyerAvatar: buyerAvatar,
      statusSummary: statusSummary,
      shippingStatus: shippingStatus,
      labelUrl: labelUrl,
      odno: odno,
      validationNo: validationNo,
      shipmentStatus: shippingStatus,
      shipmentPhotoUrls: shipmentPhotoUrls,
      receiptPhotoUrls: receiptPhotoUrls,
      labelExpireAt: OrderSummary._str(j['labelExpireAt']),
      shipStatusDesc: OrderSummary._str(j['shippingStatusDesc']),
      autoCancelAt: OrderSummary._str(j['processingDeadline']),
      cancelReason: cancelReason,
      cancelRemark: cancelRemark,
      cancelByRole: cancelByRole,
    );
  }
}
