import 'package:hawkr_api/gen/hawkr/content/v1alpha1/content.pb.dart' as api;
import 'package:hawkr_api/images.dart' as api_image;
import 'package:hawkr_buyer/core/layout/app_media_query_policy.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/features/home/presentation/home_widget/home_banner_section.dart';
import 'package:hawkr_buyer/features/home/presentation/home_widget/home_member_card_section.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  for (final width in <double>[320, 390, 768]) {
    for (final textScale in <double>[1, 1.4, 2]) {
      testWidgets(
        'home chrome is compact at ${width.toInt()}px / ${textScale}x text',
        (tester) async {
          await _setViewport(tester, width);

          await tester.pumpWidget(
            _testApp(
              textScale: textScale,
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  HomeMemberCardSection(
                    userName: 'Cora Chen Long Name',
                    couponCount: 123,
                    memberCardCount: 5,
                    merchantAvatars: const [],
                    onCouponTap: () {},
                    onMemberCardTap: () {},
                  ),
                ],
              ),
            ),
          );
          await tester.pump();

          final cardContext = tester.element(
            find.byKey(const ValueKey('home-member-card')),
          );
          expect(
            MediaQuery.textScalerOf(cardContext).scale(10),
            closeTo(10 * textScale.clamp(0, 1.4), 0.0001),
          );
          expect(tester.takeException(), isNull);
          expect(
            tester
                .getSize(find.byKey(const ValueKey('home-member-card')))
                .height,
            96,
          );
        },
      );

      testWidgets(
        'home banner keeps 16:9 at ${width.toInt()}px / ${textScale}x text',
        (tester) async {
          await _setViewport(tester, width);

          await tester.pumpWidget(
            _testApp(
              textScale: textScale,
              child: HomeBannerSection(banners: [api.Banner()]),
            ),
          );
          await tester.pump();

          expect(tester.takeException(), isNull);
          final height = tester
              .getSize(find.byKey(const ValueKey('home-banner-viewport')))
              .height;
          // banner 卡片固定 16:9，高度由卡片寬度換算，不隨字級變動。
          final cardWidth = width - HkBreakpoints.pageGutter(width) * 2;
          expect(height, closeTo(cardWidth * 9 / 16, 0.5));

          await tester.pumpWidget(const SizedBox.shrink());
        },
      );
    }
  }

  testWidgets('home discovery controls meet mobile accessibility guidelines', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    await _setViewport(tester, 390);

    await tester.pumpWidget(
      _testApp(
        textScale: 1.4,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            HomeMemberCardSection(
              userName: 'Cora',
              couponCount: 2,
              memberCardCount: 3,
              merchantAvatars: const [],
              onCouponTap: () {},
              onMemberCardTap: () {},
            ),
            HomeBannerSection(
              banners: [
                api.Banner(
                  title: '本週產地活動',
                  popupImage: api_image.Image(
                    uriTemplate:
                        'https://example.test/{preset}/weekly-popup.jpg',
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
    await tester.pump();

    await expectLater(tester, meetsGuideline(androidTapTargetGuideline));
    await expectLater(tester, meetsGuideline(iOSTapTargetGuideline));
    await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));
    await tester.pumpWidget(const SizedBox.shrink());
    semantics.dispose();
  });
}

Future<void> _setViewport(WidgetTester tester, double width) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = Size(width, 844);
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
}

Widget _testApp({required double textScale, required Widget child}) {
  return MaterialApp(
    builder: (context, routedChild) => MediaQuery(
      data: AppMediaQueryPolicy.apply(
        MediaQuery.of(
          context,
        ).copyWith(textScaler: TextScaler.linear(textScale)),
      ),
      child: routedChild!,
    ),
    home: Scaffold(body: SingleChildScrollView(child: child)),
  );
}
