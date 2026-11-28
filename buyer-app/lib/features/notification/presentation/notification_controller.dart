import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.pb.dart'
    as pb;
import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.pbenum.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/session/session_provider.dart';
import '../data/notification_repository.dart';

part 'notification_controller.g.dart';

Duration? _disableNotificationAutomaticRetry(int _, Object __) => null;

/// 通知列表狀態
class NotificationListState {
  final List<pb.InboxItem> notifications;
  final int unreadCount;
  final String nextPageToken;
  final bool isLoadingMore;
  final Object? paginationError;

  const NotificationListState({
    this.notifications = const [],
    this.unreadCount = 0,
    this.nextPageToken = '',
    this.isLoadingMore = false,
    this.paginationError,
  });

  bool get hasMore => nextPageToken.isNotEmpty;

  NotificationListState copyWith({
    List<pb.InboxItem>? notifications,
    int? unreadCount,
    String? nextPageToken,
    bool? isLoadingMore,
    Object? paginationError,
    bool clearPaginationError = false,
  }) {
    return NotificationListState(
      notifications: notifications ?? this.notifications,
      unreadCount: unreadCount ?? this.unreadCount,
      nextPageToken: nextPageToken ?? this.nextPageToken,
      isLoadingMore: isLoadingMore ?? this.isLoadingMore,
      paginationError: clearPaginationError
          ? null
          : (paginationError ?? this.paginationError),
    );
  }
}

/// 通知列表 AsyncNotifier
///
/// 管理通知列表 + 分頁 + 未讀數量。
/// 使用 `AsyncValue<NotificationListState>` 表示整體狀態。
@Riverpod(keepAlive: true, retry: _disableNotificationAutomaticRetry)
class NotificationListController extends _$NotificationListController {
  static const int _pageSize = 20;

  NotificationRepository get _repo => ref.read(notificationRepositoryProvider);

  @override
  Future<NotificationListState> build() async {
    final session = ref.watch(sessionProvider).value;
    if (!hasUsableSession(session)) {
      return const NotificationListState();
    }
    final results = await Future.wait([
      _repo.listInbox(pageSize: _pageSize),
      _repo.unreadCount(),
    ]);

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
    state = await AsyncValue.guard(() => build());
  }

  /// 靜默刷新（FruitRefreshIndicator 下拉刷新專用）
  ///
  /// 不設 AsyncValue.loading()，避免觸發 TruckLoadingOverlay；
  /// 動畫收起時機由 FruitRefreshIndicator 在 Future 完成後自動處理。
  Future<void> silentRefresh() async {
    state = await AsyncValue.guard(() => build());
  }

  /// 載入更多（分頁滾動）
  Future<void> loadMore() => _loadMore(isRetry: false);

  /// 重試上一個分頁。既有 records 留在畫面上，只有尾端錯誤區塊更新。
  Future<void> retryLoadMore() => _loadMore(isRetry: true);

  Future<void> _loadMore({required bool isRetry}) async {
    final current = state.value;
    if (current == null ||
        current.isLoadingMore ||
        !current.hasMore ||
        (!isRetry && current.paginationError != null)) {
      return;
    }

    if (current.notifications.isEmpty) {
      state = AsyncValue.data(
        current.copyWith(
          nextPageToken: '',
          isLoadingMore: false,
          clearPaginationError: true,
        ),
      );
      return;
    }

    state = AsyncValue.data(
      current.copyWith(isLoadingMore: true, clearPaginationError: true),
    );

    try {
      final result = await _repo.listInbox(
        pageToken: current.nextPageToken,
        pageSize: _pageSize,
      );

      state = AsyncValue.data(
        current.copyWith(
          notifications: [...current.notifications, ...result.items],
          nextPageToken: result.nextPageToken,
          isLoadingMore: false,
          clearPaginationError: true,
        ),
      );
    } catch (error) {
      state = AsyncValue.data(
        current.copyWith(isLoadingMore: false, paginationError: error),
      );
    }
  }

