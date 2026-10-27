import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/features/order/data/after_sale_model.dart';
import 'package:hawkr_buyer/features/order/presentation/widgets/return_logistics_timeline.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';

/// 退貨物流 timeline 的 provider 分派契約（移除二元 fallback）。
///
/// 這支測試的鑑別力來源：**未知 `returnShippingMethod` 不得被畫成任何一家物流商**。
/// 把 `ReturnLogisticsTimeline.build` 改回 `_isCvs ? 超商 : 黑貓` 之後，
/// 「3003/3004/3005/空值不得出現黑貓字樣」那幾條會立刻轉紅。
Future<void> _pump(WidgetTester tester, AfterSaleCase c) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = const Size(390, 1400);
  addTearDown(tester.view.resetDevicePixelRatio);
  addTearDown(tester.view.resetPhysicalSize);
  await tester.pumpWidget(
    ScreenUtilInit(
      designSize: const Size(390, 844),
      builder: (_, _) => MaterialApp(
        home: Scaffold(
          body: SingleChildScrollView(
            child: ReturnLogisticsTimeline(caseData: c),
          ),
        ),
      ),
    ),
  );
  await tester.pump();
}

AfterSaleCase _caseWith(String returnShippingMethod) => AfterSaleCase(
  caseNo: 'AS-TEST-1',
  status: '25',
  returnShippingMethod: returnShippingMethod,
  returnShipmentNo: 'RS12345678',
);

void main() {
  setUpAll(() {
    if (!F.isInitialized) F.init(Flavor.dev);
  });

  testWidgets('3001 → 黑貓宅配收件流程（既有行為不變）', (tester) async {
    await _pump(tester, _caseWith('3001'));
    expect(find.text('黑貓宅配收件流程'), findsOneWidget);
    expect(find.text('司機到府收件'), findsOneWidget);
  });

  testWidgets('3002 → 7-11 退貨便流程（既有行為不變）', (tester) async {
    await _pump(tester, _caseWith('3002'));
    expect(find.text('7-11 退貨便流程'), findsOneWidget);
    expect(find.text('交付門市人員'), findsOneWidget);
  });

  for (final unknown in const ['3003', '3004', '3005', '', '9999']) {
    testWidgets('returnShippingMethod=${unknown.isEmpty ? "(空)" : unknown} '
        '→ 不得畫成黑貓、也不得畫成 7-11', (tester) async {
      await _pump(tester, _caseWith(unknown));

      // ① 不得掛上任一家物流商的標題
      expect(find.text('黑貓宅配收件流程'), findsNothing);
      expect(find.text('7-11 退貨便流程'), findsNothing);
      // ② 不得出現物流商專屬步驟文案
      expect(find.text('黑貓確認派車'), findsNothing);
      expect(find.text('司機到府收件'), findsNothing);
      expect(find.text('至門市 ibon 機台選擇「退貨便」'), findsNothing);
      expect(find.text('交付門市人員'), findsNothing);
      // ③ 不得憑空消失：仍要有 provider-neutral 的進度可看
      expect(find.text('退貨物流進度'), findsOneWidget);
      expect(find.text('退貨運送中'), findsOneWidget);
      expect(find.text('賣家已收到退貨'), findsOneWidget);
    });
  }
}
