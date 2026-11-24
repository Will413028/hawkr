import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_seller/features/notification/data/notification_repository.dart';

void main() {
  test(
    'every call names the STORE audience; the preference update names category and channel',
    () async {
      final seen = <Object>[];
      final repo = NotificationRepository(
        NotificationAppServiceClient(
          FakeTransportBuilder()
              .unary(NotificationAppService.getUnreadCount, (r, _) {
                seen.add(r);
                return GetUnreadCountResponse(unreadCount: 4);
              })
              .unary(NotificationAppService.markAllInboxItemsRead, (r, _) {
                seen.add(r);
                return MarkAllInboxItemsReadResponse(updatedCount: 2);
              })
              .unary(NotificationAppService.updateNotificationPreference, (
                r,
                _,
              ) {
                seen.add(r);
                return UpdateNotificationPreferenceResponse(
                  entry: PreferenceEntry(
                    category: r.category,
                    channel: r.channel,
                    enabled: r.enabled,
                    canDisable: true,
                  ),
                );
              })
              .build(),
        ),
      );

      expect(await repo.unreadCount(), 4);
      expect(await repo.markAllRead(), 2);
      final entry = await repo.updatePreference(
        NotificationCategory.NOTIFICATION_CATEGORY_CHAT,
        NotificationChannel.NOTIFICATION_CHANNEL_PUSH,
        false,
      );
      expect(entry.enabled, isFalse);

      const store = NotificationAudience.NOTIFICATION_AUDIENCE_STORE;
      expect((seen[0] as GetUnreadCountRequest).audience, store);
      expect((seen[1] as MarkAllInboxItemsReadRequest).audience, store);
      final update = seen[2] as UpdateNotificationPreferenceRequest;
      expect(update.audience, store);
      expect(update.category, NotificationCategory.NOTIFICATION_CATEGORY_CHAT);
      expect(update.channel, NotificationChannel.NOTIFICATION_CHANNEL_PUSH);
    },
  );
}
