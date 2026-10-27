import 'dart:io';
import 'dart:ui' as ui;

import 'package:hawkr_buyer/core/theme/app_theme.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/features/cart/presentation/widgets/cart_checkout_action.dart';
import 'package:hawkr_buyer/features/cart/presentation/widgets/delivery_notes_section.dart';
import 'package:hawkr_buyer/features/cart/presentation/widgets/payment_details_section.dart';
import 'package:hawkr_buyer/features/cart/presentation/widgets/shipping_section.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:hawkr_buyer/features/address/data/address_model.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../support/production_layout_test_harness.dart';

void main() {
  for (final width in [320.0, 390.0, 768.0]) {
    for (final scale in [1.0, 1.4]) {
      testWidgets(
        'cart controls are readable and actionable at $width / $scale',
        (tester) async {
          final notes = TextEditingController();
          addTearDown(notes.dispose);
          final captureKey = GlobalKey();
          var edits = 0;
          var additions = 0;
          var coupons = 0;
          var checkouts = 0;
          await _pump(
            tester,
            width: width,
            scale: scale,
            child: RepaintBoundary(
              key: captureKey,
              child: Column(
                children: [
                  _shipping(onEdit: () => edits++, onAdd: () => additions++),
                  DeliveryNotesSection(controller: notes),
                  PaymentDetailsSection(
                    productTotal: 1360,
                    shippingTotal: 120,
                    couponDiscount: 0,
                    grandTotal: 1480,
                    canSelectCoupon: true,
                    onSelectCoupon: () => coupons++,
                  ),
                  CartCheckoutAction(onPressed: () => checkouts++),
                ],
              ),
            ),
          );
          expect(tester.takeException(), isNull);
          if (width == 320 || (width == 390 && scale > 1)) {
            expect(
              tester.getTopLeft(find.text('NT\$ 1360')).dy,
              greaterThanOrEqualTo(tester.getBottomLeft(find.text('商品總金額')).dy),
            );
            expect(
              tester.getTopLeft(find.text('NT\$ 120').first).dy,
              greaterThanOrEqualTo(
                tester.getBottomLeft(find.text('黑貓宅急便').last).dy,
              ),
            );
          }
          for (final label in ['編輯地址', '新增住址', '選擇優惠券', '前往結帳']) {
            final button = find.ancestor(
              of: find.text(label),
              matching: find.byWidgetPredicate((w) => w is ButtonStyleButton),
            );
            expect(
              tester.getSize(button).height,
              greaterThanOrEqualTo(HkControl.minTap),
            );
            await tester.ensureVisible(button);
            await tester.tap(button);
            await tester.pump();
          }
          expect([edits, additions, coupons, checkouts], [1, 1, 1, 1]);
          expect(tester.widget<Text>(find.text('寄送方式')).style!.fontSize, 22);
          expect(tester.widget<Text>(find.text('商品總金額')).style!.fontSize, 16);
          expect(
            tester.widget<TextField>(find.byType(TextField)).style!.fontSize,
            16,
          );
          await tester.ensureVisible(find.byType(TextField));
          await tester.enterText(find.byType(TextField), '請放管理室，謝謝');
          expect(notes.text, '請放管理室，謝謝');
          FocusManager.instance.primaryFocus?.unfocus();
          await tester.pump();
          expect(tester.takeException(), isNull);

          const captureDir = String.fromEnvironment('CART_UI_CAPTURE_DIR');
          if (captureDir.isNotEmpty) {
            await tester.runAsync(() async {
              final boundary =
                  captureKey.currentContext!.findRenderObject()!
                      as RenderRepaintBoundary;
              final image = await boundary.toImage(pixelRatio: 1);
              final data = await image.toByteData(
                format: ui.ImageByteFormat.png,
              );
              await Directory(captureDir).create(recursive: true);
              await File(
                '$captureDir/buyer-cart-${width.toInt()}-$scale.png',
              ).writeAsBytes(data!.buffer.asUint8List());
              image.dispose();
            });
          }
        },
      );
    }
  }

  testWidgets('store editing does not toggle the selected pickup', (
    tester,
  ) async {
    var selected = 0;
    Map<String, dynamic>? edited;
    var additions = 0;
    final semantics = tester.ensureSemantics();
    await _pump(
      tester,
      width: 320,
      scale: 1.4,
      child: _shipping(
        method: '3002',
        onStoreSelected: (_) => selected++,
        onStoreEdit: (store) {
          if (store == null) {
            additions++;
          } else {
            edited = store;
          }
        },
      ),
    );
    final edit = find.byKey(const ValueKey('cart-edit-store-demo'));
    expect(tester.getSize(edit).height, greaterThanOrEqualTo(48));
    await tester.tap(edit);
    await tester.pump();
    expect(edited?['id'], 'demo');
    expect(selected, 0);
    await tester.tap(find.text('新增 7-11 門市'));
    await tester.pump();
    expect(additions, 1);
    await expectLater(tester, meetsGuideline(androidTapTargetGuideline));
    await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));
    semantics.dispose();
    expect(tester.takeException(), isNull);
  });

  testWidgets(
    'retry has a usable touch target and disabled coupon stays inert',
    (tester) async {
      var retries = 0;
      var coupons = 0;
      await _pump(
        tester,
        child: Column(
          children: [
            _shipping(empty: true, onRetry: () => retries++),
            PaymentDetailsSection(
              productTotal: 0,
              shippingTotal: 0,
              couponDiscount: 0,
              grandTotal: 0,
              canSelectCoupon: false,
              onSelectCoupon: () => coupons++,
            ),
          ],
        ),
      );
      final retry = find.widgetWithText(TextButton, '重試');
      expect(tester.getSize(retry).height, greaterThanOrEqualTo(48));
      await tester.tap(retry);
      await tester.tap(find.text('選擇優惠券'));
      await tester.pump();
      expect(retries, 1);
      expect(coupons, 0);
    },
  );

  for (final isValidating in [false, true]) {
    testWidgets(
      'checkout remains inert when ${isValidating ? 'validating' : 'unselected'} by tap and swipe',
      (tester) async {
        var opened = 0;
        await _pump(
          tester,
          width: 320,
          scale: 1.4,
          child: CartCheckoutAction(
            onPressed: isValidating ? () => opened++ : null,
            storeName: isValidating ? '範例海鮮店' : null,
            selectedItemCount: isValidating ? 1 : 0,
            isValidating: isValidating,
          ),
        );
        expect(
          find.text(isValidating ? '正在確認商品與配送方式…' : '請先選取一家店的商品'),
          findsOneWidget,
        );
        final button = find.byKey(const ValueKey('cart-checkout-action'));
        expect(tester.widget<FilledButton>(button).onPressed, isNull);
        expect(tester.getSize(button).height, greaterThanOrEqualTo(48));
        await tester.tap(button);
        await tester.drag(
          find.byType(CartCheckoutAction),
          const Offset(0, -80),
        );
        await tester.pump();
        expect(opened, 0);
        expect(tester.takeException(), isNull);
      },
    );
  }

  testWidgets('checkout opens the same review action by tap and upward swipe', (
    tester,
  ) async {
    var opened = 0;
    await _pump(tester, child: CartCheckoutAction(onPressed: () => opened++));
    await tester.tap(find.text('前往結帳'));
    await tester.pump();
    expect(opened, 1);
    await tester.drag(find.byType(CartCheckoutAction), const Offset(0, -80));
    await tester.pump();
    expect(opened, 2);
    expect(tester.takeException(), isNull);
  });
}

