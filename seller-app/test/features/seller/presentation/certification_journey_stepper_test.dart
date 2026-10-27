import 'package:hawkr_seller/features/seller/presentation/widgets/certification_journey_stepper.dart';
import 'package:hawkr_seller/shared/widgets/hk_frosted_card_surface.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('shows one restrained glass journey shared by certification', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();

    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(
          body: Center(
            child: SizedBox(
              width: 390,
              child: CertificationJourneyStepper(currentStep: 2),
            ),
          ),
        ),
      ),
    );

    expect(find.byType(HkFrostedCardSurface), findsOneWidget);
    expect(find.text('賣家類型'), findsOneWidget);
    expect(find.text('身分資料'), findsOneWidget);
    expect(find.text('收款資料'), findsOneWidget);
    expect(find.text('送審開通'), findsOneWidget);
    expect(find.bySemanticsLabel('認證進度，第 3 步，共 4 步，收款資料'), findsOneWidget);
    expect(tester.takeException(), isNull);

    semantics.dispose();
  });
}
