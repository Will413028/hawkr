import 'dart:math' as math;

import 'package:hawkr_buyer/core/layout/app_media_query_policy.dart';
import 'package:hawkr_buyer/core/layout/app_text_size.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/widgets/buyer_floating_navigation_layout.dart';
import 'package:hawkr_buyer/shared/widgets/common_top_bar.dart';
import 'package:hawkr_buyer/shared/widgets/shared_bottom_nav.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  const devices = <({String name, Size size, EdgeInsets safeArea})>[
    (
      name: 'iPhone SE portrait',
      size: Size(375, 667),
      safeArea: EdgeInsets.only(top: 20),
    ),
    (
      name: 'iPhone notch portrait',
      size: Size(375, 812),
      safeArea: EdgeInsets.only(top: 44, bottom: 34),
    ),
    (
      name: 'iPhone Dynamic Island portrait',
      size: Size(390, 844),
      safeArea: EdgeInsets.only(top: 59, bottom: 34),
    ),
    (
      name: 'iPhone Max portrait',
      size: Size(430, 932),
      safeArea: EdgeInsets.only(top: 59, bottom: 34),
    ),
    (
      name: 'iPhone landscape',
      size: Size(844, 390),
      safeArea: EdgeInsets.fromLTRB(59, 0, 59, 21),
    ),
    (
      name: 'iPad portrait',
      size: Size(768, 1024),
      safeArea: EdgeInsets.only(top: 24, bottom: 20),
    ),
  ];

  for (final device in devices) {
    for (final textScale
        in <({double system, AppTextSize preference, double effective})>[
          (system: 1.0, preference: AppTextSize.small, effective: 0.9),
          (system: 1.0, preference: AppTextSize.standard, effective: 1.0),
          (system: 1.0, preference: AppTextSize.large, effective: 1.15),
          (system: 1.4, preference: AppTextSize.standard, effective: 1.4),
          (system: 2.0, preference: AppTextSize.large, effective: 1.4),
        ]) {
      testWidgets(
        '${device.name} keeps chrome inside the physical safe area at '
        '${textScale.system}x system text + ${textScale.preference.name}',
        (tester) async {
          tester.view.devicePixelRatio = 1;
          tester.view.physicalSize = device.size;
          addTearDown(tester.view.resetDevicePixelRatio);
          addTearDown(tester.view.resetPhysicalSize);

          double pageBottomPadding = 0;
          double scaledText = 0;
          await tester.pumpWidget(
            ProviderScope(
              child: MaterialApp(
                home: MediaQuery(
                  data: AppMediaQueryPolicy.apply(
                    MediaQueryData(
                      size: device.size,
                      padding: device.safeArea,
                      viewPadding: device.safeArea,
                      textScaler: TextScaler.linear(textScale.system),
                    ),
                    textSize: textScale.preference,
                  ),
                  child: BuyerFloatingNavigationLayout(
                    navigationVisible: true,
                    visibilityResetKey: device.name,
                    navigation: const SharedBottomNav(),
                    page: Builder(
                      builder: (context) {
                        final mediaQuery = MediaQuery.of(context);
                        pageBottomPadding = mediaQuery.padding.bottom;
                        scaledText = mediaQuery.textScaler.scale(16);
                        return Column(
                          children: [
                            KeyedSubtree(
                              key: const ValueKey('matrix-top-bar'),
                              child: commonTopBar(
                                context: context,
                                child: const SizedBox.expand(
                                  key: ValueKey('matrix-top-bar-content'),
                                ),
                              ),
                            ),
                            const Expanded(
                              child: ColoredBox(color: Colors.white),
                            ),
                          ],
                        );
                      },
                    ),
                  ),
                ),
              ),
            ),
          );

          final topBarRect = tester.getRect(
            find.byKey(const ValueKey('matrix-top-bar')),
          );
          final topBarContentRect = tester.getRect(
            find.byKey(const ValueKey('matrix-top-bar-content')),
          );
          final navigationRect = tester.getRect(
            find.byKey(const ValueKey('buyer-bottom-nav-surface')),
          );

          expect(topBarRect.height, device.safeArea.top + kTopBarHeight);
          expect(
            topBarContentRect.left,
            greaterThanOrEqualTo(device.safeArea.left + HkSpace.md),
          );
          expect(
            topBarContentRect.right,
            lessThanOrEqualTo(
              device.size.width - device.safeArea.right - HkSpace.md,
            ),
          );
          expect(
            navigationRect.left,
            greaterThanOrEqualTo(device.safeArea.left + HkSpace.md),
          );
          expect(
            navigationRect.right,
            lessThanOrEqualTo(
              device.size.width - device.safeArea.right - HkSpace.md,
            ),
          );
          expect(
            navigationRect.bottom,
            lessThanOrEqualTo(
              device.size.height - math.max(device.safeArea.bottom, HkSpace.md),
            ),
          );
          expect(
            pageBottomPadding,
            HkControl.navHeight +
                HkSpace.md +
                math.max(device.safeArea.bottom, HkSpace.md),
          );
          expect(scaledText, closeTo(16 * textScale.effective, 0.0001));
          expect(tester.takeException(), isNull);
        },
      );
    }
  }
}
