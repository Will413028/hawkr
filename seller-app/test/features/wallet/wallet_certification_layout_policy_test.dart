import 'package:hawkr_seller/core/layout/app_media_query_policy.dart';
import 'package:hawkr_seller/features/seller/presentation/widgets/certification_journey_stepper.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets(
    'wallet certification journey caps 2.0x system text without overflow',
    (tester) async {
      tester.view.devicePixelRatio = 1;
      tester.view.physicalSize = const Size(320, 844);
      addTearDown(tester.view.resetDevicePixelRatio);
      addTearDown(tester.view.resetPhysicalSize);

      await tester.pumpWidget(
        MaterialApp(
          builder: (context, child) => MediaQuery(
            data: AppMediaQueryPolicy.apply(
              MediaQuery.of(
                context,
              ).copyWith(textScaler: const TextScaler.linear(2)),
            ),
            child: child!,
          ),
          home: const Scaffold(
            body: SingleChildScrollView(
              padding: EdgeInsets.all(16),
              child: CertificationJourneyStepper(currentStep: 0),
            ),
          ),
        ),
      );
      await tester.pump();

      final walletContext = tester.element(
        find.byType(CertificationJourneyStepper),
      );
      expect(MediaQuery.textScalerOf(walletContext).scale(10), 14);
      expect(find.text('賣家類型'), findsOneWidget);
      expect(find.text('送審開通'), findsOneWidget);
      expect(tester.takeException(), isNull);
    },
  );
}
