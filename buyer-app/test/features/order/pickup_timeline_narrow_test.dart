import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/features/order/data/after_sale_model.dart';
import 'package:hawkr_buyer/l10n/l10n.dart';
import 'package:hawkr_buyer/features/order/presentation/widgets/return_logistics_timeline.dart';

void main() {
  setUpAll(() {
    if (!F.isInitialized) F.init(Flavor.dev);
  });

  testWidgets('pickup timeline step with a full date-time does not overflow '
      'at 320 logical px', (tester) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(320, 1400);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);
    final c = AfterSaleCase(
      caseNo: 'AS-TEST-1',
      status: AfterSaleStatus.awaitingReturn,
      returnShippingMethod: ReturnShippingMethod.kuroneko,
      resolutions: const [
        AfterSaleResolution(
          resolutionType: ResolutionType.approved,
          createdAt: '2026-07-30T15:05:00+08:00',
        ),
      ],
    );
    await tester.pumpWidget(
      ScreenUtilInit(
        designSize: const Size(390, 844),
        builder: (_, _) => MaterialApp(
          locale: const Locale('zh'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Scaffold(
            body: SingleChildScrollView(
              child: BlackcatReturnLogisticsCard(caseData: c),
            ),
          ),
        ),
      ),
    );
    await tester.pump();
    expect(find.textContaining('2026/07/30 15:05'), findsWidgets);
    expect(tester.takeException(), isNull);
  });
}
