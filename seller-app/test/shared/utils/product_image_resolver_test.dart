import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/shared/utils/product_image_resolver.dart';
import 'package:flutter_test/flutter_test.dart';

// buildImageUrl 於 runtime 讀 F.appFlavor.mediaBaseHost；測試以 dev flavor
// 初始化（與 media_image_test 同慣例），故 base = 本機單體（沒有 MEDIA_BASE_HOST define）。
const String _mediaBase = 'http://localhost:8080';

// ── 以下 fixture 全部取自 2026-08-05 對 dev 的實打回應，未經改寫 ──────────
// seller 2685914682 / product 866542675787809994。
const String _key1 =
    'public/product/2685914682/20260719/cd527201-0b42-417d-8476-98d780c92a47.jpg';
const String _key2 =
    'public/product/2685914682/20260719/13b2d528-3679-4ab7-b548-99c023392f21.jpg';
const String _key3 =
    'public/product/2685914682/20260719/c42e4ff8-ce60-4551-be03-3aab7dbeb4a6.jpg';

/// `GET /api/v1/app/products?view=manage` 的 `data.records[0]`
/// 與 `GET /api/v1/app/products/{id}`、`GET /api/v1/public/products/{id}` 的
/// `data`：媒體欄位完全同形（gallery / imageKey / imageKeys / imageUrl / imageUrls）。
const Map<String, dynamic> _liveProductPayload = <String, dynamic>{
  'id': '866542675787809994',
  'name': '[範例] 快樂農夫 三地門 40年老欉 土芒果青',
  'gallery': <Map<String, dynamic>>[
    {
      'assetId': '866542678723412481',
      'objectKey': _key1,
      'url': '$_mediaBase/i/original/$_key1',
    },
    {
      'assetId': '866542681416156625',
      'objectKey': _key2,
      'url': '$_mediaBase/i/original/$_key2',
    },
    {
      'assetId': '866542684063039553',
      'objectKey': _key3,
      'url': '$_mediaBase/i/original/$_key3',
    },
  ],
  'imageKey': _key1,
  'imageUrl': '$_mediaBase/i/original/$_key1',
  'imageKeys': <String>[_key1, _key2, _key3],
  'imageUrls': <String>[
    '$_mediaBase/i/original/$_key1',
    '$_mediaBase/i/original/$_key2',
    '$_mediaBase/i/original/$_key3',
  ],
};

/// 貼文的關聯商品 `PostProductData`：列表端點放在單數 `product`、詳情端點放在
/// 陣列 `products`。**只有單數 `imageKey`／`imageUrl`**，沒有複數也沒有 gallery。
const Map<String, dynamic> _livePostProductPayload = <String, dynamic>{
  'name': '[範例] 快樂農夫 三地門 40年老欉 土芒果青',
  'imageKey': _key1,
  'imageUrl': '$_mediaBase/i/original/$_key1',
  'price': '320',
  'id': '866542675787809994',
};

/// `POST /api/v1/app/sellers/posts` 的回應：欄位都在，但值是空字串。
const Map<String, dynamic> _liveCreatedPostProductPayload = <String, dynamic>{
  'name': '',
  'imageKey': '',
  'imageUrl': '',
  'price': '',
  'id': '866542675787809994',
};