  /// 標記單個通知為已讀
  Future<void> markAsRead(String notificationId) async {
    final current = state.value;
    if (current == null) return;

    try {
      await _repo.markRead(notificationId);

      final updated = current.notifications.map((n) {
        if (n.id == notificationId && !n.read) {
          return n.rebuild((b) => b.read = true);
        }
        return n;
      }).toList();

      state = AsyncValue.data(
        current.copyWith(
          notifications: updated,
          unreadCount: (current.unreadCount - 1).clamp(0, 999),
        ),
      );
    } catch (_) {}
  }

  /// 刪除單筆通知（optimistic update）
  Future<void> deleteNotification(String notificationId) async {
    final current = state.value;
    if (current == null) return;

    final target = current.notifications.cast<pb.InboxItem?>().firstWhere(
      (n) => n!.id == notificationId,
      orElse: () => null,
    );

    final updated = current.notifications
        .where((n) => n.id != notificationId)
        .toList();
    final unreadDelta = (target != null && !target.read) ? 1 : 0;

    state = AsyncValue.data(
      current.copyWith(
        notifications: updated,
        unreadCount: (current.unreadCount - unreadDelta).clamp(0, 999),
      ),
    );

    try {
      await _repo.deleteItems([notificationId]);
    } catch (_) {}
  }

  /// 批次刪除通知（optimistic update）
  Future<void> deleteNotifications(List<String> ids) async {
    if (ids.isEmpty) {
      return;
    }
    final current = state.value;
    if (current == null) {
      return;
    }

    final idSet = ids.toSet();
    final targets = current.notifications
        .where((n) => idSet.contains(n.id))
        .toList();
    final unreadDelta = targets.where((n) => !n.read).length;

    state = AsyncValue.data(
      current.copyWith(
        notifications: current.notifications
            .where((n) => !idSet.contains(n.id))
            .toList(),
        unreadCount: (current.unreadCount - unreadDelta).clamp(0, 999),
      ),
    );
    await _repo.deleteItems(ids);
  }

  /// 標記全部已讀
  Future<int> markAllAsRead() async {
    try {
      final count = await _repo.markAllRead();
      if (count > 0) {
        await refresh();
      }
      return count;
    } catch (_) {
      return 0;
    }
  }
}

/// 通知偏好 AsyncNotifier：伺服器依買家受眾列出實際會送出的（分類、渠道）項目，畫面只呈現這些。
@Riverpod(keepAlive: true)
class NotificationPreferencesController
    extends _$NotificationPreferencesController {
  NotificationRepository get _repo => ref.read(notificationRepositoryProvider);

  @override
  Future<List<pb.PreferenceEntry>> build() async {
    final session = ref.watch(sessionProvider).value;
    if (!hasUsableSession(session)) {
      return const [];
    }
    return await _repo.preferences();
  }

  /// 更新單一（分類、渠道）的開關；失敗時回滾並回傳 false。
  Future<bool> updatePreference(
    NotificationCategory category,
    NotificationChannel channel,
    bool enabled,
  ) async {
    final current = state.value;
    if (current == null) return false;

    bool matches(pb.PreferenceEntry e) =>
        e.category == category && e.channel == channel;
    state = AsyncValue.data([
      for (final e in current)
        if (matches(e)) e.rebuild((b) => b.enabled = enabled) else e,
    ]);

    try {
      final saved = await _repo.updatePreference(
        category: category,
        channel: channel,
        enabled: enabled,
      );
      final latest = state.value ?? current;
      state = AsyncValue.data([
        for (final e in latest)
          if (matches(e)) saved else e,
      ]);
      return true;
    } catch (_) {
      state = AsyncValue.data(current);
      return false;
    }
  }

  /// 重新載入偏好設定
  Future<void> reload() async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() => _repo.preferences());
  }
}

/// 通知未讀數量（輕量，不加載完整列表）
///
/// autoDispose：切換 tab 時自動釋放，切回首頁時重新拉取，
/// 實現「每次進入首頁都刷新」的效果。
final notificationUnreadCountProvider = FutureProvider.autoDispose<int>((
  ref,
) async {
  final session = ref.watch(sessionProvider).value;
  if (!hasUsableSession(session)) {
    return 0;
  }
  final repo = ref.watch(notificationRepositoryProvider);
  return repo.unreadCount();
});
