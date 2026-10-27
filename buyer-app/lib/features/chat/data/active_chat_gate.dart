import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'active_chat_gate.g.dart';

/// 當前聊天對象狀態 — 用於抑制重複 FCM/WS banner（主 isolate 前景）。
class ActiveChat {
  const ActiveChat({this.peerId = '', this.conversationId});
  const ActiveChat.empty() : peerId = '', conversationId = null;

  final String peerId; // buyer: merchantId; seller: participantId
  final String? conversationId;

  bool get isEmpty => peerId.isEmpty;

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is ActiveChat &&
          peerId == other.peerId &&
          conversationId == other.conversationId;

  @override
  int get hashCode => Object.hash(peerId, conversationId);
}

@Riverpod(keepAlive: true)
class ActiveChatGate extends _$ActiveChatGate {
  @override
  ActiveChat build() => const ActiveChat.empty();

  void enter({required String peerId, String? conversationId}) {
    state = ActiveChat(peerId: peerId, conversationId: conversationId);
  }

  void clear() {
    state = const ActiveChat.empty();
  }
}
