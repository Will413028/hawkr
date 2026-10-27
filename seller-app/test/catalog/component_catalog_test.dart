import 'dart:ui' as ui;

import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/main_catalog.dart' as catalog;
import 'package:hawkr_seller/shared/widgets/hk_choice_chip.dart';
import 'package:hawkr_seller/shared/widgets/hk_glass_app_bar.dart';
import 'package:hawkr_seller/shared/widgets/hk_media_picker_layout.dart';
import 'package:hawkr_seller/shared/widgets/hk_round_action_button.dart';
import 'package:hawkr_seller/shared/widgets/green_header_bar.dart';
import 'package:hawkr_seller/shared/widgets/step_indicator.dart';
import 'package:hawkr_seller/shared/widgets/storefront_top_bar_icon_button.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_test/flutter_test.dart';

Future<void> pumpCatalog(
  WidgetTester tester, {
  String component = 'foundations',
  double width = 320,
  double height = 896,
  double devicePixelRatio = 1,
  EdgeInsets safeInsets = EdgeInsets.zero,
  double scale = 1.4,
  bool embedded = true,
}) async {
  tester.view.physicalSize = Size(
    width * devicePixelRatio,
    height * devicePixelRatio,
  );
  tester.view.devicePixelRatio = devicePixelRatio;
  tester.view.padding = FakeViewPadding(
    left: safeInsets.left * devicePixelRatio,
    top: safeInsets.top * devicePixelRatio,
    right: safeInsets.right * devicePixelRatio,
    bottom: safeInsets.bottom * devicePixelRatio,
  );
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  addTearDown(tester.view.resetPadding);
  await tester.pumpWidget(
    catalog.SellerComponentCatalogApp.fromUri(
      Uri.parse(
        'https://catalog.test/?component=$component&textScale=$scale&embedded=$embedded',
      ),
    ),
  );
  await tester.pump(const Duration(milliseconds: 100));
}

