import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_seller/core/networking/connect_provider.dart';
import 'package:hawkr_seller/features/notification/presentation/notification_settings_screen.dart';
import 'package:hawkr_seller/l10n/gen/app_localizations.dart';

void main() {
  testWidgets(
    'one switch per entry the server returns; a non-disableable entry is locked',
    (tester) async {
      final updates = <UpdateNotificationPreferenceRequest>[];
      final transport = FakeTransportBuilder()
          .unary(
            NotificationAppService.getNotificationPreferences,
            (_, _) => GetNotificationPreferencesResponse(
              entries: [
                PreferenceEntry(
                  category: NotificationCategory.NOTIFICATION_CATEGORY_CHAT,
                  channel: NotificationChannel.NOTIFICATION_CHANNEL_PUSH,
                  enabled: true,
                  canDisable: true,
                ),
                PreferenceEntry(
                  category: NotificationCategory.NOTIFICATION_CATEGORY_ORDER,
                  channel: NotificationChannel.NOTIFICATION_CHANNEL_INBOX,
                  enabled: true,
                  canDisable: false,
                ),
              ],
            ),
          )
          .unary(NotificationAppService.updateNotificationPreference, (r, _) {
            updates.add(r);
            return UpdateNotificationPreferenceResponse(
              entry: PreferenceEntry(
                category: r.category,
                channel: r.channel,
                enabled: r.enabled,
                canDisable: true,
              ),
            );
          })
          .build();

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            notificationClientProvider.overrideWithValue(
              NotificationAppServiceClient(transport),
            ),
          ],
          child: ScreenUtilInit(
            designSize: const Size(390, 844),
            builder: (_, _) => MaterialApp(
              locale: const Locale('zh'),
              localizationsDelegates: AppLocalizations.localizationsDelegates,
              supportedLocales: AppLocalizations.supportedLocales,
              home: const NotificationSettingsScreen(),
            ),
          ),
        ),
      );
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 100));

      final chatPush = find.byKey(
        ValueKey(
          'pref-${NotificationCategory.NOTIFICATION_CATEGORY_CHAT.value}-'
          '${NotificationChannel.NOTIFICATION_CHANNEL_PUSH.value}',
        ),
      );
      final orderInbox = find.byKey(
        ValueKey(
          'pref-${NotificationCategory.NOTIFICATION_CATEGORY_ORDER.value}-'
          '${NotificationChannel.NOTIFICATION_CHANNEL_INBOX.value}',
        ),
      );
      expect(chatPush, findsOneWidget);
      expect(orderInbox, findsOneWidget);

      bool locked(Finder f) => tester
          .widget<IgnorePointer>(
            find.ancestor(of: f, matching: find.byType(IgnorePointer)).first,
          )
          .ignoring;
      expect(locked(chatPush), isFalse);
      expect(locked(orderInbox), isTrue);

      await tester.tap(chatPush);
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 100));
      expect(updates.single.enabled, isFalse);
      expect(
        updates.single.channel,
        NotificationChannel.NOTIFICATION_CHANNEL_PUSH,
      );
    },
  );
}
