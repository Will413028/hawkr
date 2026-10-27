import 'package:freezed_annotation/freezed_annotation.dart';

part 'order_model.freezed.dart';
part 'order_model.g.dart';

/// 訂單品項的顯示圖來源。
///
/// gateway `OrderItemDetailResponse` / `OrderItemViewResponse` 成對輸出
/// `imageKey`＋`imageUrl`（49 §6.1），兩者都帶 omitempty，無圖時整組不出現。
/// 這裡優先取裸 key —— 顯示端 `AppCachedImage(scene)` 走 buildImageUrl
/// dual-mode，裸 key 與完整 URL 都吃得下。
Map<String, dynamic> _normalizeOrderItemKeys(Map<String, dynamic> json) {
  final m = Map<String, dynamic>.from(json);
  if ((m['imageUrl'] ?? '').toString().trim().isEmpty) {
    final key = m['imageKey']?.toString().trim() ?? '';
    if (key.isNotEmpty) m['imageUrl'] = key;
  }
  return m;
}

/// 訂單 JSON 的顯示前處理。
///
/// 欄名本身不需要正規化 —— gateway 的 `OrderSummaryResponse` /
/// `OrderDetailResponse` 全欄位都是 camelCase。這裡只做兩件形狀上的事：
/// 把巢狀 `review` 平鋪成頂層 `reviewRating`／`reviewContent`（DTO 沒有這兩個
/// 頂層欄位），以及把評分統一成 int。
Map<String, dynamic> _normalizeOrderKeys(Map<String, dynamic> json) {
  final m = Map<String, dynamic>.from(json);
  // 後端把評分放在 review 物件裡（review.rating / review.content）— 平鋪到頂層
  final review = m['review'];
  if (review is Map) {
    if (!m.containsKey('reviewRating') && review['rating'] != null) {
      m['reviewRating'] = review['rating'];
    }
    if (!m.containsKey('reviewContent') && review['content'] != null) {
      m['reviewContent'] = review['content'];
    }
  }
  // reviewRating 後端可能回字串「4」「4.0」— freezed 預設不會自動轉 int，先處理。
  final r = m['reviewRating'];
  if (r is String) {
    final n = num.tryParse(r);
    m['reviewRating'] = n?.round() ?? 0;
  } else if (r is num) {
    m['reviewRating'] = r.round();
  }
  return m;
}

@freezed
abstract class OrderItem with _$OrderItem {
  const factory OrderItem({
    @Default('') String id,
    @Default('') String productId,
    @Default('') String productName,
    @Default('') String skuId,
    @Default('') String skuName,
    @Default('') String quantity,
    // PayUni #758: 新欄位（成交單價 / 原價快照）
    @Default('') String unitPrice,
    @Default('') String listPrice,

    /// Deprecated: 舊欄位 fallback；新訂單請用 unitPrice。
    @Default('') String originalPrice,
    @Default('') String actualPayAmount,
    @Default('') String refundStatus,
    @Default('') String shareDiscount,
    @Default('') String imageUrl,
  }) = _OrderItem;

  factory OrderItem.fromJson(Map<String, dynamic> json) =>
      _$OrderItemFromJson(_normalizeOrderItemKeys(json));
}

/// OrderItem 顯示用 fallback：unitPrice -> originalPrice
extension OrderItemPriceX on OrderItem {
  /// 成交單價（新訂單）：unitPrice 優先，否則 fallback originalPrice。
  String get effectiveUnitPrice {
    if (unitPrice.isNotEmpty) return unitPrice;
    return originalPrice;
  }

  /// 刪除線原價：listPrice 優先；缺值時 fallback unitPrice / originalPrice。
  /// 配合 [shouldStrikeListPrice] 判斷是否顯示。
  String get effectiveListPrice {
    if (listPrice.isNotEmpty) return listPrice;
    return effectiveUnitPrice;
  }

  /// 是否顯示原價刪除線。
  bool get shouldStrikeListPrice {
    final lp = num.tryParse(effectiveListPrice);
    final up = num.tryParse(effectiveUnitPrice);
    if (lp == null || up == null) return false;
    return lp > up;
  }

  /// 分攤折抵（>0 才顯示）。
  num get shareDiscountValue => num.tryParse(shareDiscount) ?? 0;
}

@freezed
abstract class RefundRecord with _$RefundRecord {
  const factory RefundRecord({
    @Default('') String requestNo,
    @Default('') String orderItemId,
    @Default('') String status,
    @Default('') String refundAmount,
    @Default('') String reason,
    @Default('') String createTime,
  }) = _RefundRecord;

  factory RefundRecord.fromJson(Map<String, dynamic> json) =>
      _$RefundRecordFromJson(json);
}

