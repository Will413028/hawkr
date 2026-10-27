import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/widgets/share_bottom_sheet.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('share sheet renders one framework drag handle', (tester) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(390, 844);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);

    await tester.pumpWidget(
      ScreenUtilInit(
        designSize: const Size(390, 844),
        builder: (_, __) => MaterialApp(
          theme: ThemeData(
            bottomSheetTheme: const BottomSheetThemeData(
              showDragHandle: true,
              dragHandleColor: AppColors.dragHandle,
            ),
          ),
          home: Builder(
            builder: (context) => Scaffold(
              body: Center(
                child: FilledButton(
                  onPressed: () => showShareSheet(
                    context,
                    const ShareData(
                      title: '白金虱王無刺虱目魚肚',
                      description: 'NT\$299 - 和歆水產',
                      imageUrl: '',
                      url: 'https://example.com/product/1',
                    ),
                  ),
                  child: const Text('分享'),
                ),
              ),
            ),
          ),
        ),
      ),
    );

    await tester.tap(find.text('分享'));
    await tester.pumpAndSettle();

    final bottomSheet = tester.widget<BottomSheet>(find.byType(BottomSheet));
    expect(bottomSheet.showDragHandle, isTrue);
    expect(find.text('白金虱王無刺虱目魚肚'), findsOneWidget);
  });
}
