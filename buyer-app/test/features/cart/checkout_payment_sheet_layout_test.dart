import 'dart:convert';
import 'dart:typed_data';
import 'dart:ui' show CheckedState, SemanticsAction, Tristate;

import 'package:hawkr_buyer/core/codemeta/code_meta.dart';
import 'package:hawkr_buyer/features/cart/data/cart_model.dart';
import 'package:hawkr_buyer/features/cart/presentation/widgets/checkout_payment_sheet.dart';
import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart'
    show debugDefaultTargetPlatformOverride;
import 'package:flutter/material.dart';
import 'package:hawkr_buyer/features/address/data/address_model.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../support/production_layout_test_harness.dart';

class _CheckoutPreviewAdapter implements HttpClientAdapter {
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    if (options.path != '/app/checkout/preview') {
      return ResponseBody.fromString('{}', 404);
    }
    return ResponseBody.fromString(
      jsonEncode({
        'code': 0,
        'data': {
          'checkoutId': 'checkout-layout-test',
          'totalOriginalAmount': '123456789',
          'totalShippingFee': '12345',
          'totalDiscount': '12345678',
          'totalActualAmount': '111123456',
          'availablePaymentMethods': [
            {
              'paymentType': PaymentMethod.applePay.value,
              'eligible': true,
              'ineligibleReason': '',
            },
            {
              'paymentType': PaymentMethod.atm.value,
              'eligible': true,
              'ineligibleReason': '',
            },
            {
              'paymentType': PaymentMethod.creditCard.value,
              'eligible': false,
              'ineligibleReason':
                  PaymentMethodIneligibleReason.amountAboveMax.value,
            },
          ],
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
  required ValueChanged<String> onPayMethodChanged,
  // 預設維持 '3001'，既有 case 行為不變；unsupported 那組 case 才覆寫它。
  String shippingMethod = '3001',
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
              'deliveryTemp': '3203',
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
            onNavigateToPaymentWebview:
                ({
                  required String redirectUrl,
                  required String tradeNo,
                  required String orderNo,
                  required String amount,
                }) {},
            onInvalidateCartCount: () {},
            onPayMethodChanged: onPayMethodChanged,
          ),
          dio: dio,
          parentContext: context,
        ),
      ),
    ),
  );
}

void main() {
  testWidgets(
    'checkout amounts reflow and payment methods expose selected radio semantics',
    (tester) async {
      final semantics = tester.ensureSemantics();
      final dio = Dio(BaseOptions(baseUrl: 'https://checkout.test'))
        ..httpClientAdapter = _CheckoutPreviewAdapter();
      addTearDown(dio.close);
      String? selectedPayType;

      try {
        await _pumpCheckoutSheet(
          tester,
          dio: dio,
          onPayMethodChanged: (value) => selectedPayType = value,
        );
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

        // 預設平台（測試環境是 android）也只有 Apple Pay：App 只發 iOS。
        expect(find.bySemanticsLabel('Google Pay'), findsNothing);
        final applePayTarget = find.bySemanticsLabel('Apple Pay');
        final applePaySemantics = tester
            .getSemantics(applePayTarget)
            .getSemanticsData();
        expect(applePaySemantics.flagsCollection.isSelected, Tristate.isTrue);
        expect(applePaySemantics.flagsCollection.isChecked, CheckedState.none);
        expect(
          applePaySemantics.flagsCollection.isInMutuallyExclusiveGroup,
          isTrue,
        );
        expect(applePaySemantics.flagsCollection.isButton, isTrue);

        final atmTarget = find.bySemanticsLabel('ATM 虛擬轉帳');
        expect(atmTarget, findsOneWidget);
        expect(tester.getSize(atmTarget).height, greaterThanOrEqualTo(48));
        var atmSemantics = tester.getSemantics(atmTarget).getSemanticsData();
        expect(atmSemantics.flagsCollection.isSelected, Tristate.isFalse);
        expect(atmSemantics.flagsCollection.isInMutuallyExclusiveGroup, isTrue);
        final atmLabel = tester.widget<Text>(find.text('ATM 虛擬轉帳'));
        expect(atmLabel.maxLines, 1);
        expect(atmLabel.softWrap, isFalse);

        final creditCardTarget = find.bySemanticsLabel('信用卡');
        final creditCardSemantics = tester
            .getSemantics(creditCardTarget)
            .getSemanticsData();
        expect(creditCardSemantics.flagsCollection.isEnabled, Tristate.isFalse);
        expect(
          creditCardSemantics.flagsCollection.isSelected,
          Tristate.isFalse,
        );
        expect(
          creditCardSemantics.flagsCollection.isInMutuallyExclusiveGroup,
          isTrue,
        );
        expect(creditCardSemantics.hasAction(SemanticsAction.tap), isFalse);
        expect(find.text('金額超過此付款方式上限'), findsOneWidget);
        await tester.tap(find.text('信用卡'));
        await tester.pump();
        expect(selectedPayType, isNull);

        await tester.tap(atmTarget);
        await tester.pump();
        expect(selectedPayType, PaymentMethod.atm.value);
        atmSemantics = tester.getSemantics(atmTarget).getSemanticsData();
        expect(atmSemantics.flagsCollection.isSelected, Tristate.isTrue);
        expect(
          tester
              .getSemantics(applePayTarget)
              .getSemanticsData()
              .flagsCollection
              .isSelected,
          Tristate.isFalse,
        );
        expect(tester.takeException(), isNull);
      } finally {
        semantics.dispose();
      }
    },
  );

  testWidgets('payment methods expose selected radio-group semantics on iOS', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    debugDefaultTargetPlatformOverride = TargetPlatform.iOS;
    final dio = Dio(BaseOptions(baseUrl: 'https://checkout.test'))
      ..httpClientAdapter = _CheckoutPreviewAdapter();
    addTearDown(dio.close);

    try {
      await _pumpCheckoutSheet(tester, dio: dio, onPayMethodChanged: (_) {});
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 100));

      final applePaySemantics = tester
          .getSemantics(find.bySemanticsLabel('Apple Pay'))
          .getSemanticsData();
      expect(applePaySemantics.flagsCollection.isSelected, Tristate.isTrue);
      expect(applePaySemantics.flagsCollection.isChecked, CheckedState.none);
      expect(
        applePaySemantics.flagsCollection.isInMutuallyExclusiveGroup,
        isTrue,
      );

      final atmSemantics = tester
          .getSemantics(find.bySemanticsLabel('ATM 虛擬轉帳'))
          .getSemanticsData();
      expect(atmSemantics.flagsCollection.isSelected, Tristate.isFalse);
      expect(atmSemantics.flagsCollection.isChecked, CheckedState.none);
      expect(atmSemantics.flagsCollection.isInMutuallyExclusiveGroup, isTrue);
      expect(tester.takeException(), isNull);
    } finally {
      debugDefaultTargetPlatformOverride = null;
      semantics.dispose();
    }
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
      onPayMethodChanged: (_) {},
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

    await _pumpCheckoutSheet(
      tester,
      dio: dio,
      onPayMethodChanged: (_) {},
      shippingMethod: '3005',
    );
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

    await _pumpCheckoutSheet(tester, dio: dio, onPayMethodChanged: (_) {});
    await tester.pumpAndSettle();

    // 沒有這一條，上面兩條可能只是在證明「這個畫面永遠顯示那句話」。
    expect(find.textContaining('此配送方式暫不支援'), findsNothing);
  });
}