Future<void> _pump(
  WidgetTester tester, {
  double width = 390,
  double scale = 1,
  required Widget child,
}) => pumpProductionLayoutTestApp(
  tester,
  size: Size(width, 1200),
  systemTextScale: scale,
  home: Theme(
    data: AppTheme.light,
    child: Scaffold(body: SingleChildScrollView(child: child)),
  ),
);

Widget _shipping({
  String method = '3001',
  bool empty = false,
  VoidCallback? onEdit,
  VoidCallback? onAdd,
  VoidCallback? onRetry,
  ValueChanged<String>? onStoreSelected,
  void Function(Map<String, dynamic>?)? onStoreEdit,
}) => ShippingSection(
  shippingOptions: empty
      ? const []
      : [
          {
            'shippingMethod': method,
            'label': method == '3001' ? '黑貓宅急便（常溫）' : '7-11 超商取貨（常溫）',
            'deliveryTemp': '3201',
            'fee': '120',
            'estimatedDelivery': '依訂單確認',
          },
        ],
  selectedShippingIdx: 0,
  validateResult: const {'valid': true, 'errors': []},
  validateLoading: false,
  selectedAddress: const Address(
    etag: 'v1',
    id: 1,
    townCode: '63000050',
    addressDetail: '範例路一段 123 號 8 樓',
    receiverName: '範例收件人',
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
  storePickups: const [
    {
      'id': 'demo',
      'storeName': '範例門市',
      'storeAddress': '台北市中正區範例路 123 號',
      'receiverName': '範例收件人',
      'receiverPhone': '0912345678',
    },
  ],
  selectedStorePickupId: 'demo',
  checkedSkuIds: const {101},
  onShippingIdxChanged: (_) {},
  onStorePickupSelected: onStoreSelected ?? (_) {},
  onExpandedGroupsChanged: (_) {},
  onShowAddressPicker: onEdit ?? () {},
  onOpenStorePickupForm: onStoreEdit ?? (_) {},
  onAddAddress: onAdd ?? () {},
  onRetryValidate: onRetry ?? () {},
);
