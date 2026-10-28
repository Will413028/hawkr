import 'package:hawkr_seller/core/codemeta/shipping_presentation.dart';
import 'package:hawkr_seller/features/order/data/order_model.dart';
import 'package:hawkr_seller/features/order/data/payuni_logistics_service.dart';
import 'package:hawkr_seller/features/order/presentation/order_card_widgets/order_expanded_section.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';

/// 賣家端物流 provider 分派契約（移除二元 fallback）。
///
/// 鑑別力來源：**未知 `shippingMethod` 不得被畫成超商段**。把
/// `OrderExpandedSection._buildInfoContent` 改回 `if (!isTaqbin) 超商段`，
/// 下面「3003/3005/未知不得出現出貨便代碼」那幾條會立刻轉紅。
OrderSummary _order(String shippingMethod) => OrderSummary(
  id: 'O-TEST-1',
  orderSn: 'O-TEST-1',
  statusCode: 20,
  title: '待出貨',
  subtitle: '20',
  dateTime: '2026-09-10T00:00:00Z',
  productName: '測試商品',
  price: '100',
  shippingMethod: shippingMethod,
  // 超商段的判定依據：odno 有值就會秀「出貨便代碼」。
  odno: 'ODNO1234',
  validationNo: '5678',
  shipmentStatus: '91',
);

Future<void> _pump(WidgetTester tester, OrderSummary order) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = const Size(390, 1200);
  addTearDown(tester.view.resetDevicePixelRatio);
  addTearDown(tester.view.resetPhysicalSize);
  await tester.pumpWidget(
    ScreenUtilInit(
      designSize: const Size(390, 844),
      builder: (_, _) => MaterialApp(
        home: Scaffold(
          body: SingleChildScrollView(
            child: OrderExpandedSection(
              order: order,
              onCancelShipment: () {},
              onDownloadPdf: () {},
            ),
          ),
        ),
      ),
    ),
  );
  await tester.pump();
}

void main() {
  group('OrderExpandedSection：出貨資訊依配送方式明確分派', () {
    testWidgets('3002 超商 → 顯示出貨便代碼（既有行為不變）', (tester) async {
      await _pump(tester, _order('3002'));
      expect(find.textContaining('出貨便代碼'), findsOneWidget);
    });

    for (final unknown in const ['3003', '3005', '', '9999']) {
      testWidgets(
        'shippingMethod=${unknown.isEmpty ? "(空)" : unknown} → 不得走超商段',
        (tester) async {
          await _pump(tester, _order(unknown));
          // ① 不得顯示超商專屬欄位
          expect(find.textContaining('出貨便代碼'), findsNothing);
          // ② 不得憑空消失：仍要有 provider-neutral 的狀態文案
          expect(find.text('訂單處理中'), findsOneWidget);
        },
      );
    }

    testWidgets('3001 宅配 → 不得走超商段', (tester) async {
      await _pump(tester, _order('3001'));
      expect(find.textContaining('出貨便代碼'), findsNothing);
    });
  });

  group('LogisticsStatusCode.getProgressPercent：lgsType 逐值分派', () {
    test('HOME / C2C 維持既有百分比', () {
      expect(
        LogisticsStatusCode.getProgressPercent(
          31,
          PayuniLogisticsService.lgsTypeHOME,
        ),
        60,
      );
      expect(
        LogisticsStatusCode.getProgressPercent(
          11,
          PayuniLogisticsService.lgsTypeC2C,
        ),
        80,
      );
    });

    test('B2C / C2B / 空字串 / 未知 → null（不得回 C2C 的百分比）', () {
      // C2C 對 code=11 回 80；若哪天又退回「不是 HOME 就當超商」，
      // 下面四條會全部拿到 80 而轉紅。
      for (final lgsType in const [
        PayuniLogisticsService.lgsTypeB2C,
        PayuniLogisticsService.lgsTypeC2B,
        '',
        'WHATEVER',
      ]) {
        expect(
          LogisticsStatusCode.getProgressPercent(11, lgsType),
          isNull,
          reason: 'lgsType=$lgsType 借用了別條流程的進度表',
        );
      }
    });
  });

  group('ShippingPresentation 與賣家端的支援範圍', () {
    test('只有 3001 / 3002 有專屬出貨 UI', () {
      expect(shippingPresentationOf('3001').isSupported, isTrue);
      expect(shippingPresentationOf('3002').isSupported, isTrue);
      for (final v in const ['3003', '3005', '', '9999']) {
        expect(shippingPresentationOf(v).isSupported, isFalse);
      }
    });
  });
}
