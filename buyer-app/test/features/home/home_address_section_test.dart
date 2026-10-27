import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:hawkr_buyer/features/home/presentation/home_widget/home_address_section.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  for (final width in <double>[320, 390, 768]) {
    for (final textScale in <double>[1, 1.15, 1.4]) {
      testWidgets(
        'address row is compact at ${width.toInt()}px / ${textScale}x text',
        (tester) async {
          tester.view.devicePixelRatio = 1;
          tester.view.physicalSize = Size(width, 844);
          addTearDown(tester.view.resetPhysicalSize);
          addTearDown(tester.view.resetDevicePixelRatio);

          var tapCount = 0;
          await tester.pumpWidget(
            ProviderScope(
              child: ScreenUtilInit(
                designSize: const Size(390, 844),
                enableScaleWH: () => false,
                enableScaleText: () => false,
                builder: (_, __) => MaterialApp(
                  builder: (context, child) => MediaQuery(
                    data: MediaQuery.of(
                      context,
                    ).copyWith(textScaler: TextScaler.linear(textScale)),
                    child: child!,
                  ),
                  home: Scaffold(
                    body: HomeAddressSection(onTap: () => tapCount += 1),
                  ),
                ),
              ),
            ),
          );
          await tester.pumpAndSettle(const Duration(milliseconds: 100));

          expect(tester.takeException(), isNull);
          final icon = tester.widget<HkIcon>(
            find.byKey(const ValueKey('home-address-delivery-icon')),
          );
          expect(icon.name, HkIconName.truck);
          expect(icon.size, 32);
          expect(icon.color, isNull);
          expect(
            tester
                .widget<Icon>(
                  find.descendant(
                    of: find.byKey(
                      const ValueKey('home-address-delivery-icon'),
                    ),
                    matching: find.byType(Icon),
                  ),
                )
                .icon,
            AppIcons.localShippingOutlined,
          );
          final sectionHeight = tester
              .getSize(find.byKey(const ValueKey('home-address-section')))
              .height;
          if (textScale == 1) {
            expect(sectionHeight, 56);
          } else {
            expect(sectionHeight, inInclusiveRange(56, 80));
          }

          await tester.tap(
            find.byKey(const ValueKey('home-address-delivery-icon')),
          );
          await tester.pump(const Duration(milliseconds: 150));

          expect(tapCount, 1);
        },
      );
    }
  }
}
