import 'package:freezed_annotation/freezed_annotation.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'active_chat_gate.freezed.dart';
part 'active_chat_gate.g.dart';

/// 當前使用者正在聊天的對象資訊。
/// 由 chat screen mount/dispose 寫入；由 FCM/WS toast gate 讀取，
/// 避免「人就在聊天室內還跳通知」的雙重通知體驗。
@freezed
abstract class ActiveChat with _$ActiveChat {
  const factory ActiveChat({
    String? peerId, // seller 側 = buyer/customer accountId
    String? conversationId, // server 端 roomId（可能比 peerId 晚到）
  }) = _ActiveChat;
}

/// Singleton gate notifier，App 整個生命週期只有一個 state。
@Riverpod(keepAlive: true)
class ActiveChatGate extends _$ActiveChatGate {
  @override
  ActiveChat build() => const ActiveChat();

  /// Chat screen 進場呼叫，設定當前聊天對象。
  /// peerId 可為 null（通知直開 conversationId 路徑，此時 peerId 尚不明）。
  void enter({String? peerId, String? conversationId}) {
    state = ActiveChat(peerId: peerId, conversationId: conversationId);
  }

  /// 已知 conversationId 後補 patch（WS 第一則訊息回來才知道 roomId）。
  void updateConversationId(String conversationId) {
    state = state.copyWith(conversationId: conversationId);
  }

  /// Chat screen 離場呼叫，清空 gate。
  void clear() => state = const ActiveChat();
}
