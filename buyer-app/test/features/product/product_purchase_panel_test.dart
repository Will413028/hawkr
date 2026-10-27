import 'package:hawkr_buyer/shared/widgets/hk_temperature_tag.dart';

import 'package:hawkr_buyer/core/layout/app_media_query_policy.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/features/product/presentation/widgets/product_purchase_panel.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:hawkr_buyer/shared/widgets/quantity_stepper.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('product purchase spec helpers', () {
    test('normalizes API fields from the gateway SkuItem spelling', () {
      final specs = normalizeProductPurchaseSpecs(<Object?>[
        <String, dynamic>{
          'id': 11,
          'skuName': '約300g/包',
          'price': '200',
          'salePrice': '180',
          'stockQty': '0',
          'unit': '包',
        },
        <String, dynamic>{
          'skuId': '12',
          'name': '約600g/包',
          'amount': 400,
          'stock': 8,
        },
        'not-a-map',
      ]);

      expect(specs, hasLength(2));
      expect(specs.first, <String, dynamic>{
        'id': '11',
        'name': '約300g/包',
        'price': '200',
        'quantity': '0',
        'unit': '包',
        'salePrice': '180',
      });
      expect(specs[1]['id'], '12');
      expect(specs[1]['name'], '約600g/包');
      expect(specs[1]['price'], 400);
      expect(specs[1]['quantity'], 8);
      expect(specs[1].containsKey('salePrice'), isFalse);
    });

    // 回歸鎖：後端 `SkuItem` 只吐 camelCase。只給舊 snake 拼寫時，
    // 正規化結果不得憑空生出特價欄 —— 任何 `sale_price` fallback 復活即轉紅。
    test('legacy snake_case sku keys are not read back', () {
      final specs = normalizeProductPurchaseSpecs(<Object?>[
        <String, dynamic>{
          'sku_id': '99',
          'name': '舊契約規格',
          'price': 400,
          'sale_price': 350,
          'stock': 8,
        },
      ]);

      expect(specs, hasLength(1));
      expect(specs.first.containsKey('salePrice'), isFalse);
      expect(specs.first.containsKey('sale_price'), isFalse);
      // sku_id 不再被當成 id 來源，id 落回空字串。
      expect(specs.first['id'], '');
    });

    test('reads stock and selects the first in-stock spec', () {
      final specs = <Map<String, dynamic>>[
        <String, dynamic>{'id': 'sold', 'quantity': '-2'},
        <String, dynamic>{'id': 'available', 'stockQty': '7'},
        <String, dynamic>{'id': 'later', 'stock': 9},
      ];

      expect(purchaseSpecStock(null), 0);
      expect(purchaseSpecStock(specs.first), 0);
      expect(purchaseSpecStock(specs[1]), 7);
      expect(firstAvailablePurchaseSpec(specs)?['id'], 'available');
      expect(
        firstAvailablePurchaseSpec(<Map<String, dynamic>>[
          <String, dynamic>{'id': 'sold', 'quantity': 0},
        ]),
        isNull,
      );
    });
  });

  testWidgets('delivery metadata uses document icon and exact carrier mark', (
    tester,
  ) async {
    await _pumpAt(
      tester,
      width: 320,
      textScale: 1.4,
      child: const ProductDeliveryMetadata(
        temperatureCodes: <String>['3201', '3202', '3203'],
        shippingMethodCodes: <String>['3001'],
      ),
    );

    final icon = tester.widget<HkIcon>(
      find.byKey(const ValueKey('product-purchase-delivery-icon')),
    );
    expect(icon.name, HkIconName.document);
    expect(icon.size, 16);
    expect(find.text('配送溫層'), findsOneWidget);
    expect(find.text('常溫'), findsOneWidget);
    expect(find.text('冷藏'), findsOneWidget);
    expect(find.text('冷凍'), findsOneWidget);
    expect(find.byType(HkTemperatureTag), findsNWidgets(3));

    final carrier = find.byKey(const ValueKey('product-purchase-carrier-3001'));
    expect(tester.getSize(carrier), const Size.square(24));
    final carrierIcon = tester.widget<HkIcon>(
      find.descendant(of: carrier, matching: find.byType(HkIcon)),
    );
    expect(carrierIcon.name, HkIconName.truck);
    expect(tester.takeException(), isNull);
  });

  for (final width in <double>[320, 390, 768]) {
    testWidgets('summary and selector stay readable at ${width.toInt()}px and '
        '2.0x system / 1.4x effective text', (tester) async {
      Map<String, dynamic>? selected;
      int? changedQuantity;
      await _pumpAt(
        tester,
        width: width,
        textScale: 2,
        scrollable: true,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            const ProductPurchaseSummaryCard(
              productName: '範例生鮮冷凍野生小花枝',
              imageUrl: '',
              rating: 4.8,
              reviewCount: 326,
              temperatureCodes: <String>['3203'],
              shippingMethodCodes: <String>['3001'],
              loading: false,
            ),
            const SizedBox(height: 16),
            ProductPurchaseSelector(
              specs: _specs,
              selectedSpecId: '11',
              quantity: 1,
              currentPrice: 200,
              currentListPrice: 220,
              loading: false,
              onSpecSelected: (value) => selected = value,
              onQuantityChanged: (value) => changedQuantity = value,
            ),
          ],
        ),
      );

      expect(find.text('範例生鮮冷凍野生小花枝'), findsOneWidget);
      expect(find.text('4.8 (326則評價)'), findsOneWidget);
      expect(find.text('選擇規格'), findsOneWidget);
      expect(find.text('NT\$ '), findsNothing);
      expect(find.textContaining('200'), findsOneWidget);

      final chip = find.byKey(const ValueKey('product-purchase-spec-11'));
      expect(tester.getSize(chip).width, greaterThanOrEqualTo(96));
      final stepper = tester.widget<QuantityStepper>(
        find.byType(QuantityStepper),
      );
      expect(stepper.minCount, 1);
      expect(stepper.maxCount, 12);
      for (final symbol in ['minus', 'plus']) {
        expect(
          tester.getSize(
            find.byKey(ValueKey('quantity-stepper-$symbol-visual')),
          ),
          const Size.square(32),
        );
      }

      if (width >= 390) {
        final priceRect = tester.getRect(
          find.byKey(const ValueKey('product-purchase-price')),
        );
        final quantityRect = tester.getRect(
          find.byKey(const ValueKey('product-purchase-quantity')),
        );
        expect(
          (priceRect.center.dy - quantityRect.center.dy).abs(),
          lessThan(1),
        );
      }

      await tester.tap(find.byKey(const ValueKey('product-purchase-spec-12')));
      await tester.pump();
      expect(selected?['id'], '12');

      await tester.tap(find.bySemanticsLabel('增加數量'));
      await tester.pump();
      expect(changedQuantity, 2);
      expect(tester.takeException(), isNull);
    });

    testWidgets('footer CTAs match at ${width.toInt()}px and 1.4x text', (
      tester,
    ) async {
      var addCount = 0;
      var buyCount = 0;
      await _pumpAt(
        tester,
        width: width,
        textScale: 1.4,
        child: ProductPurchaseFooter(
          hasSelection: true,
          stock: 100,
          enabled: true,
          addLoading: false,
          buyLoading: false,
          onAdd: () => addCount++,
          onBuy: () => buyCount++,
        ),
      );

      expect(find.text('庫存：100 件'), findsOneWidget);
      expect(find.text('加入購物車'), findsOneWidget);
      expect(find.text('立即結帳'), findsOneWidget);
      expect(
        tester
            .getSize(find.byKey(const ValueKey('product-purchase-add')))
            .height,
        HkControl.minTap,
      );
      expect(
        tester
            .getSize(find.byKey(const ValueKey('product-purchase-buy')))
            .height,
        HkControl.minTap,
      );
      for (final entry in [
        (key: 'product-purchase-add', label: '加入購物車'),
        (key: 'product-purchase-buy', label: '立即結帳'),
      ]) {
        final button = tester.getRect(find.byKey(ValueKey(entry.key)));
        final label = tester.getRect(find.text(entry.label));
        expect((button.center.dy - label.center.dy).abs(), lessThan(1));
      }

      final addIcon = tester.widget<HkIcon>(
        find.descendant(
          of: find.byKey(const ValueKey('product-purchase-add')),
          matching: find.byType(HkIcon),
        ),
      );
      final buyIcon = tester.widget<HkIcon>(
        find.descendant(
          of: find.byKey(const ValueKey('product-purchase-buy')),
          matching: find.byType(HkIcon),
        ),
      );
      expect(addIcon.name, HkIconName.cart);
      expect(buyIcon.name, HkIconName.checkoutCoins);
      expect(addIcon.color, AppColors.commerceCartForeground);
      expect(buyIcon.color, AppColors.commerceCheckoutForeground);

      final addSurface = tester.widget<Container>(
        find.byKey(const ValueKey('product-purchase-cart-surface')),
      );
      final buySurface = tester.widget<Container>(
        find.byKey(const ValueKey('product-purchase-checkout-surface')),
      );
      expect(
        (addSurface.decoration! as BoxDecoration).color,
        AppColors.commerceCartAction,
      );
      expect(
        (buySurface.decoration! as BoxDecoration).color,
        AppColors.commerceCheckoutAction,
      );
      expect(
        tester.widget<Text>(find.text('加入購物車')).style?.color,
        AppColors.commerceCartForeground,
      );
      expect(
        tester.widget<Text>(find.text('立即結帳')).style?.color,
        AppColors.commerceCheckoutForeground,
      );

      await tester.tap(find.byKey(const ValueKey('product-purchase-add')));
      await tester.pump();
      await tester.tap(find.byKey(const ValueKey('product-purchase-buy')));
      await tester.pump();
      expect(addCount, 1);
      expect(buyCount, 1);
      expect(tester.takeException(), isNull);
    });
  }

  testWidgets('product purchase controls meet mobile a11y guidelines', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    await _pumpAt(
      tester,
      width: 390,
      textScale: 1.4,
      scrollable: true,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          ProductPurchaseSelector(
            specs: _specs,
            selectedSpecId: '11',
            quantity: 1,
            currentPrice: 200,
            currentListPrice: 220,
            loading: false,
            onSpecSelected: (_) {},
            onQuantityChanged: (_) {},
          ),
          const SizedBox(height: HkSpace.lg),
          ProductPurchaseFooter(
            hasSelection: true,
            stock: 100,
            enabled: true,
            addLoading: false,
            buyLoading: false,
            onAdd: () {},
            onBuy: () {},
          ),
        ],
      ),
    );

    await expectLater(tester, meetsGuideline(androidTapTargetGuideline));
    await expectLater(tester, meetsGuideline(iOSTapTargetGuideline));
    await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));
    semantics.dispose();
  });

  testWidgets('footer stacks only below 320px', (tester) async {
    await _pumpAt(
      tester,
      width: 319,
      textScale: 1.4,
      child: ProductPurchaseFooter(
        hasSelection: true,
        stock: 5,
        enabled: true,
        addLoading: false,
        buyLoading: false,
        onAdd: () {},
        onBuy: () {},
      ),
    );

    final addRect = tester.getRect(
      find.byKey(const ValueKey('product-purchase-add')),
    );
    final buyRect = tester.getRect(
      find.byKey(const ValueKey('product-purchase-buy')),
    );
    expect(buyRect.top, greaterThan(addRect.bottom));
    expect(tester.takeException(), isNull);
  });

  testWidgets('single-spec phone selector uses a compact horizontal row', (
    tester,
  ) async {
    await _pumpAt(
      tester,
      width: 390,
      textScale: 1,
      child: SizedBox(
        width: 318,
        child: ProductPurchaseSelector(
          specs: const <Map<String, dynamic>>[
            <String, dynamic>{
              'id': '11',
              'name': '約250g/包',
              'price': 120,
              'quantity': 12,
            },
          ],
          selectedSpecId: '11',
          quantity: 1,
          currentPrice: 120,
          currentListPrice: null,
          loading: false,
          onSpecSelected: (_) {},
          onQuantityChanged: (_) {},
        ),
      ),
    );

    final selector = tester.getRect(
      find.byKey(const ValueKey('product-purchase-selector')),
    );
    final price = tester.getRect(
      find.byKey(const ValueKey('product-purchase-price')),
    );
    final quantity = tester.getRect(
      find.byKey(const ValueKey('product-purchase-quantity')),
    );
    final chip = tester.getRect(
      find.byKey(const ValueKey('product-purchase-spec-11')),
    );
    final chipTarget = tester.getRect(
      find.byKey(const ValueKey('product-purchase-spec-target-11')),
    );
    final chipLabel = tester.getRect(find.text('約250g/包'));
    final stepper = tester.getRect(
      find.byKey(const ValueKey('quantity-stepper')),
    );
    final minus = tester.getRect(
      find.byKey(const ValueKey('quantity-stepper-minus-visual')),
    );
    final plus = tester.getRect(
      find.byKey(const ValueKey('quantity-stepper-plus-visual')),
    );

    expect(selector.height, lessThan(160));
    expect((price.center.dy - quantity.center.dy).abs(), lessThan(1));
    expect(chipTarget.height, HkControl.minTap);
    expect(chip.height, lessThan(chipTarget.height));
    expect(chip.center.dy, closeTo(chipTarget.center.dy, 0.1));
    expect(
      chipLabel.top - chip.top,
      closeTo(chip.bottom - chipLabel.bottom, 0.1),
    );
    expect(stepper.height, HkControl.minTap);
    expect(find.byKey(const ValueKey('quantity-stepper-frame')), findsNothing);
    expect(minus.size, const Size.square(32));
    expect(plus.size, const Size.square(32));
    expect(minus.top - stepper.top, closeTo(8, 0.1));
    expect(stepper.bottom - plus.bottom, closeTo(8, 0.1));
    expect(minus.left - stepper.left, closeTo(8, 0.1));
    expect(stepper.right - plus.right, closeTo(8, 0.1));
    expect(tester.takeException(), isNull);
  });

  testWidgets('list-price strike is applied to digits only', (tester) async {
    await _pumpAt(
      tester,
      width: 390,
      textScale: 1,
      child: ProductPurchaseSelector(
        specs: const <Map<String, dynamic>>[
          <String, dynamic>{
            'id': '11',
            'name': '150g(2入組)',
            'price': 299,
            'quantity': 12,
          },
        ],
        selectedSpecId: '11',
        quantity: 1,
        currentPrice: 299,
        currentListPrice: 350,
        loading: false,
        onSpecSelected: (_) {},
        onQuantityChanged: (_) {},
      ),
    );

    final richPrice = tester.widget<Text>(
      find
          .descendant(
            of: find.byKey(const ValueKey('product-purchase-price')),
            matching: find.byWidgetPredicate(
              (widget) => widget is Text && widget.textSpan != null,
            ),
          )
          .first,
    );
    final children = (richPrice.textSpan! as TextSpan).children!;
    expect((children[1] as TextSpan).style?.fontSize, 20);
    expect((children[2] as TextSpan).text, '  ');
    expect(
      (children[2] as TextSpan).style?.decoration,
      isNot(TextDecoration.lineThrough),
    );
    expect((children[3] as TextSpan).text, '350');
    expect(
      (children[3] as TextSpan).style?.decoration,
      TextDecoration.lineThrough,
    );
  });
}

