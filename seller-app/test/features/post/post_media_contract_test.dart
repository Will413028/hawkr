import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/config/media_image.dart';
import 'package:hawkr_seller/features/post/data/post_model.dart';
import 'package:flutter_test/flutter_test.dart';

/// 貼文媒體欄位契約測試（hawkr-website#430 A0 / B3）。
///
/// ## fixture 出處
///
/// fixture 形狀逐欄對齊 backend PR `#2235` 的
/// `gateway/internal/handler/post.go`（欄位名、`omitempty` 與否、型別）：
///
/// | fixture | 來源 API | 後端型別 |
/// |---|---|---|
/// | [_createResponse] / [_updateKeepResponse] / [_publicDetail] | `POST`、`PUT /app/sellers/posts{/id}`、`GET /public/posts/{id}` | `PostDetailData` |
/// | [_listRecord] / [_listRecordNoImage] | `GET /app/sellers/me/posts` 的 `records[]` | `PostListItemData` |
///
/// 值（objectKey / 顯示 URL / 貼文 id）沿用 2026-08-05 於 dev 擷取的真實樣本，
/// 只有欄位名隨 #2235 收斂改寫。
///
/// ## 契約重點
///
/// 1. **貼文圖只有複數 `imageKeys` / `imageUrls`**，且**無圖時回 `[]`**
///    （不再 `omitempty`）。單數 `imageKey` / `imageUrl` 已從貼文物件移除——貼文
///    物件上還留單數欄的只有巢狀關聯商品 `product`（`PostProductData`），那是
///    商品圖不是貼文圖。
/// 2. **`images` / `image` 在任何貼文端點都不存在**（gateway 全域無這兩個 response
///    欄位），曾列入候選純屬猜欄位。
/// 3. 只讀單數欄的解析，會讓「編輯存檔後」的貼文變成沒有圖片：畫面掉圖，而且再編輯
///    一次時保留判斷誤判成「本來就沒圖」→ `keepAssetId` 與 `newStagedToken` 同時送
///    空 → 後端把圖片從 DB 刪掉（#430 A0）。
/// 4. 其餘欄名一律 camelCase，**沒有 snake 別名**——下面的負向案例就是守這件事：
///    舊鍵不得再被認得，否則等於偷留一條雙讀 fallback。
const String _objectKey =
    'public/post/2685914682/20260804/a9b9cc70-e290-4a15-b2ad-774f028aa6ba.png';
const String _displayUrl =
    'https://test-media.example.com/i/original/$_objectKey';
const String _logoKey =
    'public/user_avatar/2685914682/20260729/2c7c6d22-28b9-4ad0-b802-f5818bbb632b.png';
const String _logoUrl = 'https://test-media.example.com/i/original/$_logoKey';

/// `POST /app/sellers/posts` 的 `data`（`PostDetailData`）。
const Map<String, dynamic> _createResponse = {
  'sellerName': '',
  'logoKey': '',
  'logoUrl': '',
  'content': 'A0 wire-shape 取樣貼文（收尾批，測完刪）',
  'imageKeys': [_objectKey],
  'imageUrls': [_displayUrl],
  'id': '7490479078313794603',
  'sellerId': '2685914682',
  'createdAt': 1785869379,
  'shareCount': 0,
  'viewCount': 0,
  'likeCount': 0,
  'commentCount': 0,
  'isLiked': false,
};

/// `PUT /app/sellers/posts/{id}`（`keepAssetId=keep`）的 `data`（`PostDetailData`）。
const Map<String, dynamic> _updateKeepResponse = {
  'sellerName': '',
  'logoKey': '',
  'logoUrl': '',
  'content': 'A0 wire-shape 取樣貼文（第一次編輯 keep）',
  'imageKeys': [_objectKey],
  'imageUrls': [_displayUrl],
  'id': '7490479078313794603',
  'sellerId': '2685914682',
  'createdAt': 1785869379,
  'shareCount': 0,
  'viewCount': 0,
  'likeCount': 0,
  'commentCount': 0,
  'isLiked': false,
};

