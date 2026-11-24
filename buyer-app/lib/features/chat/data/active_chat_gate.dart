import 'package:riverpod_annotation/riverpod_annotation.dart';

import 'chat_repository.dart';

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

/// 使用者正開著的聊天室是不是 [conversationId] 這個對話：對話 ID 相符，或從店面進來還沒有對話 ID 時
/// 這個對話屬於同一家店。[gate] 在讀店家資料之後重讀，使用者可能在等待時離開。
Future<bool> activeChatCovers(
  ActiveChat Function() gate,
  ChatRepository repo,
  String conversationId,
) async {
  var current = gate();
  if (current.conversationId == conversationId) return true;
  final storeId = current.storeId;
  if (current.conversationId != null || storeId == null) return false;
  var card = repo.storeCards[conversationId];
  if (card == null) {
    try {
      await repo.listAllConversations();
    } catch (_) {
      return false;
    }
    card = repo.storeCards[conversationId];
    current = gate();
    if (current.conversationId == conversationId) return true;
    if (current.conversationId != null || current.storeId != storeId) {
      return false;
    }
  }
  return card?.id == storeId;
}

/// 使用者正開著的聊天室是不是店家 [storeId]（通知深連結 `/account/messages/{targetId}` 的 targetId 是店家）：
/// 從店面進來直接比店家，從對話進來用 [storeIdOfConversation] 取對話的店家。
bool activeChatIsStore(
  ActiveChat gate,
  String storeId,
  String? Function(String conversationId) storeIdOfConversation,
) {
  if (storeId.isEmpty) return false;
  if (gate.storeId == storeId) return true;
  final conversationId = gate.conversationId;
  return conversationId != null &&
      storeIdOfConversation(conversationId) == storeId;
}
