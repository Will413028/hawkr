import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.pb.dart'
    as pb;
import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.pbenum.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/connect_provider.dart';

part 'notification_repository.g.dart';

@riverpod
NotificationRepository notificationRepository(Ref ref) =>
    NotificationRepository(ref.watch(notificationAppClientProvider));

/// 收件匣的一頁。
class InboxPage {
  const InboxPage({required this.items, required this.nextPageToken});

  final List<pb.InboxItem> items;

  /// 空字串表示沒有下一頁。
  final String nextPageToken;

  bool get hasMore => nextPageToken.isNotEmpty;
}

/// 買家的通知（`NotificationAppService`）：收件匣、偏好。文字都是伺服器依請求語系組好的，這裡不組字。
/// 錯誤原樣丟出 `ConnectException`，由畫面以 `errorText` 顯示。
class NotificationRepository {
  NotificationRepository(this._client);

  final NotificationAppServiceClient _client;

  static const _audience = NotificationAudience.NOTIFICATION_AUDIENCE_BUYER;

  Future<InboxPage> listInbox({
    String pageToken = '',
    int pageSize = 20,
  }) async {
    final response = await _client.listInboxItems(
      pb.ListInboxItemsRequest(
        audience: _audience,
        pageSize: pageSize,
        pageToken: pageToken,
      ),
    );
    return InboxPage(
      items: response.items,
      nextPageToken: response.nextPageToken,
    );
  }

  Future<int> unreadCount() async {
    final response = await _client.getUnreadCount(
      pb.GetUnreadCountRequest(audience: _audience),
    );
    return response.unreadCount;
  }

  /// 即時訊號串流（`WatchNotifications`）：只傳「該重讀了」，內容以列表為準。
  Stream<pb.WatchNotificationsResponse> watch() => _client.watchNotifications(
    pb.WatchNotificationsRequest(audience: _audience),
  );

  Future<void> markRead(String id) async {
    await _client.markInboxItemRead(pb.MarkInboxItemReadRequest(id: id));
  }

  Future<int> markAllRead() async {
    final response = await _client.markAllInboxItemsRead(
      pb.MarkAllInboxItemsReadRequest(audience: _audience),
    );
    return response.updatedCount;
  }

  Future<void> deleteItems(List<String> ids) async {
    if (ids.isEmpty) return;
    await _client.batchDeleteInboxItems(
      pb.BatchDeleteInboxItemsRequest(ids: ids),
    );
  }

  /// 伺服器依買家受眾實際會送出的（分類、渠道）列出，只含這些。
  Future<List<pb.PreferenceEntry>> preferences() async {
    final response = await _client.getNotificationPreferences(
      pb.GetNotificationPreferencesRequest(audience: _audience),
    );
    return response.entries;
  }

  Future<pb.PreferenceEntry> updatePreference({
    required NotificationCategory category,
    required NotificationChannel channel,
    required bool enabled,
  }) async {
    final response = await _client.updateNotificationPreference(
      pb.UpdateNotificationPreferenceRequest(
        audience: _audience,
        category: category,
        channel: channel,
        enabled: enabled,
      ),
    );
    return response.entry;
  }
}
