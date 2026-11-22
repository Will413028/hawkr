import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../chat/data/chat_repository.dart';

part 'dashboard_chat_unread_provider.g.dart';

/// 儀表板聊天未讀數：`GetStoreUnreadCount`。讀取失敗時丟出錯誤（儀表板的徽章把錯誤當成沒有數字，不當成 0）。
@riverpod
Future<int> dashboardChatUnread(Ref ref) =>
    ref.read(chatRepositoryProvider).unreadCount();
