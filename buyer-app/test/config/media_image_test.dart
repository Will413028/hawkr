import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/config/media_image.dart';

/// media 域名基底跟隨當前 flavor（buildImageUrl 於 UI 渲染期 runtime 讀
/// F.appFlavor.mediaBaseHost）。測試以 dev flavor 初始化、不帶 MEDIA_BASE_HOST，
/// 故 base = 本機單體。
const String _mediaBase = 'http://localhost:8080';

void main() {
  setUpAll(() {
    if (!F.isInitialized) F.init(Flavor.dev);
  });

  group('flavor → mediaBaseHost 對應（與 gatewayHost 同 flavor）', () {
    test('沒有 MEDIA_BASE_HOST define：dev → 本機單體、stage/prod → 無預設', () {
      expect(Flavor.dev.mediaBaseHost, _mediaBase);
      expect(Flavor.dev.mediaBaseHost, Flavor.dev.gatewayHost);
      expect(Flavor.stage.mediaBaseHost, '');
      expect(Flavor.prod.mediaBaseHost, '');
    });
  });

  group('buildImageUrl 共通', () {
    test('空字串回空字串', () {
      expect(buildImageUrl('', ImagePreset.thumb), '');
      expect(buildImageUrl('   ', ImagePreset.thumb), '');
    });
  });

  group('buildImageUrl Worker（base 跟隨 flavor）', () {
    test('裸 objectKey 組 /i/{preset}/{key}', () {
      expect(
        buildImageUrl('products/x.jpg', ImagePreset.thumb),
        '$_mediaBase/i/thumb/products/x.jpg',
      );
      // 前導斜線去除
      expect(
        buildImageUrl('/products/x.jpg', ImagePreset.thumb),
        '$_mediaBase/i/thumb/products/x.jpg',
      );
    });

    test('設定的媒體域名 URL 還原 key 後組 preset URL', () {
      expect(
        buildImageUrl('$_mediaBase/products/x.jpg', ImagePreset.card),
        '$_mediaBase/i/card/products/x.jpg',
      );
    });

    test('其他媒體域名不是設定的媒體來源，原樣 passthrough', () {
      const other = 'https://media.example.com/products/x.jpg';
      expect(buildImageUrl(other, ImagePreset.card), other);
    });

    // 媒體域名不做新舊相容（backend#2283）：已退役的 OSS CDN／aliyuncs 不再被當成
    // 媒體來源，一律視為外部網址原樣 passthrough，不會被靜默還原成 key。
    test('已退役的舊儲存域名一律 passthrough，不還原 key', () {
      const retiredCdn = 'https://cdn.example.com/products/x.jpg';
      expect(buildImageUrl(retiredCdn, ImagePreset.card), retiredCdn);
      const oss =
          'https://example-media.oss-ap-southeast-1.aliyuncs.com/a/b.jpg';
      expect(buildImageUrl(oss, ImagePreset.detail), oss);
    });

    test('Gateway 檔案代理 /api/v1/files/{key} 還原 key', () {
      expect(
        buildImageUrl(
          'https://test-api.example.com/api/v1/files/posts/p.jpg',
          ImagePreset.card,
        ),
        '$_mediaBase/i/card/posts/p.jpg',
      );
    });

    test('已是 Worker /i/original/{key} 可重組 preset', () {
      expect(
        buildImageUrl(
          '$_mediaBase/i/original/products/x.jpg',
          ImagePreset.thumb,
        ),
        '$_mediaBase/i/thumb/products/x.jpg',
      );
    });

    test('Gateway 動態端點 /api/... passthrough（不產生無效 /i/ URL）', () {
      const dyn = 'https://test-api.example.com/api/v1/member/avatar/view/123';
      expect(buildImageUrl(dyn, ImagePreset.avatar), dyn);
    });

    test('外部 URL（Google 頭像）passthrough', () {
      const g = 'https://lh3.googleusercontent.com/a/abc';
      expect(buildImageUrl(g, ImagePreset.avatar), g);
    });
  });

  group('isInlineImageData：只認 data: scheme，不用長度猜（#89）', () {
    // 實打回應裡的 objectKey（2026-08-05 GET /app/chat/conversations 逐字取出）。
    const objectKey =
        'public/user_avatar/2685914682/20260729/'
        '2c7c6d22-28b9-4ad0-b802-f5818bbb632b.png';

    test('裸 objectKey 不是 inline 圖片——它比所有舊門檻都長', () {
      // 這一行就是舊寫法會炸的原因：objectKey 長度落在 60~120 之間，
      // 門檻 60 的兩處（wallet / merchant 頭像）已經被撞穿，
      // 門檻 120 的（category landing）只差 41 個字元。
      expect(objectKey.length, greaterThan(60));
      expect(objectKey.length, lessThan(120));
      expect(isInlineImageData(objectKey), isFalse);
    });

    test('把長度當判準的舊寫法會把 objectKey 誤判成 base64', () {
      bool legacyGuess(String v, int threshold) =>
          v.startsWith('data:image/') ||
          (!v.startsWith('http') && v.length > threshold);
      expect(legacyGuess(objectKey, 60), isTrue, reason: '這就是修掉的 bug');
      expect(isInlineImageData(objectKey), isFalse, reason: '新判準不受長度影響');
    });

    test('data: URI 是唯一為真的形態', () {
      expect(isInlineImageData('data:image/png;base64,iVBORw0KGgo='), isTrue);
      expect(isInlineImageData('  data:image/jpeg;base64,/9j/4AA'), isTrue);
    });

    test('完整 URL、空字串、純 base64 字串都不是 inline 圖片', () {
      expect(isInlineImageData('$_mediaBase/i/avatar/$objectKey'), isFalse);
      expect(isInlineImageData(''), isFalse);
      // 沒有 data: 前綴的裸 base64 不再被當成圖片資料——後端不會這樣回。
      expect(
        isInlineImageData('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJ'),
        isFalse,
      );
    });

    test('非 inline 的值交給 buildImageUrl 就組得出顯示 URL', () {
      expect(
        buildImageUrl(objectKey, ImagePreset.avatar),
        '$_mediaBase/i/avatar/$objectKey',
      );
    });
  });
}
