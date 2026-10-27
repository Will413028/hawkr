import 'package:hawkr_seller/features/seller/presentation/widgets/certification_glass_step_card.dart';
import 'package:hawkr_seller/shared/widgets/hk_frosted_card_surface.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('certification step stays readable on a compact phone', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();

    await tester.pumpWidget(
      ScreenUtilInit(
        designSize: const Size(390, 844),
        builder: (_, __) => const MaterialApp(
          home: Scaffold(
            body: Center(
              child: SizedBox(
                width: 320,
                child: CertificationGlassStepCard(
                  title: '收款帳戶',
                  subtitle: '確認辨識結果；帳戶名稱需與認證身分一致',
                  icon: HkIconName.wallet,
                  child: TextField(
                    decoration: InputDecoration(labelText: '銀行帳號'),
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );

    expect(find.byType(HkFrostedCardSurface), findsOneWidget);
    expect(find.byType(HkIcon), findsOneWidget);
    expect(find.text('收款帳戶'), findsOneWidget);
    expect(
      find.bySemanticsLabel(RegExp(r'收款帳戶.*帳戶名稱需與認證身分一致')),
      findsOneWidget,
    );
    expect(tester.takeException(), isNull);

    semantics.dispose();
  });
}
