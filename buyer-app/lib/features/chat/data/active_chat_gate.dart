import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'active_chat_gate.g.dart';

/// 當前聊天對象狀態 — 用於抑制重複 FCM／串流 banner（主 isolate 前景）。對話 ID 是推播與串流共同的識別；
/// 從店面進來、對話還不存在時只有 [storeId]。
class ActiveChat {
  const ActiveChat({this.storeId, this.conversationId});
  const ActiveChat.empty() : storeId = null, conversationId = null;

  final String? storeId;
  final String? conversationId;

  bool get isEmpty => storeId == null && conversationId == null;

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is ActiveChat &&
          storeId == other.storeId &&
          conversationId == other.conversationId;

  @override
  int get hashCode => Object.hash(storeId, conversationId);
}

@Riverpod(keepAlive: true)
class ActiveChatGate extends _$ActiveChatGate {
  @override
  ActiveChat build() => const ActiveChat.empty();

  void enter({String? storeId, String? conversationId}) {
    state = ActiveChat(storeId: storeId, conversationId: conversationId);
  }

  void clear() {
    state = const ActiveChat.empty();
  }
}
