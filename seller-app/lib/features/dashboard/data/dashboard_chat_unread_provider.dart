import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../chat/data/chat_repository.dart';
import '../../chat/data/chat_session.dart';

part 'dashboard_chat_unread_provider.g.dart';

/// 儀表板聊天未讀數：`GetStoreUnreadCount`。讀取失敗時丟出錯誤（儀表板的徽章把錯誤當成沒有數字，不當成 0）。
/// 事件串流的 Ready 與 UnreadChanged 帶來新的總數時重讀，徽章不必等使用者重新整理。
@riverpod
Future<int> dashboardChatUnread(Ref ref) {
  final sub = ref
      .watch(chatSessionProvider)
      .unread
      .skip(1)
      .listen((_) => ref.invalidateSelf());
  ref.onDispose(sub.cancel);
  return ref.read(chatRepositoryProvider).unreadCount();
}
