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

  /// 每次整個重讀（build）加一：在那之前發出的載入下一頁，結果屬於舊的列表，丟掉。
  int _epoch = 0;

  @override
  Future<ChatConversationsState> build() async {
    _epoch++;
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

  /// 重讀第一頁（背景更新）：第一頁併進已載入的列（新的在前、依 id 去重），不收掉已載入的後續頁；失敗時留著畫面上已有的列。
  Future<void> refresh() async {
    if (state.value == null) return;
    final epoch = _epoch;
    try {
      final page = await ref.read(chatRepositoryProvider).listConversations();
      final cur = state.value;
      if (!ref.mounted || cur == null || epoch != _epoch) return;
      ref.read(chatSessionProvider).rememberConversations(page.items);
      final ids = page.items.map((c) => c.id).toSet();
      state = AsyncData(
        cur.copyWith(
          items: [
            ...page.items,
            ...cur.items.where((c) => !ids.contains(c.id)),
          ],
          // 續頁的 token 以目前的為準（第一頁的 token 指向已載入的第二頁）。
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
    final epoch = _epoch;
    try {
      final page = await ref
          .read(chatRepositoryProvider)
          .listConversations(pageToken: cur.nextPageToken);
      final latest = state.value;
      if (!ref.mounted || latest == null || epoch != _epoch) return;
      ref.read(chatSessionProvider).rememberConversations(page.items);
      final ids = latest.items.map((c) => c.id).toSet();
      state = AsyncData(
        ChatConversationsState(
          items: [...latest.items, ...page.items.where((c) => ids.add(c.id))],
          nextPageToken: page.nextPageToken,
        ),
      );
    } on Object {
      final latest = state.value;
      if (!ref.mounted || latest == null || epoch != _epoch) return;
      state = AsyncData(
        latest.copyWith(loadingMore: false, loadMoreFailed: true),
      );
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
