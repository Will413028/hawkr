import 'package:hawkr_seller/core/layout/app_media_query_policy.dart';
import 'package:hawkr_seller/features/settings/presentation/shipping_settings/widgets/common_widgets.dart';
import 'package:hawkr_seller/features/settings/presentation/shipping_settings/widgets/taqbin_shipping_section.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  testWidgets('black cat shipping form does not overflow on a compact screen', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    tester.view.physicalSize = const Size(320, 844);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    final expanded = ValueNotifier(true);
    final selected = ValueNotifier(true);
    final city = ValueNotifier<String?>(null);
    final district = ValueNotifier<String?>(null);
    final verified = ValueNotifier(false);
    final verifying = ValueNotifier(false);
    final name = TextEditingController();
    final phone = TextEditingController();
    final address = TextEditingController();
    addTearDown(() {
      expanded.dispose();
      selected.dispose();
      city.dispose();
      district.dispose();
      verified.dispose();
      verifying.dispose();
      name.dispose();
      phone.dispose();
      address.dispose();
    });

    await tester.pumpWidget(
      ScreenUtilInit(
        designSize: const Size(390, 844),
        builder: (_, __) => ProviderScope(
          child: MaterialApp(
            builder: (context, child) => MediaQuery(
              data: AppMediaQueryPolicy.apply(
                MediaQuery.of(
                  context,
                ).copyWith(textScaler: const TextScaler.linear(2)),
              ),
              child: child!,
            ),
            home: Scaffold(
              body: SingleChildScrollView(
                child: Padding(
                  padding: const EdgeInsets.all(16),
                  child: TaqbinShippingSection(
                    expanded: expanded,
                    selected: selected,
                    nameCtrl: name,
                    phoneCtrl: phone,
                    city: city,
                    district: district,
                    addressCtrl: address,
                    addressVerified: verified,
                    addressVerifying: verifying,
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('黑貓宅急便'), findsOneWidget);
    final settingsContext = tester.element(find.byType(TaqbinShippingSection));
    expect(MediaQuery.textScalerOf(settingsContext).scale(10), 14);
    expect(find.text('聯絡人姓名'), findsOneWidget);
    expect(find.text('驗證地址'), findsOneWidget);
    expect(find.text('配送溫層'), findsNothing);
    expect(tester.takeException(), isNull);
    await tester.tap(find.byType(CustomCheckbox));
    await tester.pump();
    expect(selected.value, isFalse);
    await expectLater(tester, meetsGuideline(androidTapTargetGuideline));
    await expectLater(tester, meetsGuideline(iOSTapTargetGuideline));
    await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));
    semantics.dispose();
  });
}
