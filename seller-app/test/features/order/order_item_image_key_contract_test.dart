import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/config/image_config.dart';
import 'package:hawkr_seller/config/media_image.dart';
import 'package:hawkr_seller/features/order/data/order_model.dart';
import 'package:flutter_test/flutter_test.dart';

// 訂單商品圖欄位契約（49 §6.1 objectKey 為 SSOT）。
//
// fixture 全部是 2026-08-05 對 dev 實打的逐字複製，未經改寫：
//   GET /api/v1/app/orders?view=seller&page=1&page_size=3
//   GET /api/v1/app/orders/O578806254?view=seller
// seller account 2685914682（store_id 同值）。

// buildImageUrl 於 runtime 讀 F.appFlavor.mediaBaseHost；測試以 dev flavor
// 初始化（與 product_image_resolver_test 同慣例），故 base = 本機單體（沒有 MEDIA_BASE_HOST define）。
const String _mediaBase = 'http://localhost:8080';

const String _itemKey =
    'public/product/2685914682/20260719/cd527201-0b42-417d-8476-98d780c92a47.jpg';
const String _logoKey =
    'public/user_avatar/2685914682/20260729/2c7c6d22-28b9-4ad0-b802-f5818bbb632b.png';
const String _buyerAvatarKey =
    'public/buyer_avatar/2623900520/20260719/008b97af-544d-4efb-8bc4-8f11e2b4e027.png';

/// `GET /app/orders?view=seller` → `data.records[0]`（實打逐字複製）。
Map<String, dynamic> _liveListOrder() => <String, dynamic>{
  'buyerNickname': '林小明',
  'status': '80',
  'totalActualAmount': '445',
  'itemCount': '1',
  'storeId': '2685914682',
  'sellerName': '陳記',
  'storeCode': '2685914682',
  'logoKey': _logoKey,
  'logoUrl': '$_mediaBase/i/original/$_logoKey',
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
  'avatarKey': _buyerAvatarKey,
  'avatarUrl': '$_mediaBase/i/original/$_buyerAvatarKey',
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
      'imageKey': _itemKey,
      'imageUrl': '$_mediaBase/i/original/$_itemKey',
    },
  ],
  'isReviewed': false,
  'canReview': false,
  'needRefundPage': false,
};

/// `GET /app/orders/{orderNo}?view=seller` → `data`（實打逐字複製；只留與本契約
/// 相關的欄位＋item 全鍵，金額/地址等其他欄位不影響圖片解析）。
Map<String, dynamic> _liveDetailOrder() => <String, dynamic>{
  'orderNo': 'O578806254',
  'status': '80',
  'buyerNickname': '林小明',
  'buyerId': '2623900520',
  'avatarKey': _buyerAvatarKey,
  'avatarUrl': '$_mediaBase/i/original/$_buyerAvatarKey',
  'logoKey': _logoKey,
  'logoUrl': '$_mediaBase/i/original/$_logoKey',
  'createTime': '2026-07-28T15:49:06.07433Z',
  'totalActualAmount': '445',
  'shippingFee': '125',
  'discountAmount': '0',
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
      'imageKey': _itemKey,
      'imageUrl': '$_mediaBase/i/original/$_itemKey',
    },
  ],
};

/// 2026-08-05 之前被並讀的 14 個候選裡，`imageKey` 之外的 13 個。
/// 其中 `imageUrl` 是後端真的會回的欄位，但它是 Gateway 由 key 組出的**衍生物**
/// （`gateway/internal/handler/order.go`：`ImageUrl: buildURL(mediaBase, item.ImageKey)`），
/// 不是 SSOT；其餘 12 個任何訂單端點都不回。
const List<String> _removedItemImageCandidates = <String>[
  'skuImage',
  'image',
  'img',
  'imageUrl',
  'pic',
  'picUrl',
  'spuPic',
  'spuImage',
  'skuImg',
  'spuImg',
  'goodsImg',
  'thumbnail',
  'thumb',
  'cover',
];

/// 訂單**根層**曾被當作「item 沒圖時的 fallback」並讀的 6 個鍵。
/// 訂單物件根本沒有商品圖欄位，這 6 個一個都不存在。
const List<String> _removedRootImageCandidates = <String>[
  'image',
  'img',
  'imageUrl',
  'pic',
  'picUrl',
  'cover',
];

