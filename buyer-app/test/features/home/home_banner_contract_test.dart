import 'package:hawkr_api/gen/hawkr/content/v1alpha1/content.pb.dart' as api;
import 'package:hawkr_api/images.dart' as api_image;
import 'package:hawkr_buyer/features/home/presentation/home_widget/banner_popup_dialog.dart';
import 'package:hawkr_buyer/features/home/presentation/home_widget/home_banner_section.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

/// 首頁橫幅吃 ContentPublicService 的 `Banner`（ListLiveBanners）。
Future<void> _pumpBanners(WidgetTester tester, List<api.Banner> banners) async {
  await tester.pumpWidget(
    MaterialApp(
      home: Scaffold(
        body: SizedBox(width: 390, child: HomeBannerSection(banners: banners)),
      ),
    ),
  );
  await tester.pump();
}

api_image.Image _image(String name) =>
    api_image.Image(uriTemplate: 'https://media.example/i/{preset}/$name');

void main() {
  testWidgets('標題是橫幅的無障礙名稱，沒有標題時用按鈕文字', (tester) async {
    await _pumpBanners(tester, [
      api.Banner(id: '1', title: '週末特賣', previewImage: _image('a.jpg')),
      api.Banner(id: '2', buttonText: '立即搶購', previewImage: _image('b.jpg')),
    ]);

    expect(find.bySemanticsLabel('活動橫幅：週末特賣'), findsOneWidget);
  });

  testWidgets('有彈窗大圖才能點開', (tester) async {
    await _pumpBanners(tester, [
      api.Banner(id: '1', previewImage: _image('preview.jpg')),
    ]);
    expect(find.bySemanticsLabel('查看活動橫幅'), findsNothing);

    await _pumpBanners(tester, [
      api.Banner(
        id: '1',
        previewImage: _image('preview.jpg'),
        popupImage: _image('popup.jpg'),
      ),
    ]);
    expect(find.bySemanticsLabel('查看活動橫幅'), findsOneWidget);
  });

  group('BannerAction.fromProto', () {
    test('每一種點擊行為', () {
      final popup = BannerAction.fromProto(
        api.BannerAction(popup: api.BannerAction_Popup()),
      );
      expect(popup.type, 'popup');
      expect(popup.hasCta, isFalse);

      final url = BannerAction.fromProto(
        api.BannerAction(url: 'https://example.test/a'),
        buttonText: '看活動',
      );
      expect(
        (url.type, url.url, url.label),
        ('url', 'https://example.test/a', '看活動'),
      );

      final product = BannerAction.fromProto(
        api.BannerAction(productId: '123'),
      );
      expect(
        (product.type, product.id, product.label),
        ('product', '123', '立即購買'),
      );

      final post = BannerAction.fromProto(api.BannerAction(postId: '456'));
      expect((post.type, post.id, post.hasCta), ('post', '456', true));
    });

    test('沒有設定點擊行為時沒有按鈕', () {
      final none = BannerAction.fromProto(api.BannerAction());
      expect(none.type, isNull);
      expect(none.hasCta, isFalse);
    });
  });
}
