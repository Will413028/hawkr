import 'package:freezed_annotation/freezed_annotation.dart';

import '../../../config/media_image.dart';

part 'order_model.freezed.dart';

@freezed
abstract class OrderItem with _$OrderItem {
  const OrderItem._();

  const factory OrderItem({
    required String name,
    required String price,
    // PayUni #758: 新欄位（成交單價 / 原價快照 / 折抵 / 實付）
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

    // 實付優先、退回成交單價：詳情端點兩者都有（實付已扣分攤折扣），
    // 列表端點只有 unitPrice。原本並讀的 9 個候選裡，其餘 7 個
    // （originalPrice / realAmount / price / amount / payAmount /
    //  totalPrice / payPrice）任何訂單端點都不回。
    final price = OrderItem._str(j['actualPayAmount'] ?? j['unitPrice']);
    final qty = OrderItem._int(j['quantity']);
    // 商品圖只有**扁平成對的兩個欄位**：`imageKey`（objectKey，SSOT）與
    // `imageUrl`（Gateway 由 key 組出的顯示 URL）。2026-08-05 對 dev 實打
    // `/app/orders?view=seller` 與 `/app/orders/{orderNo}?view=seller` 逐鍵確認，
    // 兩支的 item 鍵集完全是
    // {id, productId, productName, skuId, skuName, quantity, unitPrice, listPrice,
    //  imageKey, imageUrl(, actualPayAmount, refundStatus, shareDiscount)}。
    // 原本並讀的 14 個候選裡，只有 `imageUrl` 會命中——而它是衍生物不是 SSOT
    // （`gateway/internal/handler/order.go` 的 `ImageUrl: buildURL(mediaBase, item.ImageKey)`），
    // 其餘 13 個任何端點都不回。這裡只讀 key，顯示 URL 交給 [buildImageUrl]
    // 唯一入口（49 §6.1 / §8.6）。**新增候選前必須先實打端點確認。**
    final imageKey = mediaObjectKey(OrderItem._str(j['imageKey']));
    final skuId = OrderItem._str(j['skuId']);
    // 商品 ID 的契約欄名是 `productId`；`spuId`/`goodsId`/`spu_id`/`spu` 都不回。
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

    // `status` 是數字碼字串（CodeMeta OrderStatus 的 Value，例 '80'）。
    // 原本並讀的 14 個候選裡只有它存在；`statusDesc`/`orderStatus`/`state`
    // 這類描述性欄名後端從來沒有過。
    final status = OrderSummary._str(j['status'] ?? '');

    String itemName = '';
    String itemPrice = '';
    Map<String, dynamic>? firstItem;
    List<OrderItem> items = const [];

    // 明細只在 `items` 底下。原本並讀 22 個候選鍵、再往巢狀
    // `order`/`orderInfo` 找、最後還有一段「掃過每個 List 欄位，用鍵名長得
    // 像不像商品來猜」的啟發式嗅探——那段是真的會咬人的：`looksLikeItemMap`
    // 只要鍵名含 name/title 就算命中，而同一份回應裡的
    // `availablePaymentMethods`（付款方式，有 name）正好符合。只要哪天
    // `items` 因為 omitempty 消失，付款方式清單就會被當成商品明細算進金額。
    final rawItems = j['items'];
    final List<dynamic>? foundList = rawItems is List && rawItems.isNotEmpty
        ? rawItems
        : null;

    if (foundList != null && foundList.isNotEmpty) {
      final mapped = <OrderItem>[];
      // 這裡原本有一段「item 沒圖就退回訂單根層的圖」的 fallback，並讀
      // image / img / imageUrl / pic / picUrl / cover 六個鍵。**訂單物件根本沒有
      // 商品圖欄位**：2026-08-05 實打 dev 兩支端點，訂單層唯一的圖片類欄位是
      // `logoKey`/`logoUrl`（賣家商店 logo）與 `avatarKey`/`avatarUrl`（買家頭像），
      // 那六個鍵一個都不存在 → 整段 fallback 從第一天就是死碼。更糟的是它一旦
      // 「復活」（有人在後端加了同名欄位）會把商店 logo 當商品圖貼上去。已物理移除。
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
      // 卡片標題旁的單品價＝成交單價。原本並讀的 7 個候選裡只有 `unitPrice`
      // 存在（注意它刻意不含 `actualPayAmount`——那是扣掉分攤折扣後的實付，
      // 屬於 item 層顯示，不是這裡要的單價）。
      itemPrice = OrderSummary._str(firstItem['unitPrice']);
    }

    // 訂單根層沒有 `productName`（只有 item 有），所以標題一律取首件商品名。
    final String productName = itemName;
    // 這個 model 只餵兩種回應：`/app/orders`（列表）與 `/app/orders/{orderNo}`
    // （詳情），兩支都只吐 `createTime`（2026-08-01 對 dev 實測）。原本的六鍵
    // 並讀是防禦性解析的外皮，實際效果是讓「其中一個寫錯」完全看不出來。
    final dateTimeStr = OrderSummary._str(j['createTime']);
    final title = productName.isNotEmpty
        ? productName
        : (id.isNotEmpty ? 'Order $id' : 'Order');
    // 訂單根層沒有 `payAmount`/`amount`（訂單總額的契約欄名是
    // `totalActualAmount`，見下方 totalAmount），所以這裡一直都是 itemPrice。
    final finalPrice = itemPrice;

    // 買家 ID 的契約欄名是 `buyerId`（字串）。原本並讀 9 個鍵再往
    // `user`/`buyer`/`customer`/`member` 巢狀物件找，以及往
    // `order`/`orderInfo` 兩層再試一次——訂單回應是扁平的，那些全不存在。
    final int? buyerId = int.tryParse(OrderSummary._str(j['buyerId']));

    // 賣家店名的契約欄名是 `sellerName`。原本並讀 16 個鍵＋6 個巢狀物件，
    // 其中 `storeName` 這種還會在別處被當成取件門市，語意本來就不同。
    final String sellerName = OrderSummary._str(j['sellerName']);

    // 收件地址的契約欄名是 `shippingAddress`（Gateway 已組成完整字串）。
    // 原本並讀 12 個鍵，再從 `addressInfo`/`receiver`/`consignee` 巢狀物件
    // 逐段拼 province+city+district+detail——後端從來不回巢狀地址物件，
    // 那段拼裝從第一天就沒執行過。
    final String address = OrderSummary._str(j['shippingAddress']);

    // 兩支來源（`/app/orders`、`/app/orders/{orderNo}`）都只吐 `buyerNickname`
    // 與扁平成對的 `avatarKey`/`avatarUrl`，沒有巢狀 buyer 物件（2026-08-01 對
    // dev 實測；欄名於 2026-08-05 由 `buyerAvatarKey`/`buyerAvatarUrl` 收斂）。
    // 原本各串了十幾個候選鍵＋巢狀 fallback——那不只是死碼：
    // `recipientName`/`consigneeName` 這類是**收件人**不是買家，一旦
    // `buyerNickname` 缺值就會把收件人姓名當成買家顯示。
    //
    // 這裡刻意只讀 `avatarUrl` 不讀 `avatarKey`：下游 `BuyerAvatarWidget` 以
    // `startsWith('http')` 判斷有無圖，餵裸 objectKey 會靜默退成預設頭像。
    final String buyerName = OrderSummary._str(j['buyerNickname']);
    final String buyerAvatar = OrderSummary._str(j['avatarUrl']);

    // 電話的契約欄名是 `recipientPhone`——它是**收件人**電話，不是買家帳號
    // 綁定的手機。原本並讀的 13 個候選裡有 `buyerPhone`/`mobile` 這類，
    // 一旦後端真的加上去，顯示的人就會從收件人變成買家而沒人察覺。
    final String phone = OrderSummary._str(j['recipientPhone']);

    num toNum(dynamic v) {
      if (v == null) return 0;
      if (v is num) return v;
      return num.tryParse(v.toString()) ?? 0;
    }

    // ── 金額三欄 ────────────────────────────────────────────────
    // 契約欄名分別是 `shippingFee` / `discountAmount` / `totalActualAmount`
    // （皆為數字字串，僅詳情端點回；列表端點只有 totalActualAmount）。
    // 原本三欄各並讀 10~13 個候選鍵，前兩欄還會在值為 0 時往
    // `order`/`orderInfo`/`data`/`detail`/`info` 五個巢狀物件再撈一次——
    // 訂單回應是扁平的，那些巢狀鍵一個都不存在。
    //
    // ⚠️ 「值為 0 就再找找看」這個模式本身就是錯的：運費 0（免運）與折扣 0
    // （沒用券）都是完全正常的值，把它當成「還沒找到」會讓一個真實的 0
    // 去觸發後續的猜測路徑。
    final num shippingFee = toNum(j['shippingFee']);
    final num discountAmount = toNum(j['discountAmount']);

    // 總額以後端為準，不自己算。原本在 totalAmount==0 時會用
    // items 單價×數量 + 運費 − 折扣 重算一份——那個數字沒有把分攤折扣
    // （item 的 shareDiscount）、部分退款（refundStatus）算進去，
    // 跟實際請款金額對不起來。顯示一個「看起來合理但其實不對」的總額，
    // 比顯示 0 更難發現。
    final num totalAmount = toNum(j['totalActualAmount']);

    // 以下全部是扁平 camelCase 單鍵。原本每欄都掛一個 snake_case 兄弟
    // （`recipient_name`、`shipping_method`…），後端從來沒有回過 snake_case——
    // Gateway 的 json tag 一律 camelCase。
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

    // 出貨狀態的契約欄名只有 `shippingStatus` 一個；`shipmentStatus` 這個鍵
    // 後端沒有，兩個欄位保留是因為 UI 兩處分別在用。
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
      // 兩支來源都只吐 `orderNo`（2026-08-01 對 dev 實測）；`orderSn`/`order_no`
      // 是死鍵，留著只會讓下一個人以為它們可能命中。
      orderSn: OrderSummary._str(j['orderNo'] ?? id),
      statusCode: (j['status'] is int)
          ? j['status'] as int
          : int.tryParse(j['status']?.toString() ?? ''),
      title: title,
      // 訂單根層沒有 `remark`/`note`/`description`（備註的契約欄名是
      // `orderRemark`，而它不是狀態文案），所以 subtitle 一律是狀態碼。
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
      // 自動取消時間的契約欄名是 `processingDeadline`（賣家處理期限）。
      // `autoCancelAt` 這個鍵後端沒有。
      autoCancelAt: OrderSummary._str(j['processingDeadline']),
      cancelReason: cancelReason,
      cancelRemark: cancelRemark,
      cancelByRole: cancelByRole,
    );
  }
}
