import 'package:hawkr_seller/features/order/data/order_model.dart';
import 'package:flutter_test/flutter_test.dart';

/// 訂單欄位契約（`gateway/internal/handler/order.go` 的 `OrderDetailResponse`
/// 與 `OrderItemDetailResponse`）。
///
/// fixture 是 2026-08-05 對 dev 實打的逐字複製：
///   GET /api/v1/app/orders?view=seller&page=1&page_size=5   → data.records[0]
///   GET /api/v1/app/orders/O578806254?view=seller           → data
/// seller account 2685914682。
///
/// 為什麼要有這一份：這個 model 過去對幾乎每個欄位都並讀一大串候選鍵
/// （price 9 個、status 14 個、items 22 個＋啟發式嗅探、sellerName 16 個、
/// address/phone 各 12–14 個＋巢狀拼裝、金額三欄各 10–13 個＋巢狀再撈）。
/// 那些候選裡真正會命中的永遠只有一個，其餘全是死路——但它們讓「後端改欄名」
/// 從一個立刻看得見的空值，變成一個沒人知道何時會壞、壞了也查不出來的猜謎。
/// 每條 `isEmpty` 斷言都是在釘住「這個鍵不存在」這個事實。

/// `GET /app/orders?view=seller` → `data.records[0]`（實打逐字複製）。
Map<String, dynamic> _liveListOrder() => <String, dynamic>{
  'buyerNickname': '林小明',
  'status': '80',
  'totalActualAmount': '445',
  'itemCount': '1',
  'storeId': '2685914682',
  'sellerName': '陳記',
  'storeCode': '2685914682',
  'sellerCity': '桃園市',
  'sellerDistrict': '中壢區',
  'shippingMethod': '3001',
  'deliveryTemp': '3201',
  'buyerId': '2623900520',
  'statusTime': '2026-07-28T15:49:27.346782Z',
  'statusSummary': '訂單已取消。',
  'recipientName': 'Smoke Test',
  'createTime': '2026-07-28T15:49:06.07433Z',
  'shippingAddress': '台北市內湖區瑞光路1號',
  'orderNo': 'O578806254',
  'cancelByRole': 'BUYER',
  'items': <Map<String, dynamic>>[
    {
      'id': '869969726603969759',
      'productId': '866542675787809994',
      'productName': '[範例] 快樂農夫 三地門 40年老欉 土芒果青',
      'skuId': '314',
      'skuName': '土芒果青 3斤裝',
      'quantity': '1',
      'unitPrice': '320',
      'listPrice': '320',
    },
  ],
  'isReviewed': false,
  'canReview': false,
  'needRefundPage': false,
};

/// `GET /app/orders/{orderNo}?view=seller` → `data`（實打逐字複製）。
///
/// 與列表的差異（omitempty，空值＝整個鍵消失）：
/// 詳情多了金額細項與 ATM／付款欄位，item 多了
/// `actualPayAmount` / `refundStatus` / `shareDiscount`；
/// 列表獨有 `itemCount` / `statusSummary` / `statusTime`。
Map<String, dynamic> _liveDetailOrder() => <String, dynamic>{
  'orderNo': 'O578806254',
  'status': '80',
  'buyerNickname': '林小明',
  'buyerId': '2623900520',
  'sellerName': '陳記',
  'storeId': '2685914682',
  'createTime': '2026-07-28T15:49:06.07433Z',
  'totalActualAmount': '445',
  'totalOriginalAmount': '320',
  'shippingFee': '125',
  'discountAmount': '0',
  'shippingFeePayer': 'BUYER',
  'shippingMethod': '3001',
  'shippingArea': '1',
  'deliveryTemp': '3201',
  'recipientName': 'Smoke Test',
  'recipientPhone': '0912345678',
  'shippingAddress': '台北市內湖區瑞光路1號',
  'paymentMethod': '2004',
  'paymentMerId': 'HAWK2685914682',
  'cancelByRole': 'BUYER',
  'items': <Map<String, dynamic>>[
    {
      'id': '869969726603969759',
      'productId': '866542675787809994',
      'productName': '[範例] 快樂農夫 三地門 40年老欉 土芒果青',
      'skuId': '314',
      'skuName': '土芒果青 3斤裝',
      'quantity': '1',
      'unitPrice': '320',
      'listPrice': '320',
      'actualPayAmount': '320',
      'refundStatus': '0',
      'shareDiscount': '0',
    },
  ],
};