@freezed
abstract class Order with _$Order {
  const factory Order({
    @Default(0) int id,
    @JsonKey(name: 'orderNo') @Default('') String orderSn,
    @Default('') String status,
    @Default('') String totalOriginalAmount,
    @Default('') String totalActualAmount,
    @Default('') String createTime,
    @Default('') String payTime,
    @Default('') String shipTime,
    @Default('') String shipmentNo,
    @Default('') String transactionId,
    @Default('') String buyerId,
    @Default('') String deliveryTemp,
    @Default('') String shippingMethod,
    @Default('') String shippingFee,
    @Default('') String shippingFeePayer,
    @Default('') String shippingArea,
    @Default('') String couponId,
    @Default('') String discountAmount,
    @Default('') String shippingStatus,
    @Default('') String statusTime,
    @Default('') String statusSummary,

    /// B-124：送達後自動完成時間（ISO 8601 UTC，status=40 時有值）
    /// 政策：送達後 3 天自動完成（從 7 天縮短，2026-04-25 後生效）
    @Default('') String autoCompleteAt,

    /// B-132：買家付款後賣家須於此時間前處理物流，逾時後端自動取消。
    @Default('') String processingDeadline,
    @Default('') String labelUrl,
    @Default('') String recipientName,
    @Default('') String recipientPhone,
    @Default('') String shippingAddress,
    @Default('') String shippingNote,
    @Default('') String orderRemark,
    @Default('') String partnerId,
    @Default('') String odno,
    @Default('') String validationNo,
    @Default('') String paymentMethod,
    @Default('') String atmBankCode,
    @Default('') String atmVirtualAccount,
    @Default('') String atmExpireDate,
    // 付款期限（ISO 8601 UTC，僅 status='0' 時有值；超過後訂單會被系統自動取消，cancelReason 9101）
    @Default('') String paymentDeadline,
    @Default('') String itemCount,
    @Default('') String sellerName,
    @JsonKey(name: 'logoUrl') @Default('') String sellerAvatarUrl,
    @Default('') String sellerCity,
    @Default('') String sellerDistrict,
    @Default('') String pickupStoreName,
    @Default('') String storeId,
    @Default(false) bool isReviewed,

    /// B-101：後端回傳目前是否仍可評價。預設 true 以保留舊 API 行為。
    @Default(true) bool canReview,

    /// B-101：評價截止時間（ISO 8601 UTC，status=50 且可評價時有值）。
    @Default('') String reviewDeadline,

    /// 買家給此訂單的評分（1~5），未評分為 0。
    /// 後端可能放在 `reviewRating` 或 `review.rating`，由 fromJson 正規化。
    @Default(0) int reviewRating,

    /// 買家評論內容（reviewContent / review.content），未填為空字串。
    @Default('') String reviewContent,
    @Default('') String cancelReason,
    @Default('') String cancelRemark,
    @Default('') String cancelByRole,

    /// status=80 時若為 true，表示買家需手動填寫 ATM 退款銀行資訊。
    /// 觸發 POST /app/orders/{order_no}/refund_page 取得 PayUni WebView URL。
    @Default(false) bool needRefundPage,

    /// 賣家出貨上傳的商品照片（status>=20 後才會有值）
    @Default(<String>[]) List<String> shipmentPhotoUrls,

    /// 買家確認收貨上傳的照片（status>=40 後才會有值）
    @Default(<String>[]) List<String> receiptPhotoUrls,

    /// 再次付款可用付款方式（依 totalActualAmount 算；僅 pending 時非空）。§3.4.1
    /// 每筆 = {paymentType, eligible, ineligibleReason}。
    @Default(<Map<String, dynamic>>[])
    List<Map<String, dynamic>> availablePaymentMethods,
    @Default([]) List<OrderItem> items,
    @Default([]) List<RefundRecord> refundHistory,
  }) = _Order;

  factory Order.fromJson(Map<String, dynamic> json) =>
      _$OrderFromJson(_normalizeOrderKeys(json));
}

/// B-124：autoCompleteAt 倒數顯示 helper
/// 政策（2026-04-25 後）：送達後 3 天自動完成（從 7 天縮短）
extension OrderAutoCompleteX on Order {
  /// 距離自動完成還有多少時間（DateTime.now() < autoCompleteAt 時為正值）
  Duration? get autoCompleteRemaining {
    if (autoCompleteAt.isEmpty) return null;
    final dt = DateTime.tryParse(autoCompleteAt);
    if (dt == null) return null;
    return dt.toLocal().difference(DateTime.now());
  }

  /// 顯示用倒數文字：3 天後自動完成 / 5 小時後自動完成 / 已自動完成
  String get autoCompleteCountdownLabel {
    final r = autoCompleteRemaining;
    if (r == null) return '';
    if (r.isNegative) return '已自動完成';
    if (r.inDays >= 1) return '${r.inDays} 天後自動完成';
    if (r.inHours >= 1) return '${r.inHours} 小時後自動完成';
    if (r.inMinutes >= 1) return '${r.inMinutes} 分鐘後自動完成';
    return '即將自動完成';
  }
}

/// B-185：paymentDeadline 倒數顯示 helper
/// 後端 commit cddcfa8（2026-05-22）— 僅 status='0' 時有值；
/// 逾期由 scheduler 自動取消（cancelReason 9101）。
extension OrderPaymentDeadlineX on Order {
  /// 距離付款逾時還有多少時間（正值 = 還沒逾時）
  Duration? get paymentRemaining {
    if (paymentDeadline.isEmpty) return null;
    final dt = DateTime.tryParse(paymentDeadline);
    if (dt == null) return null;
    return dt.toLocal().difference(DateTime.now());
  }

  /// 顯示用倒數文字：
  /// 3 天後逾時取消 / 6 小時後逾時取消 / 23 分鐘後逾時取消 / 付款已逾時
  String get paymentCountdownLabel {
    final r = paymentRemaining;
    if (r == null) return '';
    if (r.isNegative) return '付款已逾時';
    if (r.inHours >= 24) return '${r.inDays} 天後逾時取消';
    if (r.inHours >= 1) return '${r.inHours} 小時後逾時取消';
    if (r.inMinutes >= 1) return '${r.inMinutes} 分鐘後逾時取消';
    return '即將逾時';
  }

  /// 剩餘 < 1 小時 → UI 用警示色強調
  bool get paymentDeadlineUrgent {
    final r = paymentRemaining;
    return r != null && !r.isNegative && r.inMinutes < 60;
  }

  /// 是否已逾時（true 時應引導使用者「再次付款」或顯示已逾期）
  bool get paymentDeadlineExpired {
    final r = paymentRemaining;
    return r != null && r.isNegative;
  }
}
