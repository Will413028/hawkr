import 'dart:async';

import 'package:flutter/widgets.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:rxdart/rxdart.dart';

import '../../../core/l10n/ui_locale.dart';
import '../presentation/chat_preview.dart';
import 'chat_model.dart';
import 'chat_repository.dart';
import 'chat_stream_client.dart';

part 'chat_session.g.dart';

/// 新訊息的 in-app 通知（App 在前景、使用者不在那個對話裡時）。
class ChatNotificationEvent {
  const ChatNotificationEvent({
    required this.unreadCount,
    required this.conversationId,
    required this.buyerId,
    required this.preview,
    this.senderName,
    this.senderAvatar,
  });

  final int unreadCount;
  final String conversationId;
  final String buyerId;
  final String preview;
  final String? senderName;
  final String? senderAvatar;
}

/// 登入後整個 App 共用的一條店家事件串流（同一個身分的並行串流有上限，所以只開一條）。
/// 對話頁與列表訂閱 [events]；總未讀數與新訊息通知由這裡統一整理。
class ChatSession with WidgetsBindingObserver {
  ChatSession(this._repo, this._locale, {ChatStreamClient? client})
    : client = client ?? ChatStreamClient(open: _repo.stream) {
    _eventSub = this.client.events.listen(_onEvent);
  }

  final ChatRepository _repo;
  final UiLocale Function() _locale;
  final ChatStreamClient client;
  late final StreamSubscription<ChatStreamEvent> _eventSub;

  final _unread = BehaviorSubject<int>.seeded(0);
  final _notifications = StreamController<ChatNotificationEvent>.broadcast();
  final Map<String, ChatConversation> _conversations = {};
  bool _observing = false;

  Stream<ChatStreamEvent> get events => client.events;
  Stream<ChatStreamStatus> get statuses => client.statuses;
  ChatStreamStatus get status => client.status;

  /// 總未讀數（串流的 Ready 與 UnreadChanged 更新）。
  ValueStream<int> get unread => _unread;
  Stream<ChatNotificationEvent> get onNewMessage => _notifications.stream;

  /// 對話列表載入時記下買家資料，新訊息通知用它補名稱與頭像。
  void rememberConversations(Iterable<ChatConversation> items) {
    for (final c in items) {
      _conversations[c.id] = c;
    }
  }

  /// 列表載入時記下的對話；對話頁開啟時先用它，省去逐頁翻列表。
  ChatConversation? conversationFor({String? conversationId, String? buyerId}) {
    if ((conversationId ?? '').isNotEmpty)
      return _conversations[conversationId];
    if ((buyerId ?? '').isEmpty) return null;
    for (final c in _conversations.values) {
      if (c.buyer.id == buyerId) return c;
    }
    return null;
  }

  void start() {
    if (!_observing) {
      WidgetsBinding.instance.addObserver(this);
      _observing = true;
    }
    client.start();
  }

  void stop() {
    if (_observing) {
      WidgetsBinding.instance.removeObserver(this);
      _observing = false;
    }
    client.stop();
    _conversations.clear();
    _unread.add(0);
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed) client.reconnectNow();
  }

  Future<void> dispose() async {
    stop();
    await _eventSub.cancel();
    await client.dispose();
    await _unread.close();
    await _notifications.close();
  }

  void _onEvent(ChatStreamEvent e) {
    switch (e) {
      case ChatReady(:final unreadCount):
        _unread.add(unreadCount);
      case ChatUnreadChanged(:final unreadCount):
        _unread.add(unreadCount);
      case ChatMessageArrived(:final message, :final updated):
        if (!updated && !message.fromStore && !message.isTombstone) {
          unawaited(_notify(message));
        }
      case ChatPeerRead():
      case ChatResync():
        break;
    }
  }

  Future<void> _notify(ChatMessage m) async {
    var conv = _conversations[m.conversationId];
    if (conv == null) {
      try {
        final page = await _repo.listConversations();
        rememberConversations(page.items);
        conv = _conversations[m.conversationId];
      } on Object {
        // 沒有買家資料時仍通知，只是沒有名稱。
      }
    }
    if (_notifications.isClosed) return;
    final preview = previewText(
      ChatPreview(
        kind: m.kind,
        text: m.text,
        hidden: m.hidden,
        erased: m.erased,
        fromStore: m.fromStore,
      ),
      _locale(),
    );
    _notifications.add(
      ChatNotificationEvent(
        unreadCount: _unread.value,
        conversationId: m.conversationId,
        buyerId: conv?.buyer.id ?? '',
        preview: preview,
        senderName: conv?.buyer.name,
        senderAvatar: conv?.buyer.avatarUri,
      ),
    );
  }
}

@Riverpod(keepAlive: true)
ChatSession chatSession(Ref ref) {
  final session = ChatSession(
    ref.watch(chatRepositoryProvider),
    () => ref.read(uiLocaleProvider),
  );
  ref.onDispose(session.dispose);
  return session;
}
