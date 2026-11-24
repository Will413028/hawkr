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
  // 預設維持 '3001'，既有 case 行為不變；unsupported 那組 case 才覆寫它。
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
              // CodeMeta ShippingMethod value（後端 order-service CheckoutService 以
              // `opt.method().value()` 產出，`HOME_DELIVERY` 是 ShippingType 的
              // code、不是這個軸的值）
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

  // ─────────────────────────────────────────────────────────────────────
  // 未支援配送方式：結帳表單的第二、三道防線
  //
  // 第一道在購物車（`ShippingSection._groupOptions` 直接濾掉，見
  // `shipping_unsupported_filter_test.dart`）。這兩道是 defence in depth——
  // 萬一有路徑把 unsupported 的 option 帶進結帳表單，它必須**明白拒絕**，
  // 而不是帶著錯誤欄位去打 `/app/checkout/preview`。
  //
  // 🔴 為什麼只能在這一層驗：`3003`（Uber 即時配送，已刪除）與 `3005`
  // （商家自行配送）在 dev 上**造不出來**——`chk_shipping_type CHECK
  // (shipping_type IN ('3651','3652'))` 擋住了 ⇒ E2E 與截圖都驗不到。
  //
  // ⚠️ **第三道（`_handleCheckout` 的 unsupported dialog）本檔沒有覆蓋。**
  // 不是漏掉，是它在 unsupported 路徑上**到不了**：`_loadPreview` 在 initState
  // 就設了 `_previewError` 並停在錯誤畫面，使用者按不到送出。
  // 要覆蓋它得先偽造一個「preview 成功但 option 是 unsupported」的狀態，
  // 而那個組合在真實流程裡不存在 ⇒ 測出來的會是一個不會發生的情境。
  // 這裡誠實留白，不用一支到不了的測試充數。
  // ─────────────────────────────────────────────────────────────────────
  testWidgets('unsupported 配送方式在打 preview 之前就被擋下，顯示可行動的錯誤訊息', (tester) async {
    final dio = Dio(BaseOptions(baseUrl: 'https://checkout.test'))
      ..httpClientAdapter = _CheckoutPreviewAdapter();

    await _pumpCheckoutSheet(
      tester,
      dio: dio,
      // 3003：Uber 即時配送（已刪除的功能），App 端無對應 UI。
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

    // ⚠️ 這一條刻意與上一條分開：只測 3003 的話，
    //    「把 3003 寫死成特例」與「走 shippingPresentationOf 的通用判定」
    //    兩種實作都會過，鑑別力為 0。
    expect(find.textContaining('此配送方式暫不支援'), findsWidgets);
    expect(tester.takeException(), isNull);
  });

  testWidgets('supported 的 3001 不會誤觸這條錯誤（陰性對照）', (tester) async {
    final dio = Dio(BaseOptions(baseUrl: 'https://checkout.test'))
      ..httpClientAdapter = _CheckoutPreviewAdapter();

    await _pumpCheckoutSheet(tester, dio: dio);
    await tester.pumpAndSettle();

    // 沒有這一條，上面兩條可能只是在證明「這個畫面永遠顯示那句話」。
    expect(find.textContaining('此配送方式暫不支援'), findsNothing);
  });
}
