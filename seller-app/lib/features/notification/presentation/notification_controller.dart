import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../data/notification_repository.dart';

part 'notification_controller.g.dart';

/// 收件匣列表狀態
class NotificationListState {
  const NotificationListState({
    this.notifications = const [],
    this.unreadCount = 0,
    this.nextPageToken = '',
    this.isLoadingMore = false,
  });

  final List<InboxItem> notifications;
  final int unreadCount;
  final String nextPageToken;
  final bool isLoadingMore;

  bool get hasMore => nextPageToken.isNotEmpty;

  NotificationListState copyWith({
    List<InboxItem>? notifications,
    int? unreadCount,
    String? nextPageToken,
    bool? isLoadingMore,
  }) => NotificationListState(
    notifications: notifications ?? this.notifications,
    unreadCount: unreadCount ?? this.unreadCount,
    nextPageToken: nextPageToken ?? this.nextPageToken,
    isLoadingMore: isLoadingMore ?? this.isLoadingMore,
  );
}

/// 收件匣 AsyncNotifier：第一頁、分頁與未讀數都來自伺服器；已讀與刪除成功後以伺服器的答案為準。
@Riverpod(keepAlive: true)
class NotificationListController extends _$NotificationListController {
  NotificationRepository get _repo => ref.read(notificationRepositoryProvider);

  @override
  Future<NotificationListState> build() => _load();

  Future<NotificationListState> _load() async {
    final results = await Future.wait([_repo.listInbox(), _repo.unreadCount()]);
    final page = results[0] as InboxPage;
    return NotificationListState(
      notifications: page.items,
      unreadCount: results[1] as int,
      nextPageToken: page.nextPageToken,
    );
  }

  /// 重新載入（頁面級刷新，顯示 loading 遮罩）
  Future<void> refresh() async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(_load);
  }

  /// 靜默刷新（下拉刷新與串流訊號用）：不設 loading；失敗時保留目前的列表。
  Future<void> silentRefresh() async {
    final next = await AsyncValue.guard(_load);
    if (next.hasError && state.hasValue) return;
    state = next;
  }

  Future<void> loadMore() async {
    final current = state.value;
    if (current == null || current.isLoadingMore || !current.hasMore) return;
    state = AsyncValue.data(current.copyWith(isLoadingMore: true));
    try {
      final page = await _repo.listInbox(pageToken: current.nextPageToken);
      state = AsyncValue.data(
        current.copyWith(
          notifications: [...current.notifications, ...page.items],
          nextPageToken: page.nextPageToken,
          isLoadingMore: false,
        ),
      );
    } catch (_) {
      state = AsyncValue.data(current.copyWith(isLoadingMore: false));
    }
  }

  Future<void> markAsRead(String id) async {
    try {
      await _repo.markRead(id);
    } catch (_) {
      return;
    }
    await silentRefresh();
  }

  Future<void> deleteNotifications(List<String> ids) async {
    if (ids.isEmpty) return;
    try {
      await _repo.deleteItems(ids);
    } finally {
      await silentRefresh();
    }
  }

  Future<int> markAllAsRead() async {
    try {
      final count = await _repo.markAllRead();
      await silentRefresh();
      return count;
    } catch (_) {
      return 0;
    }
  }
}

/// 偏好：伺服器回哪些 (分類, 渠道) 就列哪些；不可關的 `canDisable` 為 false。
@Riverpod(keepAlive: true)
class NotificationPreferencesController
    extends _$NotificationPreferencesController {
  NotificationRepository get _repo => ref.read(notificationRepositoryProvider);

  @override
  Future<List<PreferenceEntry>> build() => _repo.preferences();

  /// 設定一個 (分類, 渠道) 的開關；成功以伺服器回的那一項取代，失敗回復原狀並回傳 false。
  Future<bool> setEnabled(
    NotificationCategory category,
    NotificationChannel channel,
    bool enabled,
  ) async {
    final current = state.value;
    if (current == null) return false;
    bool same(PreferenceEntry e) =>
        e.category == category && e.channel == channel;
    state = AsyncValue.data([
      for (final e in current)
        same(e)
            ? PreferenceEntry(
                category: e.category,
                channel: e.channel,
                enabled: enabled,
                canDisable: e.canDisable,
              )
            : e,
    ]);
    try {
      final saved = await _repo.updatePreference(category, channel, enabled);
      state = AsyncValue.data([
        for (final e in state.value ?? current) same(e) ? saved : e,
      ]);
      return true;
    } catch (_) {
      state = AsyncValue.data(current);
      return false;
    }
  }

  Future<void> reload() async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(_repo.preferences);
  }
}