void main() {
  test('deep links select components and safely constrain text size', () {
    final app = catalog.SellerComponentCatalogApp.fromUri(
      Uri.parse(
        'https://catalog.test/?component=buttons&textScale=1.4&embedded=true',
      ),
    );
    expect(app.initialComponent, 'buttons');
    expect(app.textScale, 1.4);
    expect(app.embedded, isTrue);
    expect(
      catalog.SellerComponentCatalogApp.fromUri(
        Uri.parse('https://catalog.test/?component=chrome&embedded=true'),
      ).initialComponent,
      'chrome',
    );
    for (final invalid in ['NaN', 'Infinity', 'invalid']) {
      expect(
        catalog.SellerComponentCatalogApp.fromUri(
          Uri.parse(
            'https://catalog.test/?component=unknown&textScale=$invalid',
          ),
        ).textScale,
        1,
      );
    }
    expect(
      catalog.SellerComponentCatalogApp.fromUri(
        Uri.parse('https://catalog.test/?component=unknown&textScale=10'),
      ).initialComponent,
      'foundations',
    );
  });

  testWidgets('actual entrypoint starts without production bootstrap', (
    tester,
  ) async {
    catalog.main();
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));
    expect(find.text('Hawkr · 賣家元件庫'), findsOneWidget);
    final context = tester.element(find.byType(Scaffold));
    expect(Theme.of(context).colorScheme.primary, AppColors.actionPrimary);
    expect(tester.takeException(), isNull);
  });

  for (final width in [320.0, 390.0, 402.0, 440.0, 768.0]) {
    for (final component in catalog.sellerCatalogComponents.keys) {
      testWidgets(
        '$component at ${width.toInt()}px and 1.4 text has no overflow',
        (tester) async {
          await pumpCatalog(tester, component: component, width: width);
          expect(
            find.text(catalog.sellerCatalogComponents[component]!),
            findsOneWidget,
          );
          final context = tester.element(find.byType(Scaffold).first);
          expect(
            MediaQuery.textScalerOf(context).scale(16),
            closeTo(22.4, 0.01),
          );
          expect(tester.takeException(), isNull);
          await tester.pumpWidget(const SizedBox.shrink());
        },
      );
    }
  }

  for (final viewport in [
    (size: const Size(320, 568), insets: const EdgeInsets.only(top: 20)),
    (
      size: const Size(402, 874),
      insets: const EdgeInsets.only(top: 62, bottom: 34),
    ),
    (
      size: const Size(440, 956),
      insets: const EdgeInsets.only(top: 62, bottom: 34),
    ),
    (
      size: const Size(874, 402),
      insets: const EdgeInsets.fromLTRB(62, 0, 62, 21),
    ),
  ]) {
    for (final scale in [1.0, 1.4]) {
      testWidgets(
        'chrome at ${viewport.size} and $scale text respects safe insets',
        (tester) async {
          await pumpCatalog(
            tester,
            component: 'chrome',
            width: viewport.size.width,
            height: viewport.size.height,
            devicePixelRatio: 3,
            safeInsets: viewport.insets,
            scale: scale,
          );
          expect(find.byType(GreenHeaderBar), findsNWidgets(2));
          expect(find.byType(HkGlassAppBar), findsOneWidget);
          final scrollRect = tester.getRect(
            find.byKey(const ValueKey('catalog-scroll')),
          );
          expect(scrollRect.left, greaterThanOrEqualTo(viewport.insets.left));
          expect(scrollRect.top, greaterThanOrEqualTo(viewport.insets.top));
          expect(
            scrollRect.right,
            lessThanOrEqualTo(viewport.size.width - viewport.insets.right),
          );
          expect(
            scrollRect.bottom,
            lessThanOrEqualTo(viewport.size.height - viewport.insets.bottom),
          );
          expect(tester.takeException(), isNull);
          for (final key in ['catalog-green-save', 'catalog-glass-save']) {
            final save = find.byKey(ValueKey(key));
            await tester.ensureVisible(save);
            await tester.tap(save);
            await tester.pump();
          }
          expect(find.text('頁首操作次數：2'), findsOneWidget);
          final storeHeader = find.byKey(
            const ValueKey('catalog-store-header'),
          );
          await tester.ensureVisible(storeHeader);
          await tester.pump();
          final award = tester.getRect(find.text('年度友善耕作優良農產品獎項'));
          expect(
            award.bottom,
            lessThanOrEqualTo(tester.getRect(storeHeader).bottom),
          );
          final controls = find.descendant(
            of: storeHeader,
            matching: find.byType(StorefrontTopBarIconButton),
          );
          expect(controls, findsNWidgets(2));
          for (final control in controls.evaluate()) {
            expect(
              tester.getSize(find.byWidget(control.widget)),
              const Size.square(48),
            );
          }
          expect(tester.takeException(), isNull);
        },
      );
    }
  }

  testWidgets('safe header paints continuously across the 62px inset edge', (
    tester,
  ) async {
    await pumpCatalog(tester, component: 'chrome', width: 402, height: 874);
    final header = find.byKey(const ValueKey('catalog-safe-header'));
    await tester.ensureVisible(header);
    await tester.pump();
    final headerRect = tester.getRect(header);
    final toolbarRect = tester.getRect(
      find.descendant(of: header, matching: find.byType(NavigationToolbar)),
    );
    final surfaceRect = tester.getRect(
      find.descendant(
        of: header,
        matching: find.byKey(const ValueKey('seller-glass-header-surface')),
      ),
    );
    expect(headerRect.height, 118);
    expect(surfaceRect, headerRect);
    expect(toolbarRect.top - headerRect.top, 62);
    expect(toolbarRect.height, 56);

    await tester.runAsync(() async {
      final boundary = tester.renderObject<RenderRepaintBoundary>(
        find.byKey(const ValueKey('catalog-safe-header-preview')),
      );
      final image = await boundary.toImage(pixelRatio: 1);
      try {
        final bytes = (await image.toByteData(
          format: ui.ImageByteFormat.rawRgba,
        ))!;
        // Sample beside the title, on either side of the status/toolbar edge.
        // A glass gradient may change slightly; a separately painted strip
        // must not create a visible color jump at y=62.
        for (var channel = 0; channel < 3; channel++) {
          final inset = bytes.getUint8((60 * image.width + 10) * 4 + channel);
          final toolbar = bytes.getUint8((64 * image.width + 10) * 4 + channel);
          expect((inset - toolbar).abs(), lessThanOrEqualTo(3));
        }
      } finally {
        image.dispose();
      }
    });
    expect(tester.takeException(), isNull);
  });

  for (final size in [const Size(320, 568), const Size(874, 402)]) {
    testWidgets('media catalog keeps preview and gallery visible at $size', (
      tester,
    ) async {
      await pumpCatalog(
        tester,
        component: 'chrome',
        width: size.width,
        height: size.height,
      );
      final picker = find.byType(HkMediaPickerLayout);
      await tester.ensureVisible(picker);
      final scroll = tester.getRect(
        find.byKey(const ValueKey('catalog-scroll')),
      );
      for (final key in ['media-picker-preview', 'media-picker-grid']) {
        final pane = tester.getRect(find.byKey(ValueKey(key)));
        expect(pane.width, greaterThan(0));
        expect(pane.height, greaterThan(0));
        expect(scroll.contains(pane.topLeft), isTrue);
        expect(scroll.contains(pane.bottomRight - const Offset(1, 1)), isTrue);
      }
      await tester.tap(find.byKey(const ValueKey('catalog-photo-1')));
      await tester.pump();
      expect(find.text('範例照片 2'), findsOneWidget);
      expect(tester.takeException(), isNull);
    });
  }

  testWidgets(
    'compact navigation searches and selects the real button examples',
    (tester) async {
      await pumpCatalog(tester, embedded: false);
      await tester.enterText(
        find.byKey(const ValueKey('catalog-search')),
        'buttons',
      );
      await tester.pump();
      expect(find.byKey(const ValueKey('nav-cards')), findsNothing);
      await tester.tap(find.byKey(const ValueKey('nav-buttons')));
      await tester.pump();
      expect(find.byType(HkRoundActionButton), findsNWidgets(2));
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets('production buttons update state and preserve disabled minimum', (
    tester,
  ) async {
    await pumpCatalog(tester, component: 'buttons');
    expect(
      tester
          .widget<HkRoundActionButton>(
            find.byKey(const ValueKey('quantity-minus')),
          )
          .onPressed,
      isNull,
    );
    await tester.tap(find.byKey(const ValueKey('primary-action')));
    await tester.pump();
    expect(find.text('操作次數：1'), findsOneWidget);
    await tester.ensureVisible(find.byKey(const ValueKey('quantity-plus')));
    await tester.tap(find.byKey(const ValueKey('quantity-plus')));
    await tester.pump();
    expect(
      tester.widget<Text>(find.byKey(const ValueKey('quantity'))).data,
      '2',
    );
    expect(
      tester.getSize(find.byKey(const ValueKey('quantity-plus'))),
      const Size.square(48),
    );
    await tester.tap(find.byKey(const ValueKey('quantity-minus')));
    await tester.pump();
    expect(
      tester.widget<Text>(find.byKey(const ValueKey('quantity'))).data,
      '1',
    );
    expect(tester.takeException(), isNull);
  });

  testWidgets('real form validates empty input and accepts valid text', (
    tester,
  ) async {
    await pumpCatalog(tester, component: 'inputs');
    await tester.ensureVisible(find.byKey(const ValueKey('validate-form')));
    await tester.tap(find.byKey(const ValueKey('validate-form')));
    await tester.pump();
    expect(find.text('請填寫商品名稱'), findsOneWidget);
    await tester.ensureVisible(find.byKey(const ValueKey('product-name')));
    await tester.enterText(find.byKey(const ValueKey('product-name')), '季節蔬菜箱');
    await tester.ensureVisible(find.byKey(const ValueKey('validate-form')));
    await tester.tap(find.byKey(const ValueKey('validate-form')));
    await tester.pump();
    expect(find.text('表單檢查通過'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets(
    'frosted action demos share callbacks and keep busy actions inert',
    (tester) async {
      await pumpCatalog(tester, component: 'buttons');
      for (final action in ['send', 'more', 'save', 'disabled', 'loading']) {
        final button = find.byKey(ValueKey('catalog-frosted-$action'));
        await tester.ensureVisible(button);
        await tester.tap(button);
        await tester.pump();
      }
      expect(find.text('操作次數：3'), findsOneWidget);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets('choice chips and commerce progress are interactive', (
    tester,
  ) async {
    await pumpCatalog(tester, component: 'chips');
    await tester.tap(find.widgetWithText(HkChoiceChip, '草稿'));
    await tester.pump();
    expect(find.text('目前篩選：草稿'), findsOneWidget);
    await tester.pumpWidget(const SizedBox.shrink());
    await pumpCatalog(tester, component: 'commerce');
    await tester.tap(find.byKey(const ValueKey('temperature-select-冷凍')));
    await tester.pump();
    expect(find.text('選擇溫層：冷凍'), findsOneWidget);
    await tester.ensureVisible(find.byKey(const ValueKey('next-step')));
    await tester.tap(find.byKey(const ValueKey('next-step')));
    await tester.pump();
    expect(
      tester.widget<StepIndicator>(find.byType(StepIndicator)).currentStep,
      2,
    );
    expect(tester.takeException(), isNull);
  });

  for (final width in [320.0, 402.0]) {
    for (final scale in [1.0, 1.4]) {
      testWidgets(
        'order pills retain content width and targets at $width/$scale',
        (tester) async {
          await pumpCatalog(
            tester,
            component: 'commerce',
            width: width,
            scale: scale,
          );
          final actions = find.byKey(const ValueKey('catalog-order-actions'));
          await tester.ensureVisible(actions);
          final available = tester.getSize(actions).width;
          final details = tester.getRect(
            find.byKey(const ValueKey('catalog-order-details')),
          );
          final shipping = tester.getRect(
            find.byKey(const ValueKey('catalog-order-shipping')),
          );
          expect(details.top, shipping.top);
          expect(details.right, lessThan(shipping.left));
          expect(shipping.width, greaterThan(details.width));
          for (final button in [details, shipping]) {
            expect(button.width, lessThan(available / 2));
            expect(button.width, greaterThanOrEqualTo(48));
            expect(button.height, greaterThanOrEqualTo(48));
            // The invisible padding above the visual pill remains tappable.
            await tester.tapAt(button.topCenter + const Offset(0, 1));
            await tester.pump();
          }
          expect(find.text('營運操作次數：2'), findsOneWidget);
          expect(tester.takeException(), isNull);
        },
      );
    }
  }
}
