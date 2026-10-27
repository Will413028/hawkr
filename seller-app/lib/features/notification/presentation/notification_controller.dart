import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/exceptions/app_exception.dart';
import '../../../shared/utils/hk_log.dart';
import '../data/notification_model.dart';
import '../data/notification_repository.dart';

part 'notification_controller.g.dart';

// ============================================================
// 通知列表 State
// ============================================================

/// 通知列表狀態
class NotificationListState {
  final List<NotificationItem> notifications;
  final int unreadCount;
  final bool hasMore;
  final bool isLoadingMore;

  const NotificationListState({
    this.notifications = const [],
    this.unreadCount = 0,
    this.hasMore = true,
    this.isLoadingMore = false,
  });

  NotificationListState copyWith({
    List<NotificationItem>? notifications,
    int? unreadCount,
    bool? hasMore,
    bool? isLoadingMore,
  }) {
    return NotificationListState(
      notifications: notifications ?? this.notifications,
      unreadCount: unreadCount ?? this.unreadCount,
      hasMore: hasMore ?? this.hasMore,
      isLoadingMore: isLoadingMore ?? this.isLoadingMore,
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
@Riverpod(keepAlive: true)
class NotificationListController extends _$NotificationListController {
  static const int _pageSize = 20;

  NotificationRepository get _repo => ref.read(notificationRepositoryProvider);

  @override
  Future<NotificationListState> build() async {
    try {
      // 同時載入通知列表和未讀數量
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
    } on ServerException catch (e) {
      HKLog.logW(
        '[NotificationListController] 伺服器錯誤，降級為空列表: ${e.statusCode} ${e.message}',
      );
      return const NotificationListState();
    } on NetworkException catch (e) {
      HKLog.logW('[NotificationListController] 網路不可達，降級為空列表: ${e.message}');
      return const NotificationListState();
    } on TimeoutException catch (e) {
      HKLog.logW('[NotificationListController] 連線逾時，降級為空列表: ${e.message}');
      return const NotificationListState();
    }
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
  Future<void> loadMore() async {
    final current = state.value;
    if (current == null || current.isLoadingMore || !current.hasMore) return;

    // 標記正在載入
    state = AsyncValue.data(current.copyWith(isLoadingMore: true));

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
        ),
      );
    } catch (e, st) {
      // 載入更多失敗，回復到非載入狀態但保留已有資料
      state = AsyncValue.data(current.copyWith(isLoadingMore: false));
      state = AsyncValue.error(e, st);
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
    if (ids.isEmpty) return;
    final current = state.value;
    if (current == null) return;

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