void main() {
  setUpAll(() {
    if (!F.isInitialized) F.init(Flavor.dev);
  });

  group('OrderItem 商品圖只讀 imageKey（objectKey 為 SSOT）', () {
    test('列表端點：imageKey 解得出來，且值是裸 objectKey 不是 URL', () {
      final order = OrderSummary.fromJson(_liveListOrder());

      expect(order.items, hasLength(1));
      expect(order.items.single.imageKey, _itemKey);
      expect(order.items.single.imageKey, isNot(startsWith('http')));
    });

    test('詳情端點：同一個欄位、同一個值', () {
      final order = OrderSummary.fromJson(_liveDetailOrder());

      expect(order.items, hasLength(1));
      expect(order.items.single.imageKey, _itemKey);
    });

    test('顯示 URL 由 buildImageUrl 唯一入口組出 orderItem preset', () {
      final order = OrderSummary.fromJson(_liveListOrder());
      final key = order.items.single.imageKey;

      // UI 端把 objectKey 交給 OptimizedNetworkImage(scene: orderItem)，
      // 內部走 ImageConfig.urlFor → buildImageUrl。
      expect(
        ImageConfig.urlFor(key, ImageScene.orderItem),
        '$_mediaBase/i/thumb/$_itemKey',
      );
      expect(
        buildImageUrl(key, ImagePreset.original),
        '$_mediaBase/i/original/$_itemKey',
      );
    });

    test('imageKey 誤帶完整 URL 時還原成裸 key（不產生 double-preset）', () {
      final payload = _liveListOrder();
      (payload['items'] as List).first['imageKey'] =
          '$_mediaBase/i/original/$_itemKey';

      final order = OrderSummary.fromJson(payload);

      expect(order.items.single.imageKey, _itemKey);
      expect(
        ImageConfig.urlFor(order.items.single.imageKey, ImageScene.orderItem),
        '$_mediaBase/i/thumb/$_itemKey',
      );
    });

    test('toJson 送出的是 imageKey，round-trip 後不變', () {
      final item = OrderSummary.fromJson(_liveListOrder()).items.single;
      final json = item.toJson();

      expect(json['imageKey'], _itemKey);
      expect(json.containsKey('imageUrl'), isFalse);
      expect(OrderItem.fromMap(json).imageKey, _itemKey);
    });
  });

  group('猜欄位防復發', () {
    test('13 個已刪的 item 候選鍵逐一單獨給值 → 一律解不出圖', () {
      for (final candidate in _removedItemImageCandidates) {
        if (candidate == 'imageKey') continue;

        final payload = _liveListOrder();
        final item = (payload['items'] as List).first as Map<String, dynamic>;
        item.remove('imageKey');
        item.remove('imageUrl');
        item[candidate] = '$_mediaBase/i/original/$_itemKey';

        final order = OrderSummary.fromJson(payload);

        expect(
          order.items.single.imageKey,
          isEmpty,
          reason: '$candidate 不是訂單商品圖的來源，不得被解析',
        );
      }
    });

    test('訂單根層的 6 個候選鍵不得回填成 item 的圖（原 rootImg 死碼）', () {
      for (final candidate in _removedRootImageCandidates) {
        final payload = _liveListOrder();
        final item = (payload['items'] as List).first as Map<String, dynamic>;
        item.remove('imageKey');
        item.remove('imageUrl');
        payload[candidate] = '$_mediaBase/i/original/$_itemKey';

        final order = OrderSummary.fromJson(payload);

        expect(
          order.items.single.imageKey,
          isEmpty,
          reason: '訂單根層的 $candidate 不得被當成商品圖回填',
        );
      }
    });

    test('賣家 logo 與買家頭像不得溢出成商品圖', () {
      final payload = _liveListOrder();
      final item = (payload['items'] as List).first as Map<String, dynamic>;
      item.remove('imageKey');
      item.remove('imageUrl');

      final order = OrderSummary.fromJson(payload);

      // logoKey / avatar 都還在 payload 裡（實打回應本來就有），但它們是賣家
      // 商店 logo 與買家頭像，不是商品圖。
      expect(order.items.single.imageKey, isEmpty);
    });
  });
}