void main() {
  setUpAll(() {
    if (!F.isInitialized) F.init(Flavor.dev);
  });

  group('商品端點（實打 fixture）', () {
    test('imageKeys 依序組出全部顯示 URL', () {
      expect(extractProductImageUrls(_liveProductPayload), <String>[
        '$_mediaBase/i/original/$_key1',
        '$_mediaBase/i/original/$_key2',
        '$_mediaBase/i/original/$_key3',
      ]);
    });

    test('主圖 = imageKeys[0]，且與後端自己組的 imageUrl 一致', () {
      expect(
        extractPrimaryProductImageUrl(_liveProductPayload),
        _liveProductPayload['imageUrl'],
      );
      expect(extractPrimaryProductImageKey(_liveProductPayload), _key1);
    });

    test('單數 imageKey 與 imageKeys[0] 同值時不會重複輸出', () {
      expect(extractProductImageUrls(_liveProductPayload).length, 3);
    });

    test('拿掉 imageKeys（只剩衍生欄位）→ 仍由單數 imageKey 兜住主圖', () {
      final payload = Map<String, dynamic>.from(_liveProductPayload)
        ..remove('imageKeys');
      expect(extractProductImageUrls(payload), <String>[
        '$_mediaBase/i/original/$_key1',
      ]);
    });
  });

  group('貼文關聯商品（實打 fixture）', () {
    test('只有單數 imageKey 也解得出主圖', () {
      expect(
        extractPrimaryProductImageUrl(_livePostProductPayload),
        '$_mediaBase/i/original/$_key1',
      );
      expect(extractPrimaryProductImageKey(_livePostProductPayload), _key1);
    });

    test('建立回應的空字串欄位 → 判定為無圖（不是空字串 URL）', () {
      expect(extractProductImageUrls(_liveCreatedPostProductPayload), isEmpty);
      expect(
        extractPrimaryProductImageUrl(_liveCreatedPostProductPayload),
        isNull,
      );
      expect(
        extractPrimaryProductImageKey(_liveCreatedPostProductPayload),
        isNull,
      );
    });
  });

  group('猜欄位防復發（後端從未回過這些欄位名）', () {
    // 這一組是本檔存在的理由：24-key shotgun 時代，下面任何一個欄位都會被當成
    // 圖片來源。它們在任何商品/貼文端點都不存在（2026-08-05 實打逐支確認），
    // 所以現在必須**解不出任何圖**。若有人再把候選加回來，這裡會轉紅。
    const List<String> deadScalarFields = <String>[
      'image',
      'image_key',
      'image_url',
      'previewUrl',
      'preview_url',
      'thumbnail',
      'thumbnailUrl',
      'thumbnail_url',
      'mainImage',
      'main_image',
      'coverUrl',
      'cover_url',
      'pic',
      'objectKey',
      'object_key',
    ];
    const List<String> deadListFields = <String>[
      'image_keys',
      'image_urls',
      'images',
      'productImages',
      'product_images',
      '_loadedImages',
    ];

    for (final field in deadScalarFields) {
      test('$field 不再被當成圖片來源', () {
        expect(
          extractProductImageUrls(<String, dynamic>{field: _key1}),
          isEmpty,
          reason: '$field 不是後端會回的欄位',
        );
      });
    }

    for (final field in deadListFields) {
      test('$field（陣列）不再被當成圖片來源', () {
        expect(
          extractProductImageUrls(<String, dynamic>{
            field: <String>[_key1],
          }),
          isEmpty,
          reason: '$field 不是後端會回的欄位',
        );
      });
    }

    test('gallery 是 imageKeys 的衍生物，不再另外讀', () {
      // product-service 由 gallery 攤平出 image_keys；只要 imageKeys 在就夠了。
      // 單獨給 gallery（實務不會發生）解不出圖 —— 這正是「只有一個來源」的證明。
      expect(
        extractProductImageUrls(<String, dynamic>{
          'gallery': _liveProductPayload['gallery'],
        }),
        isEmpty,
      );
    });
  });

  group('objectKey 正規化', () {
    test('誤入的 i/{preset}/ 前綴會被剝掉，不產生 double-preset（#1659）', () {
      expect(
        extractPrimaryProductImageUrl(<String, dynamic>{
          'imageKey': 'i/original/$_key1',
        }),
        '$_mediaBase/i/original/$_key1',
      );
    });

    test('外部域名解不出 objectKey → 視同無圖', () {
      expect(
        extractProductImageUrls(<String, dynamic>{
          'imageKeys': <String>['https://lh3.googleusercontent.com/a/abc'],
        }),
        isEmpty,
      );
    });
  });
}
