import 'package:hawkr_seller/features/post/presentation/widgets/page_header.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  Future<void> pump(WidgetTester tester, PageHeader header) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(402, 874);
    addTearDown(tester.view.reset);
    await tester.pumpWidget(
      ScreenUtilInit(
        designSize: const Size(393, 852),
        builder: (_, _) => MaterialApp(home: Scaffold(body: header)),
      ),
    );
    await tester.pump();
  }

  testWidgets('統計沒有設：不顯示粉絲數，評分是 --', (tester) async {
    await pump(tester, const PageHeader(sellerName: '範例測試店'));

    expect(find.textContaining('位訂閱者'), findsNothing);
    expect(find.text('--'), findsOneWidget);
  });

  testWidgets('統計有值：顯示粉絲數與評分', (tester) async {
    await pump(
      tester,
      const PageHeader(
        sellerName: '範例測試店',
        fansCount: 0,
        ratingAverage: 4.5,
        reviewCount: 3,
      ),
    );

    expect(find.text('0 位訂閱者'), findsOneWidget);
    expect(find.text('4.5'), findsOneWidget);
    expect(find.text('--'), findsNothing);
  });
}
