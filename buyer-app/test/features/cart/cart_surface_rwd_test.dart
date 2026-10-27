import 'package:hawkr_buyer/features/cart/data/cart_model.dart';
import 'package:hawkr_buyer/features/cart/presentation/widgets/cart_product_tile.dart';
import 'package:hawkr_buyer/features/cart/presentation/widgets/merchant_group.dart';
import 'package:hawkr_buyer/features/cart/presentation/widgets/merchant_group_card.dart';
import 'package:hawkr_buyer/features/cart/presentation/widgets/payment_details_section.dart';
import 'package:hawkr_buyer/features/cart/presentation/widgets/shipping_section.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:hawkr_buyer/core/theme/app_theme.dart';
import 'package:hawkr_buyer/features/address/data/address_model.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../support/production_layout_test_harness.dart';

void main() {
  for (final width in <double>[320, 390, 768]) {
    testWidgets(
      'product tile keeps full commerce details at ${width.toInt()}px and '
      '2.0x system / 1.4x effective text',
      (tester) async {
        final semantics = tester.ensureSemantics();
        try {
          var selected = true;
          var quantity = _firstItem.count;
          await _pumpSurface(
            tester,
            width: width,
            child: StatefulBuilder(
              builder: (context, setState) => Padding(
                padding: const EdgeInsets.symmetric(horizontal: 16),
                child: CartProductTile(
                  item: _firstItem.copyWith(count: quantity),
                  isChecked: selected,
                  remainingStock: 99,
                  itemDeliveryTemps: const {
                    101: ['3203'],
                  },
                  onCheckChanged: (value) => setState(() => selected = value),
                  onQuantityChanged: (value) =>
                      setState(() => quantity = value),
                ),
              ),
            ),
          );

          expect(find.text('台東嚴選茂谷柑禮盒'), findsOneWidget);
          expect(find.text('規格：2.5 公斤／精裝禮盒'), findsOneWidget);
          expect(find.text('NT\$999'), findsOneWidget);
          expect(find.text('NT\$1299'), findsOneWidget);
          expect(find.text('庫存 99 件'), findsOneWidget);
          expect(find.text('冷凍'), findsOneWidget);

          final selection = find.bySemanticsLabel('取消選取台東嚴選茂谷柑禮盒');
          final selectionSize = tester.getSize(selection);
          expect(selectionSize.width, greaterThanOrEqualTo(48));
          expect(selectionSize.height, greaterThanOrEqualTo(48));
          expect(
            tester.getSize(find.bySemanticsLabel('增加數量')),
            const Size.square(48),
          );
          for (final symbol in <String>['minus', 'plus']) {
            expect(
              tester.getSize(
                find.byKey(ValueKey('quantity-stepper-$symbol-visual')),
              ),
              const Size.square(32),
            );
          }
          if (width == 390) {
            await expectLater(
              tester,
              meetsGuideline(androidTapTargetGuideline),
            );
            await expectLater(tester, meetsGuideline(iOSTapTargetGuideline));
            await expectLater(
              tester,
              meetsGuideline(labeledTapTargetGuideline),
            );
          }
          expect(tester.takeException(), isNull);
        } finally {
          semantics.dispose();
        }
      },
    );
  }

  for (final width in [320.0, 390.0]) {
    for (final count in [1, 999]) {
      testWidgets(
        'compact merchant keeps store link and quantity inline at $width / $count',
        (tester) async {
          final semantics = tester.ensureSemantics();
          try {
            var storeOpens = 0;
            var selectionChanges = 0;
            int? nextQuantity;
            await _pumpSurface(
              tester,
              width: width,
              // Single quantities also cover the embedded catalog's extra
              // gutter; large quantities cover the production cart container.
              child: Padding(
                padding: EdgeInsets.symmetric(horizontal: count == 1 ? 16 : 0),
                child: MerchantGroupCard(
                  group: MerchantGroup(
                    storeId: 'compact',
                    sellerName: '金湖水產',
                    sellerCity: '新北市',
                    sellerDistrict: '中和區',
                    items: [_firstItem.copyWith(count: count)],
                  ),
                  checkedSkuIds: const {101},
                  skuStock: const {101: 2000},
                  itemDeliveryTemps: const {
                    101: ['3203'],
                  },
                  onGroupCheckChanged: (_, _) => selectionChanges++,
                  onItemCheckChanged: (_, _) => selectionChanges++,
                  onQuantityChanged: (_, value) => nextQuantity = value,
                  onDeleteItem: (_) {},
                  resolveImageProvider: (_) => null,
                  sellerAvatarUrl: (_, {groupLogoUrl}) => null,
                  onMerchantTap: () => storeOpens++,
                ),
              ),
            );
            final store = find.byKey(const ValueKey('cart-store-link-compact'));
            final storeRect = tester.getRect(store);
            final nameRect = tester.getRect(
              find.byKey(const ValueKey('cart-store-name-compact')),
            );
            expect(find.text('賣家商店'), findsOneWidget);
            expect(storeRect.left, greaterThanOrEqualTo(nameRect.right));
            expect(storeRect.top, lessThan(nameRect.bottom));
            expect(storeRect.bottom, greaterThan(nameRect.top));
            expect(storeRect.height, greaterThanOrEqualTo(48));
            await tester.tap(store);
            expect(storeOpens, 1);
            expect(selectionChanges, 0);

            final photo = tester.getRect(
              find.byKey(const ValueKey('cart-product-image-101')),
            );
            final quantity = tester.getRect(
              find.byKey(const ValueKey('quantity-stepper')),
            );
            expect(photo.width, greaterThanOrEqualTo(48));
            expect(quantity.left, greaterThanOrEqualTo(photo.right));
            expect(quantity.top, lessThan(photo.bottom));
            expect(quantity.bottom, greaterThan(photo.top));
            expect(quantity.height, greaterThanOrEqualTo(48));
            for (final label in ['台東嚴選茂谷柑禮盒', '規格：2.5 公斤／精裝禮盒']) {
              expect(
                tester
                    .renderObject<RenderParagraph>(find.text(label))
                    .didExceedMaxLines,
                false,
              );
            }
            final plus = find.bySemanticsLabel('增加數量');
            await tester.ensureVisible(plus);
            await tester.tap(plus);
            expect(nextQuantity, count + 1);
            expect(selectionChanges, 0);
            expect(find.text('冷凍'), findsOneWidget);
            expect(tester.takeException(), isNull);
          } finally {
            semantics.dispose();
          }
        },
      );
    }
  }

  for (final count in [999, 99999, 999999999]) {
    for (final purchasable in [true, false]) {
      testWidgets(
        'narrow cart preserves large count $count and availability $purchasable',
        (tester) async {
          final semantics = tester.ensureSemantics();
          try {
            var quantityChanges = 0;
            var selectionChanges = 0;
            await _pumpSurface(
              tester,
              width: 320,
              child: Padding(
                padding: const EdgeInsets.symmetric(horizontal: 32),
                child: CartProductTile(
                  item: _firstItem.copyWith(
                    count: count,
                    purchasable: purchasable,
                    unavailableReason: purchasable ? null : 'OUT_OF_STOCK',
                  ),
                  isChecked: purchasable,
                  remainingStock: purchasable ? count + 1 : 0,
                  itemDeliveryTemps: const {
                    101: ['3203'],
                  },
                  onCheckChanged: (_) => selectionChanges++,
                  onQuantityChanged: (_) => quantityChanges++,
                ),
              ),
            );

            for (final label in ['台東嚴選茂谷柑禮盒', '規格：2.5 公斤／精裝禮盒', '$count']) {
              expect(
                tester
                    .renderObject<RenderParagraph>(find.text(label))
                    .didExceedMaxLines,
                isFalse,
              );
            }
            final photo = tester.getRect(
              find.byKey(const ValueKey('cart-product-image-101')),
            );
            final quantity = tester.getRect(
              find.byKey(const ValueKey('quantity-stepper')),
            );
            if (count > 999) {
              expect(quantity.top, greaterThanOrEqualTo(photo.bottom));
              expect(quantity.width, lessThanOrEqualTo(256));
            }
            if (!purchasable) {
              final badge = tester.getRect(
                find.byKey(const ValueKey('cart-product-unavailable-101')),
              );
              final title = tester.getRect(find.text('台東嚴選茂谷柑禮盒'));
              expect(badge.bottom, lessThanOrEqualTo(title.top));
              expect(find.bySemanticsLabel('已售完'), findsOneWidget);
            }
            final plus = find.bySemanticsLabel('增加數量');
            final minus = find.bySemanticsLabel('減少數量');
            expect(tester.getSize(plus), const Size.square(48));
            expect(tester.getSize(minus), const Size.square(48));
            await tester.ensureVisible(plus);
            await tester.tap(plus);
            await tester.tap(minus);
            expect(quantityChanges, purchasable ? 2 : 0);
            expect(selectionChanges, 0);
            expect(tester.takeException(), isNull);
          } finally {
            semantics.dispose();
          }
        },
      );
    }
  }

  testWidgets(
    'merchant selection uses an indeterminate state for partial cart',
    (tester) async {
      await _pumpSurface(
        tester,
        width: 390,
        child: MerchantGroupCard(
          group: _merchantGroup,
          checkedSkuIds: const {101},
          skuStock: const {101: 99, 102: 93},
          itemDeliveryTemps: const {
            101: ['3203'],
            102: ['3203'],
          },
          onGroupCheckChanged: (_, _) {},
          onItemCheckChanged: (_, _) {},
          onQuantityChanged: (_, _) {},
          onDeleteItem: (_) {},
          resolveImageProvider: (_) => null,
          sellerAvatarUrl: (_, {groupLogoUrl}) => null,
        ),
      );

      final checkboxes = tester.widgetList<Checkbox>(find.byType(Checkbox));
      expect(checkboxes.first.value, isNull);
      expect(find.text('NT\$999'), findsOneWidget);
      expect(find.text('NT\$680'), findsOneWidget);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets(
    'missing address is one actionable state without duplicate copy',
    (tester) async {
      var addAddressTapped = false;
      final semantics = tester.ensureSemantics();
      try {
        await _pumpSurface(
          tester,
          width: 390,
          child: ShippingSection(
            shippingOptions: const [],
            selectedShippingIdx: 0,
            validateResult: const {
              'valid': false,
              'errors': [
                {'errorCode': 'ADDRESS_REQUIRED'},
              ],
            },
            validateLoading: false,
            selectedAddress: null,
            userAddresses: const [],
            storePickups: const [],
            selectedStorePickupId: null,
            checkedSkuIds: const {101},
            onShippingIdxChanged: (_) {},
            onStorePickupSelected: (_) {},
            onExpandedGroupsChanged: (_) {},
            onShowAddressPicker: () {},
            onOpenStorePickupForm: (_) {},
            onAddAddress: () => addAddressTapped = true,
            onRetryValidate: () {},
          ),
        );

        expect(find.text('新增收貨地址'), findsOneWidget);
        expect(find.textContaining('請先設定收貨地址'), findsNothing);
        expect(
          find.byKey(const ValueKey('shipping-address-required')),
          findsOne,
        );
        final action = find.bySemanticsLabel('新增收貨地址');
        expect(tester.getSize(action).height, greaterThanOrEqualTo(48));

        await tester.tap(action);
        await tester.pump();
        expect(addAddressTapped, isTrue);
        expect(tester.takeException(), isNull);
      } finally {
        semantics.dispose();
      }
    },
  );

  testWidgets('long shipping option wraps safely on a compact screen', (
    tester,
  ) async {
    await _pumpSurface(
      tester,
      width: 320,
      child: ShippingSection(
        shippingOptions: const [
          {
            // 長 label 的 RWD 測試；method 用真實 CodeMeta value，
            // 未支援的碼值不會出現在選單裡（見 ShippingSection._groupOptions）
            'shippingMethod': '3001',
            'label': '產地低溫專車配送（冷凍）',
            'deliveryTemp': '3203',
            'fee': '12345',
            'freeShippingEligible': false,
          },
        ],
        selectedShippingIdx: 0,
        validateResult: const {'valid': true, 'errors': []},
        validateLoading: false,
        selectedAddress: const Address(
          etag: 'v1',
          id: 1,
          townCode: '63000050',
          addressDetail: '忠孝西路一段超長地址測試',
          receiverName: '王小明',
          receiverPhone: '0912345678',
        ),
        userAddresses: const [
          Address(
            id: 1,
            etag: 'v1',
            receiverName: '測試收件人',
            receiverPhone: '0912345678',
          ),
        ],
        storePickups: const [],
        selectedStorePickupId: null,
        checkedSkuIds: const {101},
        onShippingIdxChanged: (_) {},
        onStorePickupSelected: (_) {},
        onExpandedGroupsChanged: (_) {},
        onShowAddressPicker: () {},
        onOpenStorePickupForm: (_) {},
        onAddAddress: () {},
        onRetryValidate: () {},
      ),
    );

    expect(find.text('產地低溫專車配送'), findsWidgets);
    expect(find.text('冷凍'), findsOneWidget);
    expect(find.text('NT\$ 12345'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets(
    'payment amounts and coupon action reflow at 320px and 1.4x text',
    (tester) async {
      await _pumpSurface(
        tester,
        width: 320,
        child: PaymentDetailsSection(
          productTotal: 123456789,
          shippingTotal: 12345,
          couponDiscount: 12345678,
          grandTotal: 111123456,
          canSelectCoupon: true,
          onSelectCoupon: () {},
        ),
      );

      expect(find.text('NT\$ 123456789'), findsOneWidget);
      expect(find.text('-NT\$ 12345678'), findsOneWidget);
      expect(find.text('NT\$ 111123456'), findsOneWidget);

      final couponButton = find.ancestor(
        of: find.text('選擇優惠券'),
        matching: find.byType(OutlinedButton),
      );
      expect(couponButton, findsOneWidget);
      expect(tester.getSize(couponButton).height, greaterThanOrEqualTo(48));
      final couponLabel = tester.widget<Text>(find.text('選擇優惠券'));
      expect(couponLabel.maxLines, 1);
      expect(couponLabel.softWrap, isFalse);
      expect(tester.takeException(), isNull);
    },
  );
}

Future<void> _pumpSurface(
  WidgetTester tester, {
  required double width,
  required Widget child,
}) async {
  await pumpProductionLayoutTestApp(
    tester,
    size: Size(width, width >= 600 ? 1100 : 1000),
    home: Theme(
      data: AppTheme.light,
      child: Scaffold(body: SingleChildScrollView(child: child)),
    ),
  );

  final surfaceContext = tester.element(find.byType(Scaffold));
  expect(MediaQuery.textScalerOf(surfaceContext).scale(10), 14);
}

const _firstItem = CartItem(
  id: 1,
  skuId: 101,
  count: 7,
  price: 999,
  listPrice: 1299,
  title: '台東嚴選茂谷柑禮盒',
  specName: '2.5 公斤／精裝禮盒',
);

const _secondItem = CartItem(
  id: 2,
  skuId: 102,
  count: 1,
  price: 680,
  title: '蟹味棒',
  specName: '約 25 公克 × 12 入',
);

final _merchantGroup = MerchantGroup(
  storeId: 'merchant-1',
  sellerName: '金湖水產',
  items: const [_firstItem, _secondItem],
  sellerCity: '新北市',
  sellerDistrict: '中和區',
);