const _specs = <Map<String, dynamic>>[
  <String, dynamic>{
    'id': '11',
    'name': '約300g/包',
    'price': 200,
    'quantity': 12,
    'unit': '包',
  },
  <String, dynamic>{
    'id': '12',
    'name': '約600g/家庭包',
    'price': 380,
    'quantity': 5,
    'unit': '包',
  },
];

Future<void> _pumpAt(
  WidgetTester tester, {
  required double width,
  required double textScale,
  required Widget child,
  bool scrollable = false,
}) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = Size(width, width >= 600 ? 1200 : 900);
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  await tester.pumpWidget(
    MaterialApp(
      theme: ThemeData(
        useMaterial3: true,
        textTheme: AppTypography.textTheme,
        scaffoldBackgroundColor: AppColors.bgPrimary,
      ),
      builder: (context, appChild) => MediaQuery(
        data: AppMediaQueryPolicy.apply(
          MediaQuery.of(
            context,
          ).copyWith(textScaler: TextScaler.linear(textScale)),
        ),
        child: appChild!,
      ),
      home: Scaffold(
        body: SafeArea(
          child: scrollable
              ? SingleChildScrollView(
                  padding: const EdgeInsets.all(16),
                  child: child,
                )
              : Align(
                  alignment: Alignment.topCenter,
                  child: Padding(
                    padding: const EdgeInsets.all(16),
                    child: child,
                  ),
                ),
        ),
      ),
    ),
  );
  await tester.pump();

  final surfaceContext = tester.element(find.byType(Scaffold));
  expect(
    MediaQuery.textScalerOf(surfaceContext).scale(10),
    closeTo(10 * textScale.clamp(0, 1.4), 0.0001),
  );
}
