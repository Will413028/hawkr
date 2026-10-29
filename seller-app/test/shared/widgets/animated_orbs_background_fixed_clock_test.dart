import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_seller/shared/widgets/animated_orbs_background.dart';

void main() {
  testWidgets('沒有傳 currentTime 時使用測試設定固定的白天時刻', (tester) async {
    late bool prefersDarkText;
    await tester.pumpWidget(
      MaterialApp(
        home: AnimatedOrbsBackground(
          enableAnimation: false,
          child: Builder(
            builder: (context) {
              prefersDarkText = AmbientBrightnessScope.prefersDarkTextOf(
                context,
              );
              return const SizedBox();
            },
          ),
        ),
      ),
    );

    expect(AnimatedOrbsBackground.debugNow(), DateTime(2026, 1, 15, 12));
    expect(prefersDarkText, isTrue);
  });
}
