import 'dart:convert';
import 'dart:typed_data';
import 'package:hawkr_buyer/core/codemeta/code_meta.dart'
    show ShippingTemperature;
import 'package:hawkr_buyer/features/cart/data/cart_model.dart';
import 'package:hawkr_buyer/features/cart/presentation/widgets/checkout_payment_sheet.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:hawkr_buyer/features/address/data/address_model.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../support/production_layout_test_harness.dart';

class _CheckoutPreviewAdapter implements HttpClientAdapter {
  final List<Map<String, dynamic>> previewBodies = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    if (options.path != '/app/checkout/preview') {
      return ResponseBody.fromString('{}', 404);
    }
    previewBodies.add(Map<String, dynamic>.from(options.data as Map));
    return ResponseBody.fromString(
      jsonEncode({
        'code': 0,
        'data': {
          'checkoutId': 'checkout-layout-test',
          'totalOriginalAmount': '123456789',
          'totalShippingFee': '12345',
          'totalDiscount': '12345678',
          'totalActualAmount': '111123456',
        },
      }),
      200,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

Future<void> _pumpCheckoutSheet(
  WidgetTester tester, {
  required Dio dio,
  String shippingMethod = '3001',
  String deliveryTemp = '3203',
}) {
  return pumpProductionLayoutTestApp(
    tester,
    size: const Size(320, 1000),
    home: Scaffold(
      body: Builder(
        builder: (context) => CheckoutPaymentSheet(
          params: CheckoutParams(
            selectedItems: [
              CartItem(id: 1, skuId: 101, count: 1, price: 123456789),
            ],
            shippingOption: {
              'shippingMethod': shippingMethod,
              'deliveryTemp': deliveryTemp,
            },
            selectedAddress: const Address(
              id: 1,
              townCode: '63000050',
              etag: 'v1',
              receiverName: '測試收件人',
              receiverPhone: '0912345678',
            ),
            selectedStorePickup: null,
            skuProductIds: {101: '9001'},
            deliveryNotesText: '',
            appliedCouponId: 'coupon-1',
            couponDiscount: 12345678,
            canSelectCoupon: true,
          ),
          callbacks: CheckoutCallbacks(
            onShowCouponSheet: () async => null,
            onReloadCart: () {},
            onTriggerShippingFee: () {},
            onClearCoupons: () {},
            onNavigateToOrders: () {},
            onNavigateHome: () {},
            onInvalidateCartCount: () {},
          ),
          dio: dio,
          parentContext: context,
        ),
      ),
    ),
  );
}

void main() {
  testWidgets('preview sends the selected shipping option deliveryTemp as is', (
    tester,
  ) async {
    final adapter = _CheckoutPreviewAdapter();
    final dio = Dio(BaseOptions(baseUrl: 'https://checkout.test'))
      ..httpClientAdapter = adapter;
    addTearDown(dio.close);

    await _pumpCheckoutSheet(
      tester,
      dio: dio,
      deliveryTemp: ShippingTemperature.frozen.value,
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    expect(adapter.previewBodies, isNotEmpty);
    expect(adapter.previewBodies.map((body) => body['deliveryTemp']).toSet(), {
      ShippingTemperature.frozen.value,
    });
  });

  testWidgets('checkout amounts reflow and the payment method list is gone', (
    tester,
  ) async {
    final dio = Dio(BaseOptions(baseUrl: 'https://checkout.test'))
      ..httpClientAdapter = _CheckoutPreviewAdapter();
    addTearDown(dio.close);

    await _pumpCheckoutSheet(tester, dio: dio);
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.text('NT\$ 123456789'), findsOneWidget);
    expect(find.text('NT\$ -12345678'), findsOneWidget);
    expect(find.text('NT\$ 111123456'), findsOneWidget);

    final couponButton = find.ancestor(
      of: find.text('選擇優惠券'),
      matching: find.byType(FilledButton),
    );
    expect(tester.getSize(couponButton).height, greaterThanOrEqualTo(48));
    final couponLabel = tester.widget<Text>(find.text('選擇優惠券'));
    expect(couponLabel.maxLines, 1);
    expect(couponLabel.softWrap, isFalse);

    expect(find.text('付款方式'), findsNothing);
    expect(find.text('ATM 虛擬轉帳'), findsNothing);
    expect(find.text('信用卡'), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets('unsupported 配送方式在打 preview 之前就被擋下，顯示可行動的錯誤訊息', (tester) async {
    final dio = Dio(BaseOptions(baseUrl: 'https://checkout.test'))
      ..httpClientAdapter = _CheckoutPreviewAdapter();

    await _pumpCheckoutSheet(
      tester,
      dio: dio,
      shippingMethod: '3003',
    );
    await tester.pumpAndSettle();

    expect(find.textContaining('此配送方式暫不支援'), findsWidgets);
    expect(tester.takeException(), isNull);
  });

  testWidgets('3005 商家自行配送同樣被擋下（不是只有 3003 被特判）', (tester) async {
    final dio = Dio(BaseOptions(baseUrl: 'https://checkout.test'))
      ..httpClientAdapter = _CheckoutPreviewAdapter();

    await _pumpCheckoutSheet(tester, dio: dio, shippingMethod: '3005');
    await tester.pumpAndSettle();

    expect(find.textContaining('此配送方式暫不支援'), findsWidgets);
    expect(tester.takeException(), isNull);
  });

  testWidgets('supported 的 3001 不會誤觸這條錯誤（陰性對照）', (tester) async {
    final dio = Dio(BaseOptions(baseUrl: 'https://checkout.test'))
      ..httpClientAdapter = _CheckoutPreviewAdapter();

    await _pumpCheckoutSheet(tester, dio: dio);
    await tester.pumpAndSettle();

    expect(find.textContaining('此配送方式暫不支援'), findsNothing);
  });
}
