import 'package:hawkr_buyer/catalog/component_catalog.dart';
import 'package:hawkr_buyer/core/theme/app_theme.dart';
import 'package:hawkr_buyer/features/cart/presentation/widgets/cart_checkout_action.dart';
import 'package:hawkr_buyer/features/cart/presentation/widgets/cart_product_tile.dart';
import 'package:hawkr_buyer/main_catalog.dart' as catalog_entrypoint;
import 'package:hawkr_buyer/shared/widgets/hk_choice_chip.dart';
import 'package:hawkr_buyer/shared/widgets/product_card.dart';
import 'package:hawkr_buyer/shared/widgets/quantity_stepper.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets(
    'real catalog entrypoint starts without Firebase or account setup',
    (tester) async {
      catalog_entrypoint.main();
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 400));

      expect(find.byType(BuyerComponentCatalogApp), findsOneWidget);
      expect(find.text('買家 · 基礎樣式'), findsOneWidget);
      final app = tester.widget<MaterialApp>(find.byType(MaterialApp));
      expect(app.theme!.colorScheme, AppTheme.light.colorScheme);
      expect(app.theme!.filledButtonTheme, AppTheme.light.filledButtonTheme);
      expect(tester.takeException(), isNull);
    },
  );

  for (final width in [320.0, 390.0, 768.0]) {
    for (final scale in [1.0, 1.4]) {
      for (final section in buyerCatalogSections.keys) {
        testWidgets(
          '$section renders all examples at ${width.toInt()}px / ${scale}x without overflow',
          (tester) async {
            await _pump(tester, section: section, width: width, scale: scale);
            expect(
              find.text('買家 · ${buyerCatalogSections[section]}'),
              findsOneWidget,
            );
            expect(tester.takeException(), isNull);
            // SingleChildScrollView eagerly lays out every actual widget, including
            // below-fold examples. Moving to the bottom also checks scroll bounds.
            await tester.drag(
              find.byKey(ValueKey('catalog-scroll-$section')),
              const Offset(0, -1600),
            );
            await tester.pump(const Duration(milliseconds: 200));
            expect(tester.takeException(), isNull);
          },
        );
      }
    }
  }

  testWidgets('search locates and opens the actual production component', (
    tester,
  ) async {
    await _pump(tester, section: 'foundations', width: 1280, embedded: false);
    await tester.enterText(
      find.byKey(const ValueKey('catalog-search')),
      'ProductCard',
    );
    await tester.pump();
    expect(find.byKey(const ValueKey('catalog-nav-commerce')), findsOneWidget);
    expect(find.byKey(const ValueKey('catalog-nav-buttons')), findsNothing);
    await tester.tap(find.byKey(const ValueKey('catalog-nav-commerce')));
    await tester.pump(const Duration(milliseconds: 300));
    expect(find.byType(ProductCard), findsNWidgets(2));
    expect(find.byType(QuantityStepper), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('button demo responds to the real FilledButton callback', (
    tester,
  ) async {
    await _pump(tester, section: 'buttons');
    await tester.tap(find.byKey(const ValueKey('catalog-primary-action')));
    await tester.pump();
    expect(find.text('已操作 1 次'), findsOneWidget);
  });

  testWidgets('form shows required error then clears after valid input', (
    tester,
  ) async {
    await _pump(tester, section: 'inputs');
    await tester.ensureVisible(find.byKey(const ValueKey('catalog-validate')));
    await tester.tap(find.byKey(const ValueKey('catalog-validate')));
    await tester.pump();
    expect(find.text('請填寫收件人姓名'), findsOneWidget);
    await tester.enterText(
      find.byKey(const ValueKey('catalog-name-input')),
      '元件測試',
    );
    await tester.ensureVisible(find.byKey(const ValueKey('catalog-validate')));
    await tester.tap(find.byKey(const ValueKey('catalog-validate')));
    await tester.pump();
    expect(find.text('請填寫收件人姓名'), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets(
    'frosted action demos share callbacks and keep busy actions inert',
    (tester) async {
      await _pump(tester, section: 'buttons');
      for (final action in ['send', 'more', 'save', 'disabled', 'loading']) {
        final button = find.byKey(ValueKey('catalog-frosted-$action'));
        await tester.ensureVisible(button);
        await tester.tap(button);
        await tester.pump();
      }
      expect(find.text('已操作 3 次'), findsOneWidget);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets('filter and quantity controls update and enforce stock bounds', (
    tester,
  ) async {
    await _pump(tester, section: 'chips');
    final chilled = find.widgetWithText(HkChoiceChip, '冷藏');
    await tester.tap(chilled);
    await tester.pump();
    expect(tester.widget<HkChoiceChip>(chilled).selected, isTrue);
    expect(find.text('目前篩選：冷藏'), findsOneWidget);

    await _pump(tester, section: 'commerce');
    final plus = find.byKey(const ValueKey('quantity-stepper-plus-visual'));
    for (var i = 0; i < 6; i++) {
      await tester.tap(plus);
      await tester.pump();
    }
    expect(find.text('範例數量：5 / 5'), findsOneWidget);
    await tester.tap(
      find.byKey(const ValueKey('quantity-stepper-minus-visual')),
    );
    await tester.pump();
    expect(find.text('範例數量：4 / 5'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets(
    'cart preview edits address and opens review without transactions',
    (tester) async {
      await _pump(tester, section: 'cart');
      final edit = find.byKey(const ValueKey('cart-edit-address'));
      await tester.ensureVisible(edit);
      await tester.tap(edit);
      await tester.pumpAndSettle();
      expect(find.text('選擇收件地址'), findsOneWidget);
      await tester.tap(
        find.byKey(const ValueKey('checkout-address-picker-add')),
      );
      await tester.pumpAndSettle();
      expect(find.text('選擇收件地址'), findsNothing);
      final checkout = find.byKey(const ValueKey('cart-checkout-action'));
      await tester.ensureVisible(checkout);
      await tester.tap(checkout);
      await tester.pump();
      expect(find.text('預覽操作：2 次'), findsOneWidget);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets(
    'cart text controls reuse existing small/standard/large scaling',
    (tester) async {
      await _pump(tester, section: 'cart', width: 320);
      final title = tester.widget<Text>(find.text('買家 · 購物車')).style!;
      expect((title.fontSize, title.height), (22, 28 / 22));
      for (final choice in <String, double>{
        '小': 0.9,
        '中': 1,
        '大': 1.15,
      }.entries) {
        final control = find.descendant(
          of: find.byKey(const ValueKey('catalog-cart-text-size')),
          matching: find.text(choice.key),
        );
        await tester.ensureVisible(control);
        await tester.tap(control);
        await tester.pump();
        final body = tester.element(find.text('商品總金額'));
        expect(
          MediaQuery.textScalerOf(body).scale(16),
          closeTo(16 * choice.value, 0.001),
        );
        expect(tester.takeException(), isNull);
      }
    },
  );

  for (final width in [320.0, 390.0]) {
    testWidgets(
      'store actions preserve items and update checkout at $width / 1.4',
      (tester) async {
        await _pump(tester, section: 'cart', width: width, scale: 1.4);
        final selection = find.byKey(
          const ValueKey('cart-store-selection-demo-seafood'),
        );
        for (final label in ['產地直送當季蔬果箱', '規格：家庭箱／約 3 公斤', '嚴選冷凍鮮魚']) {
          expect(
            tester
                .renderObject<RenderParagraph>(find.text(label))
                .didExceedMaxLines,
            false,
            reason: '$label must remain readable at $width / 1.4',
          );
        }
        expect(find.text('改選本店可購買商品'), findsOneWidget);
        expect(find.text('切換結帳店家，原店商品保留'), findsOneWidget);
        expect(tester.getSize(selection).height, greaterThanOrEqualTo(48));
        await tester.ensureVisible(selection);
        await tester.tap(selection);
        await tester.pump();
        final checkout = find.byKey(const ValueKey('cart-checkout-action'));
        await tester.ensureVisible(checkout);
        expect(
          tester
              .widget<CartCheckoutAction>(find.byType(CartCheckoutAction))
              .storeName,
          '範例海鮮店',
        );
        expect(
          tester
              .widget<CartCheckoutAction>(find.byType(CartCheckoutAction))
              .selectedItemCount,
          1,
        );
        expect(find.text('已選 1／1 項可購買商品'), findsOneWidget);
        expect(find.text('產地直送當季蔬果箱'), findsOneWidget);
        expect(find.text('嚴選冷凍鮮魚'), findsOneWidget);
        final unavailable = tester
            .widgetList<CartProductTile>(find.byType(CartProductTile))
            .singleWhere((tile) => tile.item.skuId == 203);
        expect(unavailable.isChecked, false);
        await tester.tap(checkout);
        await tester.pump();
        expect(find.text('預覽操作：1 次'), findsOneWidget);
        await tester.ensureVisible(selection);
        await tester.tap(selection);
        await tester.pump();
        await tester.ensureVisible(checkout);
        expect(find.text('請先選取一家店的商品'), findsOneWidget);
        expect(tester.widget<FilledButton>(checkout).onPressed, isNull);
        expect(find.text('改選本店可購買商品'), findsNothing);
        expect(find.text('選取本店可購買商品'), findsNWidgets(2));
        expect(find.text('全選'), findsNothing);
        expect(find.text('重新整理'), findsNothing);
        expect(tester.takeException(), isNull);
      },
    );
  }

  testWidgets('compact full shell remains usable with enlarged text', (
    tester,
  ) async {
    await _pump(
      tester,
      section: 'buttons',
      width: 320,
      scale: 1.4,
      embedded: false,
    );
    expect(find.byKey(const ValueKey('catalog-search')), findsOneWidget);
    expect(tester.takeException(), isNull);
    await tester.enterText(
      find.byKey(const ValueKey('catalog-search')),
      '不存在的元件',
    );
    await tester.pump();
    expect(find.text('找不到元件，請換個關鍵字。'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });
}

Future<void> _pump(
  WidgetTester tester, {
  required String section,
  double width = 390,
  double scale = 1,
  bool embedded = true,
}) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = Size(width, 844);
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  await tester.pumpWidget(
    BuyerComponentCatalogApp(
      key: UniqueKey(),
      initialUri: Uri.parse(
        'http://localhost/?component=$section&textScale=$scale&embedded=$embedded',
      ),
    ),
  );
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 400));
}
