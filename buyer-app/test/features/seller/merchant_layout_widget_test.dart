import 'package:hawkr_buyer/core/layout/app_media_query_policy.dart';
import 'package:hawkr_buyer/core/layout/app_text_size.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/features/seller/presentation/widgets/merchant_about_section.dart';
import 'package:hawkr_buyer/features/seller/presentation/widgets/merchant_header.dart';
import 'package:hawkr_buyer/features/seller/presentation/widgets/merchant_layout.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  const longDescription =
      '我們每天從產地挑選當令食材，依照不同溫層妥善包裝並安排配送。'
      '希望讓每一位顧客都能清楚知道食材從哪裡來，也能安心享用每一餐。'
      '所有商品上架前都會再次確認品質、保存方式與建議料理方法。';
  const avatarData =
      'data:image/png;base64,'
      'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=';

  void useViewport(WidgetTester tester, Size viewport) {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = viewport;
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);
  }

  Future<void> pumpHeader(
    WidgetTester tester, {
    required Size viewport,
    double textScale = 1,
    AppTextSize textSize = AppTextSize.standard,
    TextScaler? textScaler,
    String merchantAward = '年度優選店舖',
    String merchantLocation = '新北市中和區',
    double rating = 4.9,
    int ratingCount = 128,
    String description = longDescription,
  }) async {
    useViewport(tester, viewport);
    const statusBar = 59.0;

    await tester.pumpWidget(
      MaterialApp(
        home: MediaQuery(
          data: AppMediaQueryPolicy.apply(
            MediaQueryData(
              size: viewport,
              padding: const EdgeInsets.only(top: statusBar),
              viewPadding: const EdgeInsets.only(top: statusBar),
              textScaler: textScaler ?? TextScaler.linear(textScale),
            ),
            textSize: textSize,
          ),
          child: Material(
            child: Align(
              alignment: Alignment.topCenter,
              child: MerchantHeader(
                merchantName: '範例生鮮友善農產合作店舖',
                merchantAddress: '台南市永康區中華路一段 123 號',
                merchantLocation: merchantLocation,
                merchantDescription: description,
                merchantAward: merchantAward,
                rating: rating,
                ratingCount: ratingCount,
                followerCount: 12500,
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pump();
  }

  testWidgets('storefront header fits compact phone at large text', (
    tester,
  ) async {
    await pumpHeader(tester, viewport: const Size(320, 568), textScale: 1.3);

    expect(tester.takeException(), isNull);
    expect(tester.getSize(find.bySemanticsLabel('返回')), const Size(48, 48));
    expect(tester.getSize(find.bySemanticsLabel('購物車')), const Size(48, 48));
    expect(tester.getSize(find.bySemanticsLabel('聊天')), const Size(48, 48));
    for (final label in ['返回', '購物車', '聊天']) {
      final inkWell = find.descendant(
        of: find.bySemanticsLabel(label),
        matching: find.byType(InkWell),
      );
      expect(tester.getSize(inkWell), const Size(48, 48));
    }
  });

  testWidgets('storefront header reserves room at 200 percent text scale', (
    tester,
  ) async {
    const viewport = Size(320, 568);
    await pumpHeader(tester, viewport: viewport, textScale: 2);

    expect(tester.takeException(), isNull);
    expect(
      tester.getBottomLeft(find.text('年度優選店舖')).dy,
      lessThanOrEqualTo(
        tester.getRect(find.byKey(const Key('merchant_natural_header'))).bottom,
      ),
    );
  });

  testWidgets('store name replaces the general information heading', (
    tester,
  ) async {
    await pumpHeader(tester, viewport: const Size(768, 1024));

    expect(tester.takeException(), isNull);
    final name = find.text('範例生鮮友善農產合作店舖');
    expect(name, findsOneWidget);
    expect(find.text('店舖資訊'), findsNothing);
    expect(
      find.byKey(const Key('merchant_name_in_general_panel')),
      findsOneWidget,
    );
    expect(
      tester
          .getSize(find.byKey(const Key('merchant_profile_details_two_column')))
          .width,
      MerchantLayoutSpec.profileContentMaxWidth,
    );
  });

  testWidgets('profile details use aligned two columns from 360dp', (
    tester,
  ) async {
    await pumpHeader(tester, viewport: const Size(360, 844));

    expect(
      find.byKey(const Key('merchant_profile_details_two_column')),
      findsOneWidget,
    );
    expect(
      find.byKey(const Key('merchant_profile_details_stacked')),
      findsNothing,
    );
    final general = tester.getRect(
      find.byKey(const Key('merchant_general_panel')),
    );
    final about = tester.getRect(find.byKey(const Key('merchant_about_panel')));
    expect(general.left, lessThan(about.left));
    expect(general.top, closeTo(about.top, 0.1));
    expect(
      tester
          .getSize(find.byKey(const Key('merchant_profile_details_two_column')))
          .height,
      MerchantLayoutSpec.forSize(const Size(360, 844)).profileDetailsPaneHeight,
    );
    final details = tester.getRect(
      find.byKey(const Key('merchant_profile_details_two_column')),
    );
    expect(general.bottom, lessThanOrEqualTo(details.bottom));
    expect(about.bottom, lessThanOrEqualTo(details.bottom));
  });

  testWidgets('storefront avatar and certification badge use prominent sizes', (
    tester,
  ) async {
    const viewport = Size(390, 844);
    await pumpHeader(tester, viewport: viewport);
    final spec = MerchantLayoutSpec.forSize(viewport);

    expect(
      tester.getSize(find.byKey(const Key('merchant_profile_avatar'))),
      Size.square(spec.avatarSize),
    );
    expect(spec.avatarSize, 88);
    final frame = tester.widget<Container>(
      find.byKey(const Key('merchant_profile_avatar_frame')),
    );
    expect(frame.padding, const EdgeInsets.all(0.5));
    final frameDecoration = frame.decoration! as BoxDecoration;
    expect(frameDecoration.border!.top.width, 0.5);
    expect(find.byKey(const Key('merchant_certification_badge')), findsNothing);
    expect(spec.profileDetailsPaneHeight, lessThan(188));
    expect(tester.takeException(), isNull);
  });

  testWidgets('profile details stack below 360dp', (tester) async {
    await pumpHeader(tester, viewport: const Size(359, 720));

    expect(
      find.byKey(const Key('merchant_profile_details_stacked')),
      findsOneWidget,
    );
    final general = tester.getRect(
      find.byKey(const Key('merchant_general_panel')),
    );
    final about = tester.getRect(find.byKey(const Key('merchant_about_panel')));
    expect(about.top - general.bottom, 4);
  });

  testWidgets('profile detail panes have transparent backgrounds', (
    tester,
  ) async {
    await pumpHeader(tester, viewport: const Size(390, 844));

    for (final key in ['merchant_general_panel', 'merchant_about_panel']) {
      final box = tester.widget<DecoratedBox>(find.byKey(Key(key)));
      final decoration = box.decoration as BoxDecoration;
      expect(decoration.color, Colors.transparent);
      expect(decoration.border, isNull);
    }
  });

  testWidgets('three-item merchant summary removes reserved empty space', (
    tester,
  ) async {
    const viewport = Size(390, 844);
    await pumpHeader(
      tester,
      viewport: viewport,
      merchantAward: '',
      rating: 0,
      ratingCount: 0,
      description: '',
    );

    final details = tester.getSize(
      find.byKey(const Key('merchant_profile_details_two_column')),
    );
    final spec = MerchantLayoutSpec.forSize(viewport);
    expect(
      details.height,
      spec.profileDetailsPaneHeightFor(
        hasAward: false,
        displayedLocation: '新北市中和區',
        description: '',
      ),
    );
    expect(details.height, lessThan(spec.profileDetailsPaneHeight));
    final headerBottom = tester
        .getRect(find.byKey(const Key('merchant_natural_header')))
        .bottom;
    final subscriberBottom = tester.getBottomLeft(find.text('1.3萬位訂閱者')).dy;
    expect(headerBottom - subscriberBottom, lessThanOrEqualTo(20));
    expect(find.text('尚無評分'), findsOneWidget);
    expect(find.text('商家介紹'), findsOneWidget);
    expect(find.text('該商家沒有填寫'), findsOneWidget);
  });

  testWidgets('long about copy reserves the read-more row without overflow', (
    tester,
  ) async {
    const viewport = Size(390, 844);
    await pumpHeader(
      tester,
      viewport: viewport,
      merchantAward: '',
      merchantLocation: '台中市南區',
      description: longDescription,
    );

    expect(
      tester
          .getSize(find.byKey(const Key('merchant_profile_details_two_column')))
          .height,
      MerchantLayoutSpec.profileDetailsReadMorePaneHeight,
    );
    expect(find.byKey(const Key('merchant_about_read_more')), findsOneWidget);
    final headerBottom = tester
        .getRect(find.byKey(const Key('merchant_natural_header')))
        .bottom;
    final readMoreBottom = tester
        .getRect(find.byKey(const Key('merchant_about_read_more')))
        .bottom;
    expect(headerBottom - readMoreBottom, closeTo(8, 0.1));
    expect(tester.takeException(), isNull);
  });

  testWidgets(
    'intermediate iOS text scales let both profile panes grow without overflow',
    (tester) async {
      const viewport = Size(402, 874);
      const cases = [
        (textScale: 1.10, merchantAward: '', merchantLocation: '台中市南區'),
        (
          textScale: 1.14,
          merchantAward: '年度優選店舖暨產地直送夥伴',
          merchantLocation: '台南市永康區中華路一段 123 號',
        ),
      ];

      for (final caseData in cases) {
        await pumpHeader(
          tester,
          viewport: viewport,
          textScale: caseData.textScale,
          merchantAward: caseData.merchantAward,
          merchantLocation: caseData.merchantLocation,
          description: longDescription,
        );

        final details = tester.getRect(
          find.byKey(const Key('merchant_profile_details_two_column')),
        );
        for (final panelKey in const [
          'merchant_general_panel',
          'merchant_about_panel',
        ]) {
          final panel = tester.getRect(find.byKey(Key(panelKey)));
          expect(panel.top, closeTo(details.top, 0.1));
          expect(panel.bottom, lessThanOrEqualTo(details.bottom + 0.1));
        }
        expect(
          tester
              .getRect(find.byKey(const Key('merchant_about_read_more')))
              .bottom,
          lessThanOrEqualTo(details.bottom - HkSpace.sm + 0.1),
        );
        expect(tester.takeException(), isNull);
      }
    },
  );

  test('award or wrapped location restores the four-row header height', () {
    final spec = MerchantLayoutSpec.forSize(const Size(390, 844));

    final shortSummary = spec.profileSummaryNeedsFullHeight(
      hasAward: false,
      displayedLocation: '新北市中和區',
    );
    final wrappedLocation = spec.profileSummaryNeedsFullHeight(
      hasAward: false,
      displayedLocation: '台南市永康區中華路一段 123 號',
    );
    final withAward = spec.profileSummaryNeedsFullHeight(
      hasAward: true,
      displayedLocation: '新北市中和區',
    );

    expect(shortSummary, isFalse);
    expect(wrappedLocation, isTrue);
    expect(withAward, isTrue);
    expect(
      spec.profileDetailsPaneHeightFor(
        hasAward: false,
        displayedLocation: '台南市永康區中華路一段 123 號',
        description: '',
      ),
      spec.profileDetailsPaneHeight,
    );
  });

  testWidgets('bounded system text stacks storefront details for readability', (
    tester,
  ) async {
    await pumpHeader(tester, viewport: const Size(390, 844), textScale: 1.4);

    expect(
      find.byKey(const Key('merchant_profile_details_two_column')),
      findsNothing,
    );
    expect(
      find.byKey(const Key('merchant_profile_details_stacked')),
      findsOneWidget,
    );
    expect(tester.takeException(), isNull);
  });

  testWidgets(
    'in-app large text boundary keeps storefront matrix overflow-free',
    (tester) async {
      for (final viewport in const [Size(390, 844), Size(768, 1024)]) {
        await pumpHeader(
          tester,
          viewport: viewport,
          textScale: 1,
          textSize: AppTextSize.large,
          merchantAward: '年度優選店舖暨產地直送夥伴',
          merchantLocation: '台南市西港區中山路一段 123 號',
          rating: 0,
          ratingCount: 0,
          description: longDescription,
        );

        expect(tester.takeException(), isNull, reason: 'viewport=$viewport');
        expect(
          find.byKey(const Key('merchant_profile_details_two_column')),
          findsNothing,
        );
        expect(
          find.byKey(const Key('merchant_profile_details_stacked')),
          findsOneWidget,
        );
        final headerBottom = tester
            .getRect(find.byKey(const Key('merchant_natural_header')))
            .bottom;
        for (final panelKey in const [
          Key('merchant_general_panel'),
          Key('merchant_about_panel'),
        ]) {
          expect(
            tester.getRect(find.byKey(panelKey)).bottom,
            lessThanOrEqualTo(headerBottom),
            reason: '$panelKey must remain inside the natural-height header',
          );
        }
      }
    },
  );

  testWidgets('storefront layout preserves nonlinear platform text scaling', (
    tester,
  ) async {
    await pumpHeader(
      tester,
      viewport: const Size(390, 844),
      textScaler: const _FontSpecificTextScaler(),
    );

    expect(
      find.byKey(const Key('merchant_profile_details_two_column')),
      findsNothing,
    );
    expect(
      find.byKey(const Key('merchant_profile_details_stacked')),
      findsOneWidget,
    );
    expect(tester.takeException(), isNull);
  });

  testWidgets('empty seller description stays truthful and visually muted', (
    tester,
  ) async {
    useViewport(tester, const Size(320, 568));
    await tester.pumpWidget(
      const MaterialApp(
        home: MediaQuery(
          data: MediaQueryData(size: Size(320, 568)),
          child: Scaffold(
            body: MerchantAboutSection(
              merchantName: '金湖水產',
              location: '新北市中和區',
            ),
          ),
        ),
      ),
    );

    expect(tester.takeException(), isNull);
    expect(find.text('商家介紹'), findsOneWidget);
    expect(find.text(merchantAboutEmptyLabel), findsOneWidget);
    expect(find.textContaining('歡迎到 Hawkr'), findsNothing);
    expect(find.byKey(const Key('merchant_about_read_more')), findsNothing);
  });

  testWidgets('long seller description opens with compact spatial motion', (
    tester,
  ) async {
    useViewport(tester, const Size(390, 844));
    await tester.pumpWidget(
      const MaterialApp(
        home: MediaQuery(
          data: MediaQueryData(size: Size(390, 844)),
          child: Scaffold(
            body: MerchantAboutSection(
              merchantName: '測試店舖',
              location: '台南市',
              avatarPayload: avatarData,
              description: longDescription,
            ),
          ),
        ),
      ),
    );

    expect(find.byKey(const Key('merchant_about_read_more')), findsOneWidget);
    final summary = tester.widget<Text>(
      find.byKey(const Key('merchant_about_summary')),
    );
    expect(summary.maxLines, 2);
    await tester.tap(find.byKey(const Key('merchant_about_read_more')));
    await tester.pump();
    expect(find.byKey(const Key('merchant_about_banner')), findsOneWidget);
    expect(find.byType(BottomSheet), findsNothing);
    expect(
      tester.getSize(find.byKey(const Key('merchant_about_avatar'))),
      const Size.square(40),
    );
    final avatarClip = tester.widget<ClipRRect>(
      find.descendant(
        of: find.byKey(const Key('merchant_about_avatar')),
        matching: find.byType(ClipRRect),
      ),
    );
    expect(avatarClip.borderRadius, BorderRadius.circular(12));

    final openingHeight = tester
        .getSize(find.byKey(const Key('merchant_about_banner')))
        .height;
    expect(
      find.byKey(const ValueKey('hk-modal-motion-translate')),
      findsOneWidget,
    );
    expect(find.byKey(const ValueKey('hk-modal-motion-scale')), findsOneWidget);
    expect(
      find.descendant(
        of: find.byKey(const Key('merchant_about_banner')),
        matching: find.byType(FadeTransition),
      ),
      findsNothing,
    );
    expect(find.byKey(const Key('merchant_about_banner_reveal')), findsNothing);

    await tester.pump(const Duration(milliseconds: 120));
    final movingHeight = tester
        .getSize(find.byKey(const Key('merchant_about_banner')))
        .height;
    await tester.pumpAndSettle();
    final fullHeight = tester
        .getSize(find.byKey(const Key('merchant_about_banner')))
        .height;
    expect(tester.takeException(), isNull);
    expect(movingHeight, openingHeight);
    expect(fullHeight, openingHeight);
    final banner = tester.widget<Material>(
      find.byKey(const Key('merchant_about_banner')),
    );
    expect(banner.color, AppColors.neutralWhite);
    expect(find.byKey(const Key('merchant_about_full_text')), findsOneWidget);
    expect(find.text(longDescription), findsWidgets);
    expect(find.text('收合'), findsNothing);
  });

  testWidgets('seller description keeps a readable max width on desktop', (
    tester,
  ) async {
    useViewport(tester, const Size(1200, 900));
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(
          body: MerchantAboutSection(
            merchantName: '金湖水產',
            location: '新北市中和區',
            description: '每日嚴選新鮮水產，提供清楚的保存與料理資訊。',
          ),
        ),
      ),
    );

    expect(tester.takeException(), isNull);
    expect(
      tester.getSize(find.byKey(const Key('merchant_about_card'))).width,
      720,
    );
  });
}

class _FontSpecificTextScaler extends TextScaler {
  const _FontSpecificTextScaler();

  @override
  double scale(double fontSize) => fontSize <= 1 ? fontSize : fontSize * 1.4;

  @override
  double get textScaleFactor => 1.4;
}
