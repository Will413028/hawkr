import 'package:hawkr_api/images.dart' as api;
import 'package:hawkr_buyer/config/image_config.dart';
import 'package:hawkr_buyer/shared/widgets/app_cached_image.dart';
import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('API 的圖片只換掉 {preset}，不經過 buildImageUrl', (tester) async {
    final image = api.Image(
      uriTemplate: 'https://media.example/i/{preset}/public/banners/1.png',
    );
    await tester.pumpWidget(
      MaterialApp(
        home: AppCachedImage.api(image: image, scene: ImageScene.homeBanner),
      ),
    );
    final cached = tester.widget<CachedNetworkImage>(
      find.byType(CachedNetworkImage),
    );
    expect(
      cached.imageUrl,
      'https://media.example/i/banner/public/banners/1.png',
    );
  });

  testWidgets('沒有圖時顯示錯誤的佔位', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        home: AppCachedImage.api(
          image: null,
          scene: ImageScene.postImage,
          errorWidget: const Text('no image'),
        ),
      ),
    );
    expect(find.text('no image'), findsOneWidget);
    expect(find.byType(CachedNetworkImage), findsNothing);
  });
}
