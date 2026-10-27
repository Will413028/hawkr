import 'package:hooks_riverpod/hooks_riverpod.dart'; // 手寫 FutureProvider：3.0 annotation 不再 re-export
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/session/session_provider.dart';
import '../data/notification_model.dart';
import '../data/notification_repository.dart';

part 'notification_controller.g.dart';

Duration? _disableNotificationAutomaticRetry(int _, Object __) => null;

// ============================================================
// 通知列表 State
// ============================================================

/// 通知列表狀態
class NotificationListState {
  final List<NotificationItem> notifications;
  final int unreadCount;
  final bool hasMore;
  final bool isLoadingMore;
  final Object? paginationError;

  const NotificationListState({
    this.notifications = const [],
    this.unreadCount = 0,
    this.hasMore = false,
    this.isLoadingMore = false,
    this.paginationError,
  });

  NotificationListState copyWith({
    List<NotificationItem>? notifications,
    int? unreadCount,
    bool? hasMore,
    bool? isLoadingMore,
    Object? paginationError,
    bool clearPaginationError = false,
  }) {
    return NotificationListState(
      notifications: notifications ?? this.notifications,
      unreadCount: unreadCount ?? this.unreadCount,
      hasMore: hasMore ?? this.hasMore,
      isLoadingMore: isLoadingMore ?? this.isLoadingMore,
      paginationError: clearPaginationError
          ? null
          : (paginationError ?? this.paginationError),
    );
  }
}

// ============================================================
// 通知列表 Controller
// ============================================================

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
    // 未登入（訪客 / 登出後）不打需登入的 API，避免 401 風暴；
    // session 變動會自動重建。
    final session = ref.watch(sessionProvider).value;
    if (!hasUsableSession(session)) {
      return const NotificationListState();
    }
    // 同時載入通知列表和未讀數量。錯誤必須保留為 AsyncError，讓 UI
    // 區分「真的沒有通知」與「尚未成功載入」。
    final results = await Future.wait([
      _repo.getNotifications(size: _pageSize),
      _repo.getUnreadCount(),
    ]);

    final listResult = results[0] as NotificationListResult;
    final unread = results[1] as UnreadCount;

    return NotificationListState(
      notifications: listResult.records,
      unreadCount: unread.count,
      hasMore: listResult.hasMore,
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

    // Defensive guard: a malformed empty page must never access `.last`.
    if (current.notifications.isEmpty) {
      state = AsyncValue.data(
        current.copyWith(
          hasMore: false,
          isLoadingMore: false,
          clearPaginationError: true,
        ),
      );
      return;
    }

    // 標記正在載入
    state = AsyncValue.data(
      current.copyWith(isLoadingMore: true, clearPaginationError: true),
    );

    try {
      final lastId = current.notifications.last.id;
      final result = await _repo.getNotifications(
        cursorId: lastId,
        size: _pageSize,
      );

      state = AsyncValue.data(
        current.copyWith(
          notifications: [...current.notifications, ...result.records],
          hasMore: result.hasMore,
          isLoadingMore: false,
          clearPaginationError: true,
        ),
      );
    } catch (error) {
      // 分頁錯誤是局部狀態：保留既有 records，不得把整頁改成 AsyncError。
      state = AsyncValue.data(
        current.copyWith(isLoadingMore: false, paginationError: error),
      );
    }
  }

  /// 標記單個通知為已讀
  Future<void> markAsRead(int notificationId) async {
    final current = state.value;
    if (current == null) return;

    try {
      await _repo.markAsRead(notificationId);

      // 更新本地狀態
      final updated = current.notifications.map((n) {
        if (n.id == notificationId && !n.isRead) {
          return n.copyWith(isRead: true, readAt: DateTime.now());
        }
        return n;
      }).toList();

      state = AsyncValue.data(
        current.copyWith(
          notifications: updated,
          unreadCount: (current.unreadCount - 1).clamp(0, 999),
        ),
      );
    } catch (_) {
      // 靜默失敗，不影響 UI
    }
  }

  /// 刪除單筆通知（optimistic update）
  Future<void> deleteNotification(String notificationId) async {
    final current = state.value;
    if (current == null) return;

    final target = current.notifications.cast<NotificationItem?>().firstWhere(
      (n) => '${n!.id}' == notificationId,
      orElse: () => null,
    );

    // Optimistic: 立即從列表移除
    final updated = current.notifications
        .where((n) => '${n.id}' != notificationId)
        .toList();
    final unreadDelta = (target != null && !target.isRead) ? 1 : 0;

    state = AsyncValue.data(
      current.copyWith(
        notifications: updated,
        unreadCount: (current.unreadCount - unreadDelta).clamp(0, 999),
      ),
    );

    try {
      await _repo.deleteNotifications([notificationId]);
    } catch (_) {
      // 靜默失敗，不回滾
    }
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
        .where((n) => idSet.contains('${n.id}'))
        .toList();
    final unreadDelta = targets.where((n) => !n.isRead).length;

    // Optimistic: 立即從列表移除
    state = AsyncValue.data(
      current.copyWith(
        notifications: current.notifications
            .where((n) => !idSet.contains('${n.id}'))
            .toList(),
        unreadCount: (current.unreadCount - unreadDelta).clamp(0, 999),
      ),
    );
    await _repo.deleteNotifications(ids);
  }

  /// 標記全部已讀
  Future<int> markAllAsRead() async {
    try {
      final count = await _repo.markAllAsRead();
      if (count > 0) {
        await refresh();
      }
      return count;
    } catch (_) {
      return 0;
    }
  }
}

// ============================================================
// 通知偏好設定 Controller
// ============================================================

/// 通知偏好設定 AsyncNotifier
///
/// 透過後端 API 讀寫通知偏好（6 個 category toggle）。
@Riverpod(keepAlive: true)
class NotificationPreferencesController
    extends _$NotificationPreferencesController {
  NotificationRepository get _repo => ref.read(notificationRepositoryProvider);

  @override
  Future<List<NotificationPreferenceItem>> build() async {
    // 未登入不打需登入的 API，避免 401 風暴；session 變動會自動重建。
    final session = ref.watch(sessionProvider).value;
    if (!hasUsableSession(session)) {
      return const [];
    }
    return await _repo.getPreferences();
  }

  /// 更新單一 category 的開關
  Future<bool> updatePreference(String category, bool enabled) async {
    final current = state.value;
    if (current == null) return false;

    // Optimistic update
    final optimistic = current.map((item) {
      if (item.category == category) {
        return item.copyWith(enabled: enabled);
      }
      return item;
    }).toList();
    state = AsyncValue.data(optimistic);

    try {
      await _repo.updatePreference(category, enabled);
      return true;
    } catch (_) {
      // Rollback on failure
      state = AsyncValue.data(current);
      return false;
    }
  }

  /// 重新載入偏好設定
  Future<void> reload() async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() => _repo.getPreferences());
  }
}

// ============================================================
// 未讀數量 Provider（首頁鈴鐺 badge 專用）
// ============================================================

/// 通知未讀數量（輕量，不加載完整列表）
///
/// autoDispose：切換 tab 時自動釋放，切回首頁時重新拉取，
/// 實現「每次進入首頁都刷新」的效果。
final notificationUnreadCountProvider = FutureProvider.autoDispose<int>((
  ref,
) async {
  // 訪客（無 token）打 /app/notifications/unread-count 必 401，直接回 0 不打 API。
  final session = ref.watch(sessionProvider).value;
  if (!hasUsableSession(session)) {
    return 0;
  }
  final repo = ref.watch(notificationRepositoryProvider);
  final result = await repo.getUnreadCount();
  return result.count;
});
