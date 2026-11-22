//
//  Generated code. Do not modify.
//  source: hawkr/chat/v1alpha1/chat.proto
//

import "package:connectrpc/connect.dart" as connect;
import "chat.pb.dart" as hawkrchatv1alpha1chat;

/// ChatAppService 是登入者的聊天。
abstract final class ChatAppService {
  /// Fully-qualified name of the ChatAppService service.
  static const name = 'hawkr.chat.v1alpha1.ChatAppService';

  /// ListConversations 列出呼叫者（買家）的對話，最近有訊息的在前（最後訊息時間，相同時以 ID 排序）。不因封鎖隱藏對話，不回總數。
  static const listConversations = connect.Spec(
    '/$name/ListConversations',
    connect.StreamType.unary,
    hawkrchatv1alpha1chat.ListConversationsRequest.new,
    hawkrchatv1alpha1chat.ListConversationsResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// ListStoreConversations 列出呼叫者此刻的店的對話，規則同 ListConversations。呼叫者沒有店回 not_found；店家關閉之後沒有權限，回 not_found。
  static const listStoreConversations = connect.Spec(
    '/$name/ListStoreConversations',
    connect.StreamType.unary,
    hawkrchatv1alpha1chat.ListStoreConversationsRequest.new,
    hawkrchatv1alpha1chat.ListStoreConversationsResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// ListMessages 列出對話裡的訊息。預設與 before_seq 是 seq 由大到小（新的在前），before_seq 只列 seq 小於它的訊息；
  /// after_seq 給斷線補洞，只列 seq 大於它的訊息，seq 由小到大（舊的在前）。before_seq 與 after_seq 不能同時給。
  /// 呼叫者不是對話的任一方，與對話不存在一律回 not_found，不區分原因。已隱藏或已抹除的訊息是墓碑，不帶內容。
  static const listMessages = connect.Spec(
    '/$name/ListMessages',
    connect.StreamType.unary,
    hawkrchatv1alpha1chat.ListMessagesRequest.new,
    hawkrchatv1alpha1chat.ListMessagesResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// SendMessage 對既有的對話送出一則訊息，寄件方由對話的歸屬推導。同一個對話、同一個寄件方以同一個 request_id 重送，
  /// 回第一次寫入的那則訊息；內容不同回 invalid_argument。對方封鎖、店家停業、對方帳號已刪除等見 ErrorReason。
  static const sendMessage = connect.Spec(
    '/$name/SendMessage',
    connect.StreamType.unary,
    hawkrchatv1alpha1chat.SendMessageRequest.new,
    hawkrchatv1alpha1chat.SendMessageResponse.new,
  );

  /// SendMessageToStore 以買家身分對一家店送出訊息：沒有對話時開新對話，有就接續。其餘規則同 SendMessage。店家不存在回 not_found。
  static const sendMessageToStore = connect.Spec(
    '/$name/SendMessageToStore',
    connect.StreamType.unary,
    hawkrchatv1alpha1chat.SendMessageToStoreRequest.new,
    hawkrchatv1alpha1chat.SendMessageToStoreResponse.new,
  );

  /// SendMessageToBuyer 以呼叫者此刻的店的身分對一位買家送出訊息：沒有對話時開新對話，有就接續。其餘規則同 SendMessage。
  /// 呼叫者沒有店或買家不存在回 not_found。
  static const sendMessageToBuyer = connect.Spec(
    '/$name/SendMessageToBuyer',
    connect.StreamType.unary,
    hawkrchatv1alpha1chat.SendMessageToBuyerRequest.new,
    hawkrchatv1alpha1chat.SendMessageToBuyerResponse.new,
  );

  /// MarkConversationRead 把呼叫者這一方在對話裡的已讀游標推到 upto_seq：游標只前進，超過對話最後一個 seq 以最後一個計。
  static const markConversationRead = connect.Spec(
    '/$name/MarkConversationRead',
    connect.StreamType.unary,
    hawkrchatv1alpha1chat.MarkConversationReadRequest.new,
    hawkrchatv1alpha1chat.MarkConversationReadResponse.new,
  );

  /// MarkAllRead 把呼叫者（買家）的全部對話標成已讀。
  static const markAllRead = connect.Spec(
    '/$name/MarkAllRead',
    connect.StreamType.unary,
    hawkrchatv1alpha1chat.MarkAllReadRequest.new,
    hawkrchatv1alpha1chat.MarkAllReadResponse.new,
  );

  /// MarkAllStoreRead 把呼叫者此刻的店的全部對話標成已讀。
  static const markAllStoreRead = connect.Spec(
    '/$name/MarkAllStoreRead',
    connect.StreamType.unary,
    hawkrchatv1alpha1chat.MarkAllStoreReadRequest.new,
    hawkrchatv1alpha1chat.MarkAllStoreReadResponse.new,
  );

  /// GetUnreadCount 是呼叫者（買家）的總未讀訊息數。
  static const getUnreadCount = connect.Spec(
    '/$name/GetUnreadCount',
    connect.StreamType.unary,
    hawkrchatv1alpha1chat.GetUnreadCountRequest.new,
    hawkrchatv1alpha1chat.GetUnreadCountResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// GetStoreUnreadCount 是呼叫者此刻的店的總未讀訊息數。
  static const getStoreUnreadCount = connect.Spec(
    '/$name/GetStoreUnreadCount',
    connect.StreamType.unary,
    hawkrchatv1alpha1chat.GetStoreUnreadCountRequest.new,
    hawkrchatv1alpha1chat.GetStoreUnreadCountResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// GetPresence 回報呼叫者（買家）的對話的對方此刻是否在線，最多一次 50 個對話。沒有最後上線時間。
  /// 其中任何一個不是呼叫者的對話回 not_found（整個請求失敗）。任一方向有封鎖的對話回離線。
  static const getPresence = connect.Spec(
    '/$name/GetPresence',
    connect.StreamType.unary,
    hawkrchatv1alpha1chat.GetPresenceRequest.new,
    hawkrchatv1alpha1chat.GetPresenceResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// GetStorePresence 同 GetPresence，對象是呼叫者此刻的店的對話。
  static const getStorePresence = connect.Spec(
    '/$name/GetStorePresence',
    connect.StreamType.unary,
    hawkrchatv1alpha1chat.GetStorePresenceRequest.new,
    hawkrchatv1alpha1chat.GetStorePresenceResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );
}