void main() {
  group('金額：wire 值 → 顯示值', () {
    test('詳情端點三個訂單層金額原樣帶入，不重算', () {
      final order = OrderSummary.fromJson(_liveDetailOrder());
      // shippingFee   wire '125' → 125
      expect(order.shippingFee, 125);
      // discountAmount wire '0'  → 0
      expect(order.discountAmount, 0);
      // totalActualAmount wire '445' → totalAmount 445
      expect(order.totalAmount, 445);
    });

    test('列表端點沒有 shippingFee / discountAmount，維持 0 不亂猜', () {
      final order = OrderSummary.fromJson(_liveListOrder());
      expect(order.shippingFee, 0);
      expect(order.discountAmount, 0);
      expect(order.totalAmount, 445); // totalActualAmount 兩支都有
    });

    test('item 金額：詳情取實付、列表退回成交單價', () {
      // 詳情 item：actualPayAmount '320' → price '320'
      final detail = OrderSummary.fromJson(_liveDetailOrder()).items.single;
      expect(detail.unitPrice, '320');
      expect(detail.listPrice, '320');
      expect(detail.actualPayAmount, '320');
      expect(detail.shareDiscount, '0');
      expect(detail.price, '320');
      expect(detail.quantity, 1);

      // 列表 item：沒有 actualPayAmount → price 退回 unitPrice '320'
      final list = OrderSummary.fromJson(_liveListOrder()).items.single;
      expect(list.actualPayAmount, isEmpty);
      expect(list.shareDiscount, isEmpty);
      expect(list.price, '320');
    });

    test('item 實付與單價不同時，price 取實付', () {
      final payload = _liveDetailOrder();
      (payload['items'] as List).first['actualPayAmount'] = '288';
      final item = OrderSummary.fromJson(payload).items.single;
      expect(item.unitPrice, '320');
      expect(item.price, '288');
    });

    test('訂單卡片單品價取首件 unitPrice', () {
      expect(OrderSummary.fromJson(_liveDetailOrder()).price, '320');
      expect(OrderSummary.fromJson(_liveListOrder()).price, '320');
    });

    test('缺 totalActualAmount 時顯示 0，不用 items 自行推算', () {
      // 舊行為會算 320*1 + 125 - 0 = 445 頂上去。那個數字沒有把分攤折扣與
      // 部分退款算進去，跟實際請款金額對不起來——顯示一個看似合理但錯的
      // 總額，比顯示 0 更難發現。
      final payload = _liveDetailOrder()..remove('totalActualAmount');
      expect(OrderSummary.fromJson(payload).totalAmount, 0);
    });

    test('已刪的金額候選鍵單獨給值都不會被讀到', () {
      const deadTotals = [
        'totalAmount',
        'payAmount',
        'paymentAmount',
        'serverPayAmount',
        'orderAmount',
        'amount',
        'total',
        'total_amount',
      ];
      for (final k in deadTotals) {
        final payload = _liveDetailOrder()..remove('totalActualAmount');
        payload[k] = '9999';
        expect(
          OrderSummary.fromJson(payload).totalAmount,
          0,
          reason: '$k 不是契約欄位，不該被當成訂單總額',
        );
      }

      const deadFees = [
        'freightAmount',
        'deliveryFee',
        'postage',
        'freight',
        'fare',
        'shipping',
        'logisticsFee',
        'shipping_fee',
      ];
      for (final k in deadFees) {
        final payload = _liveDetailOrder()..remove('shippingFee');
        payload[k] = '9999';
        expect(
          OrderSummary.fromJson(payload).shippingFee,
          0,
          reason: '$k 不是契約欄位，不該被當成運費',
        );
      }

      const deadDiscounts = [
        'couponAmount',
        'promotionAmount',
        'reduceAmount',
        'discountTotal',
        'discount_amount',
      ];
      for (final k in deadDiscounts) {
        final payload = _liveDetailOrder()..remove('discountAmount');
        payload[k] = '9999';
        expect(
          OrderSummary.fromJson(payload).discountAmount,
          0,
          reason: '$k 不是契約欄位，不該被當成折扣',
        );
      }

      const deadItemPrices = [
        'originalPrice',
        'realAmount',
        'price',
        'amount',
        'payAmount',
        'totalPrice',
        'payPrice',
      ];
      for (final k in deadItemPrices) {
        final payload = _liveDetailOrder();
        final item = (payload['items'] as List).first as Map<String, dynamic>;
        item
          ..remove('actualPayAmount')
          ..remove('unitPrice');
        item[k] = '9999';
        expect(
          OrderSummary.fromJson(payload).items.single.price,
          isEmpty,
          reason: '$k 不是契約欄位，不該被當成商品金額',
        );
      }
    });

    test('巢狀金額不再被撈（訂單回應是扁平的）', () {
      for (final wrapper in const [
        'order',
        'orderInfo',
        'data',
        'detail',
        'info',
      ]) {
        final payload = _liveDetailOrder()
          ..remove('shippingFee')
          ..remove('discountAmount');
        payload[wrapper] = {'shippingFee': '9999', 'discountAmount': '8888'};
        final order = OrderSummary.fromJson(payload);
        expect(order.shippingFee, 0, reason: '$wrapper 巢狀不該被撈');
        expect(order.discountAmount, 0, reason: '$wrapper 巢狀不該被撈');
      }
    });
  });

  group('明細只從 items 讀', () {
    test('兩支端點都解得出唯一一件商品', () {
      expect(OrderSummary.fromJson(_liveListOrder()).items, hasLength(1));
      expect(OrderSummary.fromJson(_liveDetailOrder()).items, hasLength(1));
    });

    test('已刪的 21 個明細候選鍵單獨給值都解不出明細', () {
      const dead = [
        'orderItemEntityList',
        'orderItems',
        'orderItemList',
        'orderItemVos',
        'list',
        'data',
        'itemList',
        'orderItemEntities',
        'orderItemDtos',
        'orderItemRspList',
        'orderItemResps',
        'products',
        'productList',
        'orderProducts',
        'goods',
        'goodsList',
        'details',
        'detailList',
        'orderDetails',
        'orderDetailList',
        'skuList',
        'skus',
      ];
      for (final k in dead) {
        final payload = _liveDetailOrder()..remove('items');
        payload[k] = [
          {'productName': 'X', 'unitPrice': '9999', 'quantity': '1'},
        ];
        expect(
          OrderSummary.fromJson(payload).items,
          isEmpty,
          reason: '$k 不是契約欄位，不該被當成訂單明細',
        );
      }
    });

    test('啟發式嗅探已移除：付款方式清單不會被當成商品明細', () {
      // 舊的 looksLikeItemMap 只要鍵名含 name/title 就算命中，而同一份回應裡
      // 的 availablePaymentMethods 正好有 name。只要 items 因 omitempty 消失，
      // 付款方式就會變成商品被算進金額。
      final payload = _liveDetailOrder()..remove('items');
      payload['availablePaymentMethods'] = [
        {'method': '2004', 'name': 'ATM 轉帳', 'enabled': true},
      ];
      expect(OrderSummary.fromJson(payload).items, isEmpty);
    });

    test('巢狀 order / orderInfo 的明細不再被撈', () {
      for (final wrapper in const ['order', 'orderInfo']) {
        final payload = _liveDetailOrder()..remove('items');
        payload[wrapper] = {
          'items': [
            {'productName': 'X', 'unitPrice': '9999', 'quantity': '1'},
          ],
        };
        expect(OrderSummary.fromJson(payload).items, isEmpty);
      }
    });
  });

  group('身分與聯絡欄位', () {
    test('實打回應解得出買家、賣家、收件資訊', () {
      final order = OrderSummary.fromJson(_liveDetailOrder());
      expect(order.buyerId, 2623900520);
      expect(order.buyerName, '林小明');
      expect(order.sellerName, '陳記');
      expect(order.recipientName, 'Smoke Test');
      expect(order.phone, '0912345678');
      expect(order.address, '台北市內湖區瑞光路1號');
      expect(order.shippingAddress, '台北市內湖區瑞光路1號');
    });

    test('phone 只讀 recipientPhone——買家手機不得溢出成收件人電話', () {
      final payload = _liveDetailOrder()..remove('recipientPhone');
      for (final k in const [
        'phone',
        'mobile',
        'tel',
        'buyerPhone',
        'receiverPhone',
        'consigneePhone',
        'contactPhone',
      ]) {
        final p = Map<String, dynamic>.from(payload)..[k] = '0900000000';
        expect(
          OrderSummary.fromJson(p).phone,
          isEmpty,
          reason: '$k 不是契約欄位；它們語意上也不是收件人電話',
        );
      }
    });

    test('sellerName 不再吃 storeName 等 15 個別名與巢狀物件', () {
      final payload = _liveDetailOrder()..remove('sellerName');
      for (final k in const [
        'merchant_name',
        'shopName',
        'storeName',
        'vendorName',
        'branchName',
        'marketName',
        'providerName',
        'farmerName',
      ]) {
        final p = Map<String, dynamic>.from(payload)..[k] = '假店名';
        expect(OrderSummary.fromJson(p).sellerName, isEmpty, reason: k);
      }
      for (final k in const [
        'merchant',
        'seller',
        'shop',
        'store',
        'provider',
        'vendor',
      ]) {
        final p = Map<String, dynamic>.from(payload)..[k] = {'name': '假店名'};
        expect(OrderSummary.fromJson(p).sellerName, isEmpty, reason: k);
      }
    });

    test('buyerId 不再吃 userId / customerId 等別名與巢狀 user 物件', () {
      final payload = _liveDetailOrder()..remove('buyerId');
      for (final k in const [
        'userId',
        'customerId',
        'memberId',
        'uid',
        'user_id',
        'buyer_id',
      ]) {
        final p = Map<String, dynamic>.from(payload)..[k] = '123';
        expect(OrderSummary.fromJson(p).buyerId, isNull, reason: k);
      }
      for (final k in const ['user', 'buyer', 'customer', 'member']) {
        final p = Map<String, dynamic>.from(payload)..[k] = {'id': '123'};
        expect(OrderSummary.fromJson(p).buyerId, isNull, reason: k);
      }
    });

    test('address 不再從巢狀 addressInfo 逐段拼裝', () {
      final payload = _liveDetailOrder()..remove('shippingAddress');
      payload['addressInfo'] = {
        'province': '台灣省',
        'city': '台北市',
        'district': '內湖區',
        'detail': '瑞光路1號',
      };
      expect(OrderSummary.fromJson(payload).address, isEmpty);
    });
  });

  group('狀態與識別欄位', () {
    test('實打回應解得出訂單編號與狀態', () {
      final order = OrderSummary.fromJson(_liveDetailOrder());
      expect(order.id, 'O578806254');
      expect(order.orderSn, 'O578806254');
      expect(order.statusCode, 80);
      expect(order.subtitle, '80');
      expect(order.dateTime, '2026-07-28T15:49:06.07433Z');
      expect(order.title, '[範例] 快樂農夫 三地門 40年老欉 土芒果青');
    });

    test('orderNo 是唯一的訂單編號來源', () {
      final payload = _liveDetailOrder()..remove('orderNo');
      for (final k in const ['orderSn', 'order_id', 'number', 'sn', 'code']) {
        final p = Map<String, dynamic>.from(payload)..[k] = 'FAKE123';
        // id 缺 orderNo 時退回 title（首件商品名），不會採用這些別名
        expect(OrderSummary.fromJson(p).id, isNot('FAKE123'), reason: k);
      }
    });

    test('status 不再吃 13 個描述性別名', () {
      final payload = _liveDetailOrder()..remove('status');
      for (final k in const [
        'reservationStatus',
        'statusDesc',
        'statusName',
        'orderStatusStr',
        'orderStatusName',
        'orderStatus',
        'state',
        'order_state',
        'progress',
      ]) {
        final p = Map<String, dynamic>.from(payload)..[k] = '已完成';
        expect(OrderSummary.fromJson(p).subtitle, isEmpty, reason: k);
      }
    });

    test('processingDeadline 是自動取消時間的唯一來源', () {
      final withDeadline = _liveDetailOrder()
        ..['processingDeadline'] = '2026-05-10T15:59:59Z';
      expect(
        OrderSummary.fromJson(withDeadline).autoCancelAt,
        '2026-05-10T15:59:59Z',
      );

      final withDeadKey = _liveDetailOrder()
        ..['autoCancelAt'] = '2026-05-10T15:59:59Z';
      expect(OrderSummary.fromJson(withDeadKey).autoCancelAt, isEmpty);
    });

    test('shippingStatus 同時餵 shippingStatus 與 shipmentStatus 兩個顯示欄位', () {
      final payload = _liveDetailOrder()..['shippingStatus'] = 'SHIPPED';
      final order = OrderSummary.fromJson(payload);
      expect(order.shippingStatus, 'SHIPPED');
      expect(order.shipmentStatus, 'SHIPPED');

      // `shipmentStatus` 這個鍵後端沒有，不該被讀
      final fake = _liveDetailOrder()..['shipmentStatus'] = 'FAKE';
      expect(OrderSummary.fromJson(fake).shipmentStatus, isEmpty);
    });
  });
}