/// `GET /app/sellers/me/posts` 的單筆 record（`PostListItemData`）。
/// 該貼文沒有 tag／未下架，故 `tags` / `hiddenAt` / `hiddenReason`（皆 `omitempty`）
/// 不出現；`isLiked` 與 `storeCode` 本來就不在這個型別上。
const Map<String, dynamic> _listRecord = {
  'imageKeys': [_objectKey],
  'imageUrls': [_displayUrl],
  'product': null,
  'sellerName': '陳記',
  'logoKey': _logoKey,
  'logoUrl': _logoUrl,
  'content': 'A0 wire-shape 取樣貼文（第一次編輯 keep）',
  'id': '7490479078313794603',
  'sellerId': '2685914682',
  'createdAt': 1785869379,
  'likeCount': 0,
  'commentCount': 0,
  'viewCount': 0,
  'shareCount': 0,
};

/// 沒有圖片的列表 record：複數欄位仍在，值為 `[]`（`emptyIfNil` /
/// `buildURLsAlways`，不再 `omitempty`）。
const Map<String, dynamic> _listRecordNoImage = {
  'imageKeys': <String>[],
  'imageUrls': <String>[],
  'product': null,
  'sellerName': '陳記',
  'logoKey': _logoKey,
  'logoUrl': _logoUrl,
  'content': 'A0 wire-shape 取樣貼文（清圖後）',
  'id': '7490479078313794603',
  'sellerId': '2685914682',
  'createdAt': 1785869379,
  'likeCount': 0,
  'commentCount': 0,
  'viewCount': 0,
  'shareCount': 0,
};

/// `GET /public/posts/{id}` 的 `data`（`PostDetailData`）。
const Map<String, dynamic> _publicDetail = {
  'sellerName': '陳記',
  'logoKey': _logoKey,
  'logoUrl': _logoUrl,
  'content': 'C6 契約驗證貼文（帶圖，模擬器 UI 驗證用）',
  'imageKeys': [
    'public/post/2685914682/20260804/8bddff6a-0c90-4cfb-ace0-8c414175ecd2.png',
  ],
  'imageUrls': [
    'https://test-media.example.com/i/original/'
        'public/post/2685914682/20260804/8bddff6a-0c90-4cfb-ace0-8c414175ecd2.png',
  ],
  'tags': ['契約驗證'],
  'id': '7490317712170336958',
  'sellerId': '2685914682',
  'createdAt': 1785830906,
  'shareCount': 0,
  'viewCount': 3,
  'likeCount': 0,
  'commentCount': 0,
  'isLiked': false,
};

