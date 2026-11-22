import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_buyer/features/review/presentation/rate_screen.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  testWidgets('星等文字取自共用文案目錄，1 到 5 星各自對應', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        child: ScreenUtilInit(
          designSize: const Size(390, 844),
          builder: (_, __) => const MaterialApp(
            home: RateScreen(orderItemId: '1', orderNo: 'N1'),
          ),
        ),
      ),
    );
    expect(find.text('點擊評分'), findsOneWidget);
    const labels = ['非常不滿意', '不滿意', '一般', '滿意', '非常滿意'];
    final stars = find.byIcon(AppIcons.starBorder);
    for (var i = 0; i < 5; i++) {
      await tester.tap(stars.first);
      await tester.pump();
      expect(find.text(labels[i]), findsOneWidget);
    }
  });
}
