import 'package:hawkr_buyer/core/layout/app_text_size.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../support/production_layout_test_harness.dart';

void main() {
  testWidgets(
    'production harness reproduces app large text, platform and keyboard insets',
    (tester) async {
      late MediaQueryData mediaQuery;
      late TargetPlatform platform;

      await pumpProductionLayoutTestApp(
        tester,
        size: const Size(390, 844),
        systemTextScale: 1,
        textSize: AppTextSize.large,
        viewPadding: const EdgeInsets.only(top: 59, bottom: 34),
        viewInsets: const EdgeInsets.only(bottom: 318),
        platform: TargetPlatform.iOS,
        home: Builder(
          builder: (context) {
            mediaQuery = MediaQuery.of(context);
            platform = Theme.of(context).platform;
            return const Scaffold(body: SizedBox.expand());
          },
        ),
      );

      expect(mediaQuery.textScaler.scale(16), closeTo(18.4, 0.0001));
      expect(mediaQuery.viewPadding.top, 59);
      expect(mediaQuery.viewPadding.bottom, 34);
      expect(mediaQuery.viewInsets.bottom, 318);
      expect(platform, TargetPlatform.iOS);
      expect(tester.takeException(), isNull);
    },
  );
}