void main() {
  setUpAll(() {
    if (!F.isInitialized) F.init(Flavor.dev);
  });

  group('貼文媒體欄位契約（#430 A0）', () {
    test('更新回應的複數欄位必須解析出圖片', () {
      // 這是整條資料損毀鏈的起點：讀單數欄的舊寫法在這裡會得到空清單。
      expect(_updateKeepResponse.containsKey('imageKey'), isFalse);
      expect(_updateKeepResponse.containsKey('imageUrl'), isFalse);

      final post = Post.fromJson(
        Map<String, dynamic>.from(_updateKeepResponse),
      );
      expect(post.imageKeys, [_objectKey]);
    });

    test('建立回應同形狀', () {
      expect(_createResponse.containsKey('imageKey'), isFalse);
      final post = Post.fromJson(Map<String, dynamic>.from(_createResponse));
      expect(post.imageKeys, [_objectKey]);
    });

    test('列表回應取 objectKey，不取顯示 URL', () {
      final post = Post.fromJson(Map<String, dynamic>.from(_listRecord));
      expect(post.imageKeys, [_objectKey]);
      expect(post.imageKeys.first.startsWith('http'), isFalse);
    });

    test('公開詳情回複數 objectKey，且沒有 images / image 欄位', () {
      // 讀取端曾把 `images` / `image` 列入候選，實打證明兩者不存在（#430 F2）。
      expect(_publicDetail.containsKey('images'), isFalse);
      expect(_publicDetail.containsKey('image'), isFalse);

      final post = Post.fromJson(Map<String, dynamic>.from(_publicDetail));
      expect(post.imageKeys, [
        'public/post/2685914682/20260804/8bddff6a-0c90-4cfb-ace0-8c414175ecd2.png',
      ]);
    });

    test('無圖時 imageKeys 為 []（欄位仍在），解析結果為空清單', () {
      expect(
        Post.fromJson(Map<String, dynamic>.from(_listRecordNoImage)).imageKeys,
        isEmpty,
      );
      expect(Post.fromJson({'id': '1', 'sellerId': '2'}).imageKeys, isEmpty);
    });

    test('只有單數 imageKey / imageUrl（舊契約）→ 一律解析為無圖，不得有 fallback', () {
      final legacyOnly = <String, dynamic>{
        'id': '7490479078313794603',
        'sellerId': '2685914682',
        'imageKey': _objectKey,
        'imageUrl': _displayUrl,
      };
      expect(Post.fromJson(legacyOnly).imageKeys, isEmpty);
    });
  });

  group('其餘貼文欄位只認 camelCase（不得留 snake fallback）', () {
    test('sellerId / createdAt 讀得到（camelCase）', () {
      final post = Post.fromJson(Map<String, dynamic>.from(_publicDetail));
      expect(post.sellerId, 2685914682);
      expect(post.createdAt, isNotNull);
    });

    test('只有舊 snake 鍵時，數值欄位一律落回預設值', () {
      final legacy = <String, dynamic>{
        'id': '1',
        'seller_id': '2685914682',
        'created_at': 1785830906,
        'like_count': 7,
        'comment_count': 5,
        'share_count': 3,
        'is_liked': true,
      };
      expect(postLikeCount(legacy), 0);
      expect(postCommentCount(legacy), 0);
    });

    test('camelCase 鍵才讀得到讚數 / 留言數', () {
      final wire = <String, dynamic>{'likeCount': 7, 'commentCount': 5};
      expect(postLikeCount(wire), 7);
      expect(postCommentCount(wire), 5);
    });
  });

  group('編輯貼文的保留判斷（#430 A0 資料損毀鏈的終點）', () {
    /// 與 `post_edit_screen.dart` 的判斷式同構：沒有新圖時，
    /// 有圖 → `keepAssetId='keep'`；沒圖 → 兩者皆空（後端據此清圖）。
    String keepAssetIdFor(Post post) => post.imageKeys.isNotEmpty ? 'keep' : '';

    test('第一次編輯後的貼文再編輯，仍然送 keep（不會把圖刪掉）', () {
      final afterFirstEdit = Post.fromJson(
        Map<String, dynamic>.from(_updateKeepResponse),
      );
      expect(keepAssetIdFor(afterFirstEdit), 'keep');
    });

    test('從列表進編輯也送 keep', () {
      expect(
        keepAssetIdFor(Post.fromJson(Map<String, dynamic>.from(_listRecord))),
        'keep',
      );
    });

    test('真的無圖的貼文才送空字串', () {
      expect(keepAssetIdFor(Post.fromJson({'id': '1', 'sellerId': '2'})), '');
    });
  });

  group('顯示 URL 一律走 buildImageUrl（#430 B3）', () {
    test('objectKey 組出 media 域名的 preset URL，不是把裸 key 接在 Gateway 域名後', () {
      final post = Post.fromJson(
        Map<String, dynamic>.from(_updateKeepResponse),
      );
      final url = buildImageUrl(post.imageKeys.first, ImagePreset.card);

      expect(url, '${Flavor.dev.mediaBaseHost}/i/card/$_objectKey');
      // 這條斷言存在的理由：已移除的那套 URL 組裝會把裸 key 直接接在 gatewayHost
      // 後面（必 404）。兩套入口並存時，貼文改讀 objectKey 就會整片黑圖。
      expect(url.startsWith('${Flavor.dev.gatewayHost}/$_objectKey'), isFalse);
    });

    test('後端回的 /i/original/ 顯示 URL 會被還原成 key 再組 preset', () {
      expect(
        buildImageUrl(_displayUrl, ImagePreset.card),
        '${Flavor.dev.mediaBaseHost}/i/card/$_objectKey',
      );
    });
  });
}
