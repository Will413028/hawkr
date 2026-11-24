import 'package:hawkr_buyer/core/layout/app_text_size.dart';
import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.pb.dart'
    as pb;
import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.pbenum.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:hawkr_buyer/features/notification/presentation/notification_settings_screen.dart';
import 'package:hawkr_buyer/features/notification/presentation/widgets/notification_type_icon.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../support/production_layout_test_harness.dart';

void main() {
  const category = NotificationCategory.NOTIFICATION_CATEGORY_RESTOCK;
  final entries = [
    pb.PreferenceEntry(
      category: category,
      channel: NotificationChannel.NOTIFICATION_CHANNEL_PUSH,
      enabled: true,
      canDisable: true,
    ),
    pb.PreferenceEntry(
      category: category,
      channel: NotificationChannel.NOTIFICATION_CHANNEL_INBOX,
      enabled: true,
      canDisable: false,
    ),
  ];

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
                locale: UiLocale.zhTW,
                category: category,
                entries: entries,
                iconKind: NotificationIconKind.system,
                onChanged: (_, __) {},
              ),
            ),
          ),
        );
        await tester.pumpAndSettle();

        expect(tester.takeException(), isNull);
        final title = category.uiText(UiLocale.zhTW)!;
        expect(find.text(title), findsOneWidget);

        final card = tester.getRect(
          find.byKey(
            const ValueKey(
              'notification-preference-NOTIFICATION_CATEGORY_RESTOCK',
            ),
          ),
        );
        expect(card.left, greaterThanOrEqualTo(0));
        expect(card.right, lessThanOrEqualTo(scenario.size.width));

        final switches = tester
            .widgetList<Switch>(find.byType(Switch))
            .toList();
        expect(switches, hasLength(2));
        expect(switches[0].onChanged, isNotNull);
        expect(switches[1].onChanged, isNull, reason: 'locked channel');
        expect(
          tester.getSize(find.byType(Switch).first).height,
          greaterThanOrEqualTo(48),
        );
        if (scenario.size.width == 320 && scenario.scale == 2) {
          expect(card.height, greaterThan(68));
        }
      },
    );
  }
}
