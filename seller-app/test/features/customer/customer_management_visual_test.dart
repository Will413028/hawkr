import 'package:hawkr_seller/features/customer/presentation/customer_controller.dart';
import 'package:hawkr_seller/features/customer/presentation/customer_management_screen.dart';
import 'package:hawkr_seller/shared/widgets/hk_frosted_card_surface.dart';
import 'package:hawkr_seller/shared/widgets/glass_tab_bar.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  testWidgets('customer controls adapt to a compact screen', (tester) async {
    tester.view.physicalSize = const Size(320, 844);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(
      ScreenUtilInit(
        designSize: const Size(390, 844),
        builder: (_, __) => ProviderScope(
          overrides: [
            customerListProvider.overrideWith((ref) async => const []),
          ],
          child: const MaterialApp(home: CustomerManagementScreen()),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byType(HkFrostedCardSurface), findsOneWidget);
    expect(find.byType(HkGlassTabBar), findsOneWidget);
    expect(find.text('暫無顧客'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });
}
