import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.pb.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/connect_provider.dart';

export 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.pb.dart'
    show InboxItem, InboxSignal, PreferenceEntry, WatchNotificationsResponse;
export 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.pbenum.dart'
    show
        InboxPresentation,
        NotificationCategory,
        NotificationChannel,
        StateRefreshHint,
        SellerWorkspaceChange;

part 'notification_repository.g.dart';

@Riverpod(keepAlive: true)
NotificationRepository notificationRepository(Ref ref) =>
    NotificationRepository(ref.watch(notificationClientProvider));

/// 收件匣的一頁；文字、深連結與呈現分類都是伺服器給的。
class InboxPage {
  const InboxPage({
    required this.items,
    required this.nextPageToken,
    required this.totalSize,
  });

  final List<InboxItem> items;
  final String nextPageToken;
  final int totalSize;
}

/// 通知（`NotificationAppService`，STORE 受眾）。失敗一律丟出 `ConnectException`，由呼叫端決定如何呈現。
class NotificationRepository {
  NotificationRepository(this._client);

  static const pageSize = 20;
  static const _audience = NotificationAudience.NOTIFICATION_AUDIENCE_STORE;

  final NotificationAppServiceClient _client;

  Future<InboxPage> listInbox({String pageToken = ''}) async {
    final res = await _client.listInboxItems(
      ListInboxItemsRequest(
        audience: _audience,
        pageSize: pageSize,
        pageToken: pageToken,
      ),
    );
    return InboxPage(
      items: res.items,
      nextPageToken: res.nextPageToken,
      totalSize: res.totalSize,
    );
  }

  Future<int> unreadCount() async => (await _client.getUnreadCount(
    GetUnreadCountRequest(audience: _audience),
  )).unreadCount;

  Future<void> markRead(String id) async {
    await _client.markInboxItemRead(MarkInboxItemReadRequest(id: id));
  }

  Future<int> markAllRead() async => (await _client.markAllInboxItemsRead(
    MarkAllInboxItemsReadRequest(audience: _audience),
  )).updatedCount;

  Future<void> deleteItems(List<String> ids) async {
    if (ids.isEmpty) return;
    await _client.batchDeleteInboxItems(BatchDeleteInboxItemsRequest(ids: ids));
  }

  Future<List<PreferenceEntry>> preferences() async =>
      (await _client.getNotificationPreferences(
        GetNotificationPreferencesRequest(audience: _audience),
      )).entries;

  Future<PreferenceEntry> updatePreference(
    NotificationCategory category,
    NotificationChannel channel,
    bool enabled,
  ) async => (await _client.updateNotificationPreference(
    UpdateNotificationPreferenceRequest(
      audience: _audience,
      category: category,
      channel: channel,
      enabled: enabled,
    ),
  )).entry;

  /// 即時訊號串流；斷線與重連由 `NotificationSignalService` 處理。
  Stream<WatchNotificationsResponse> watch() => _client.watchNotifications(
    WatchNotificationsRequest(audience: _audience),
  );
}
