import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../notification/data/notification_repository.dart';

part 'dashboard_notification_unread_provider.g.dart';

/// 儀表板通知未讀數 Provider
///
/// 呼叫 NotificationRepository.getUnreadCount() 取得未讀通知數量。
/// 僅用於 Dashboard 鈴鐺紅點顯示，由下拉刷新時 invalidate。
@riverpod
Future<int> dashboardNotificationUnread(Ref ref) async {
  try {
    final repo = ref.read(notificationRepositoryProvider);
    final result = await repo.getUnreadCount();
    return result.count;
  } catch (_) {
    return 0;
  }
}
