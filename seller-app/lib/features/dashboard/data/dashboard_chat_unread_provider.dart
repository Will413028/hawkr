import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../chat/data/chat_repository.dart';

part 'dashboard_chat_unread_provider.g.dart';

/// 儀表板聊天未讀數 Provider
///
/// 呼叫 ChatRepository.getUnreadCount() 取得未讀訊息數量。
/// 由 DashboardScreen 在 FCM 事件觸發時 invalidate。
@riverpod
Future<int> dashboardChatUnread(Ref ref) async {
  try {
    return await ref.read(chatRepositoryProvider).getUnreadCount();
  } catch (_) {
    return 0;
  }
}
