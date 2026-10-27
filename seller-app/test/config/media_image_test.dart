import 'package:cached_network_image/cached_network_image.dart';
import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/config/image_config.dart';
import 'package:hawkr_seller/config/media_image.dart';
import 'package:hawkr_seller/shared/widgets/optimized_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

/// media 域名基底跟隨當前 flavor（buildImageUrl 於 UI 渲染期 runtime 讀
/// F.appFlavor.mediaBaseHost）。測試以 dev flavor 初始化，故 base = 本機單體（沒有 MEDIA_BASE_HOST define）。
const String _mediaBase = 'http://localhost:8080';

void main() {
  setUpAll(() {
    if (!F.isInitialized) F.init(Flavor.dev);
  });

  group('貼文 GIF 顯示 preset', () {
    test('副檔名判斷不分大小寫，並忽略 query 與 fragment', () {
      expect(isAnimatedGifMedia('public/post/hero.gif'), isTrue);
      expect(isAnimatedGifMedia('public/post/hero.GIF?rev=2'), isTrue);
      expect(
        isAnimatedGifMedia(
          'https://media.example.com/i/original/public/post/hero.GiF#frame',
        ),
        isTrue,
      );
    });

    test('只認 pathname 最後的 .gif，不把偽副檔名或 query 當成 GIF', () {
      expect(isAnimatedGifMedia('public/post/hero.gif.webp'), isFalse);
      expect(isAnimatedGifMedia('public/post/hero.jpg?download=.gif'), isFalse);
      expect(isAnimatedGifMedia('public/post/gif/hero.webp'), isFalse);
      expect(isAnimatedGifMedia(''), isFalse);
    });

    test('GIF 卡面固定走 motion，靜態圖片保留呼叫端 preset', () {
      expect(
        postImagePresetForMedia('public/post/hero.gif'),
        ImagePreset.motion,
      );
      expect(
        postImagePresetForMedia(
          'public/post/hero.GIF?rev=2',
          staticPreset: ImagePreset.detail,
        ),
        ImagePreset.motion,
      );
      expect(postImagePresetForMedia('public/post/hero.jpg'), ImagePreset.card);
      expect(
        postImagePresetForMedia(
          'public/post/hero.webp',
          staticPreset: ImagePreset.detail,
        ),
        ImagePreset.detail,
      );
    });

    test('postImage scene 對 GIF 組 motion URL，靜態全螢幕圖片走 detail', () {
      expect(
        ImageConfig.urlFor(
          '$_mediaBase/i/original/public/post/hero.GIF?rev=2',
          ImageScene.postImage,
        ),
        '$_mediaBase/i/motion/public/post/hero.GIF',
      );
      expect(
        ImageConfig.urlFor('public/post/hero.webp', ImageScene.postImage),
        '$_mediaBase/i/detail/public/post/hero.webp',
      );
      expect(
        buildPostImageUrl('$_mediaBase/i/card/public/post/hero.gif'),
        '$_mediaBase/i/motion/public/post/hero.gif',
      );
    });

    testWidgets('GIF 使用 motion 時仍保留 postImage 的本地 decode cache 尺寸', (
      tester,
    ) async {
      late CachedNetworkImage cachedImage;

      await tester.pumpWidget(
        MaterialApp(
          home: Builder(
            builder: (context) {
              cachedImage =
                  const OptimizedNetworkImage(
                        imageUrl: 'public/post/hero.gif',
                        scene: ImageScene.postImage,
                      ).build(context)
                      as CachedNetworkImage;
              return const SizedBox.shrink();
            },
          ),
        ),
      );

      expect(cachedImage.imageUrl, '$_mediaBase/i/motion/public/post/hero.gif');
      expect(cachedImage.memCacheWidth, 900);
      expect(cachedImage.memCacheHeight, isNull);
    });
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

    test('Gateway 動態端點 /api/... passthrough', () {
      const dyn = 'https://test-api.example.com/api/v1/member/avatar/view/123';
      expect(buildImageUrl(dyn, ImagePreset.avatar), dyn);
    });

    test('外部 URL（Google 頭像）passthrough', () {
      const g = 'https://lh3.googleusercontent.com/a/abc';
      expect(buildImageUrl(g, ImagePreset.avatar), g);
    });
  });

  group('mediaObjectKey 還原乾淨裸 key（#1659 自癒）', () {
    test('裸 key 原樣、去前導斜線', () {
      expect(mediaObjectKey('public/x.png'), 'public/x.png');
      expect(mediaObjectKey('/public/x.png'), 'public/x.png');
    });

    test('完整內部 /i/{preset}/ URL 剝到裸 key', () {
      expect(
        mediaObjectKey('$_mediaBase/i/original/public/x.png'),
        'public/x.png',
      );
      expect(
        mediaObjectKey('$_mediaBase/i/avatar/public/x.png'),
        'public/x.png',
      );
    });

    test('被污染的裸 key（preset 前綴誤入）剝除', () {
      expect(mediaObjectKey('i/original/public/x.png'), 'public/x.png');
      expect(mediaObjectKey('/i/original/public/x.png'), 'public/x.png');
      expect(mediaObjectKey('i/motion/public/x.gif'), 'public/x.gif');
    });

    test('多重污染（double preset）反覆剝除', () {
      expect(
        mediaObjectKey('i/avatar/i/original/public/x.png'),
        'public/x.png',
      );
    });

    test('非 preset 的 i/ 前綴不誤剝', () {
      expect(mediaObjectKey('i/foo/public/x.png'), 'i/foo/public/x.png');
    });

    test('外部 / 動態端點回空字串', () {
      expect(mediaObjectKey('https://lh3.googleusercontent.com/a/abc'), '');
      expect(
        mediaObjectKey(
          'https://test-api.example.com/api/v1/member/avatar/view/1',
        ),
        '',
      );
    });
  });
}
