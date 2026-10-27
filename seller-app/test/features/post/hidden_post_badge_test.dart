import 'package:hawkr_seller/features/post/presentation/widgets/hidden_post_badge.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('HiddenPostBadge 顯示「平台下架」', (tester) async {
    await tester.pumpWidget(
      ScreenUtilInit(
        designSize: const Size(393, 852),
        builder: (_, __) =>
            const MaterialApp(home: Scaffold(body: HiddenPostBadge())),
      ),
    );
    await tester.pump();
    expect(tester.takeException(), isNull);
    expect(find.text('平台下架'), findsOneWidget);
  });
}
