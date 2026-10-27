import 'package:cached_network_image/cached_network_image.dart';
import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/config/image_config.dart';
import 'package:hawkr_buyer/shared/widgets/app_cached_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  setUpAll(() {
    if (!F.isInitialized) F.init(Flavor.dev);
  });

  testWidgets('scene cache decoding keeps the source aspect ratio', (
    tester,
  ) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: SizedBox(
          width: 330,
          height: 250,
          child: AppCachedImage(
            imageUrl: 'https://example.com/product.png',
            scene: ImageScene.productDetail,
            fit: BoxFit.contain,
          ),
        ),
      ),
    );

    final image = tester.widget<CachedNetworkImage>(
      find.byType(CachedNetworkImage),
    );
    expect(image.fit, BoxFit.contain);
    expect(image.memCacheWidth, 660);
    expect(
      image.memCacheHeight,
      isNull,
      reason: 'two-axis codec resizing distorts the source bitmap',
    );
  });

  testWidgets('explicit non-scene cache dimensions remain supported', (
    tester,
  ) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: AppCachedImage(
          imageUrl: 'https://example.com/legacy.png',
          memCacheWidth: 320,
          memCacheHeight: 240,
        ),
      ),
    );

    final image = tester.widget<CachedNetworkImage>(
      find.byType(CachedNetworkImage),
    );
    expect(image.memCacheWidth, 320);
    expect(image.memCacheHeight, 240);
  });

  testWidgets('quiet loading uses a static HeroUI-style placeholder', (
    tester,
  ) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: SizedBox(
          width: 120,
          height: 90,
          child: AppCachedImage(
            imageUrl: 'https://example.com/product.jpg',
            loadingStyle: AppImageLoadingStyle.quiet,
          ),
        ),
      ),
    );

    final finder = find.byType(CachedNetworkImage);
    final image = tester.widget<CachedNetworkImage>(finder);
    final placeholder = image.placeholder!(tester.element(finder), '');

    expect(image.placeholderFadeInDuration, Duration.zero);
    expect(image.fadeOutDuration, Duration.zero);
    expect(image.fadeInDuration, const Duration(milliseconds: 180));
    expect(placeholder, isA<ColoredBox>());
    expect(
      placeholder.key,
      const ValueKey('app-cached-image-quiet-placeholder'),
    );
  });
}
