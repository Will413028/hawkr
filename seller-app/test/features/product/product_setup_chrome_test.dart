import 'package:hawkr_seller/features/product/presentation/product_setup_progress_bar.dart';
import 'package:hawkr_seller/features/product/presentation/product_setup_top_bar.dart';
import 'package:hawkr_seller/shared/widgets/app_confirm_dialog.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('product wizard chrome fits a 320px seller screen', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(320, 700));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(
          body: Column(
            children: [
              ProductSetupTopBar(title: '新增商品', subtitle: '商品圖片設定'),
              ProductSetupProgressBar(currentStep: 3),
            ],
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(tester.takeException(), isNull);
    expect(find.text('新增商品'), findsOneWidget);
    expect(find.text('圖片'), findsOneWidget);
    expect(find.bySemanticsLabel(RegExp(r'步驟 3.*圖片.*目前步驟')), findsOneWidget);
  });

  testWidgets('shared confirm dialog returns the selected action', (
    tester,
  ) async {
    Future<bool>? result;
    await tester.pumpWidget(
      MaterialApp(
        home: Builder(
          builder: (context) => Scaffold(
            body: Center(
              child: TextButton(
                onPressed: () {
                  result = showAppConfirmDialog(
                    context,
                    title: '要離開嗎？',
                    message: '目前內容可能不會保留。',
                    confirmText: '離開',
                  );
                },
                child: const Text('開啟'),
              ),
            ),
          ),
        ),
      ),
    );

    await tester.tap(find.text('開啟'));
    await tester.pumpAndSettle();
    expect(find.text('目前內容可能不會保留。'), findsOneWidget);

    await tester.tap(find.text('離開'));
    await tester.pumpAndSettle();
    expect(await result, isTrue);
  });
}
