import 'dart:async';

import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../dashboard/data/dashboard_chat_unread_provider.dart';
import 'chat_model.dart';
import 'chat_repository.dart';
import 'chat_session.dart';
import 'chat_stream_client.dart';

part 'chat_conversations_notifier.g.dart';

/// 對話列表的資料：已載入的頁與下一頁的 token。
class ChatConversationsState {
  const ChatConversationsState({
    required this.items,
    required this.nextPageToken,
    this.loadingMore = false,
    this.loadMoreFailed = false,
  });

  final List<ChatConversation> items;
  final String nextPageToken;
  final bool loadingMore;

  /// 載入下一頁失敗：已載入的列留著，列表底部提示重試。
  final bool loadMoreFailed;

  bool get hasMore => nextPageToken.isNotEmpty;

  ChatConversationsState copyWith({
    List<ChatConversation>? items,
    String? nextPageToken,
    bool? loadingMore,
    bool? loadMoreFailed,
  }) => ChatConversationsState(
    items: items ?? this.items,
    nextPageToken: nextPageToken ?? this.nextPageToken,
    loadingMore: loadingMore ?? this.loadingMore,
    loadMoreFailed: loadMoreFailed ?? this.loadMoreFailed,
  );
}

/// 讀取失敗立刻顯示錯誤與重試，不走 Riverpod 的自動重試（它會讓畫面一直停在載入中）。
Duration? _noAutoRetry(int retryCount, Object error) => null;

/// 店家的對話列表。讀取失敗是 [AsyncError]，與「沒有對話」（空的 items）分開。
@Riverpod(retry: _noAutoRetry)
class ChatConversations extends _$ChatConversations {
  StreamSubscription<ChatStreamEvent>? _sub;

  @override
  Future<ChatConversationsState> build() async {
    final session = ref.read(chatSessionProvider);
    unawaited(_sub?.cancel());
    _sub = session.events.listen(_onEvent);
    ref.onDispose(() => _sub?.cancel());
    final page = await ref.read(chatRepositoryProvider).listConversations();
    session.rememberConversations(page.items);
    return ChatConversationsState(
      items: page.items,
      nextPageToken: page.nextPageToken,
    );
  }

  void _onEvent(ChatStreamEvent e) {
    switch (e) {
      case ChatMessageArrived(:final updated) when updated:
      case ChatPeerRead():
      case ChatReady():
      case ChatUnreadChanged():
        return;
      case ChatMessageArrived():
      case ChatResync():
        unawaited(refresh());
    }
  }

  /// 重讀第一頁（背景更新）：失敗時留著畫面上已有的列。
  Future<void> refresh() async {
    final cur = state.value;
    if (cur == null) return;
    try {
      final page = await ref.read(chatRepositoryProvider).listConversations();
      if (!ref.mounted) return;
      ref.read(chatSessionProvider).rememberConversations(page.items);
      state = AsyncData(
        ChatConversationsState(
          items: page.items,
          nextPageToken: page.nextPageToken,
        ),
      );
    } on Object {
      // 背景更新失敗不蓋掉已有的列表。
    }
  }

  /// 錯誤畫面的重試與下拉更新。
  Future<void> reload() async {
    state = const AsyncLoading();
    state = await AsyncValue.guard(build);
  }

  Future<void> loadMore() async {
    final cur = state.value;
    if (cur == null || !cur.hasMore || cur.loadingMore) return;
    state = AsyncData(cur.copyWith(loadingMore: true, loadMoreFailed: false));
    try {
      final page = await ref
          .read(chatRepositoryProvider)
          .listConversations(pageToken: cur.nextPageToken);
      if (!ref.mounted) return;
      ref.read(chatSessionProvider).rememberConversations(page.items);
      final ids = cur.items.map((c) => c.id).toSet();
      state = AsyncData(
        ChatConversationsState(
          items: [...cur.items, ...page.items.where((c) => ids.add(c.id))],
          nextPageToken: page.nextPageToken,
        ),
      );
    } on Object {
      if (!ref.mounted) return;
      state = AsyncData(cur.copyWith(loadingMore: false, loadMoreFailed: true));
    }
  }

  /// 把全部對話標成已讀，列表上的未讀數歸零。
  Future<void> markAllRead() async {
    await ref.read(chatRepositoryProvider).markAllRead();
    final cur = state.value;
    if (!ref.mounted || cur == null) return;
    state = AsyncData(
      cur.copyWith(
        items: [for (final c in cur.items) c.copyWith(unreadCount: 0)],
      ),
    );
    ref.invalidate(dashboardChatUnreadProvider);
  }
}
