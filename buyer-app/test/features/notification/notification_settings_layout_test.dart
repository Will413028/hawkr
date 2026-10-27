import 'package:hawkr_buyer/core/layout/app_text_size.dart';
import 'package:hawkr_buyer/features/notification/data/notification_model.dart';
import 'package:hawkr_buyer/features/notification/presentation/notification_settings_screen.dart';
import 'package:hawkr_buyer/features/notification/presentation/widgets/notification_type_icon.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../support/production_layout_test_harness.dart';

void main() {
  const preference = NotificationPreferenceItem(
    category: 'restock',
    title: '補貨與商品重新上架通知',
    description: '追蹤商品完成補貨或重新開放購買時，第一時間通知我查看最新庫存狀態',
  );

  for (final scenario in <({Size size, double scale, AppTextSize textSize})>[
    (size: const Size(320, 568), scale: 1, textSize: AppTextSize.large),
    (size: const Size(320, 568), scale: 2, textSize: AppTextSize.standard),
    (size: const Size(390, 844), scale: 1, textSize: AppTextSize.large),
    (size: const Size(768, 1024), scale: 2, textSize: AppTextSize.standard),
  ]) {
    testWidgets(
      'preference card grows at ${scenario.size.width.toInt()}dp without '
      'overflow',
      (tester) async {
        await pumpProductionLayoutTestApp(
          tester,
          size: scenario.size,
          systemTextScale: scenario.scale,
          textSize: scenario.textSize,
          home: Scaffold(
            body: SingleChildScrollView(
              padding: const EdgeInsets.symmetric(horizontal: 24),
              child: NotificationPreferenceCard(
                preference: preference,
                iconKind: NotificationIconKind.system,
                onChanged: (_) {},
              ),
            ),
          ),
        );
        await tester.pumpAndSettle();

        expect(tester.takeException(), isNull);
        expect(find.text(preference.title), findsOneWidget);
        expect(find.text(preference.description), findsOneWidget);

        final card = tester.getRect(
          find.byKey(const ValueKey('notification-preference-restock')),
        );
        expect(card.left, greaterThanOrEqualTo(0));
        expect(card.right, lessThanOrEqualTo(scenario.size.width));

        for (final text in <String>[preference.title, preference.description]) {
          final textRect = tester.getRect(find.text(text));
          expect(card.contains(textRect.topLeft), isTrue);
          expect(card.contains(textRect.bottomRight), isTrue);
        }
        expect(
          tester.getSize(find.byType(Switch)).height,
          greaterThanOrEqualTo(48),
        );
        if (scenario.size.width == 320 && scenario.scale == 2) {
          expect(card.height, greaterThan(68));
        }
      },
    );
  }
}
