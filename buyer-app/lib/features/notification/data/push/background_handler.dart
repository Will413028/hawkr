import 'dart:convert';

import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter_local_notifications/flutter_local_notifications.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../../../../shared/utils/hk_log.dart';
import 'destination.dart';

/// FCM 訊息去重快取的 SharedPreferences key。無帳號 scope，只存訊息 ID（無
/// PII），登出時由主 isolate 直接清（見 [clearFcmProcessedIds]）——背景 isolate
/// 收不到主 isolate 的登出事件，但 SharedPreferences 是跨 isolate 共用的持久化
/// 存儲，主 isolate 寫入/清除的結果背景 isolate 下次讀取即可見。
const fcmProcessedIdsKey = 'fcm_processed_ids';

/// 帳號切換/登出時呼叫，清除 FCM 去重快取。
Future<void> clearFcmProcessedIds() async {
  final prefs = await SharedPreferences.getInstance();
  await prefs.remove(fcmProcessedIdsKey);
}

/// FCM 背景訊息處理器
///
/// 運行在獨立 Dart isolate，與主 isolate 完全隔離：
/// - 不能訪問任何主 isolate 的單例（如 PushNotificationManager.instance）
/// - Firebase / flutter_local_notifications 需在此 isolate 內獨立初始化
/// - 必須是頂層函數，並標記 @pragma('vm:entry-point')
///
/// 觸發時機：App 在背景或終止狀態收到 FCM data-only 訊息時由系統喚起。
///
/// 處理邏輯：
/// - 帶 notification 欄位 → OS 已自動顯示系統通知，直接 return（避免重複）
/// - 重複 messageId（FCM at-least-once）→ 直接 return（避免重複顯示）
/// - data-only → 優先讀 data['title'] / data['body']；
///   chat_message 額外 fallback 至 content_preview
@pragma('vm:entry-point')
Future<void> firebaseMessagingBackgroundHandler(RemoteMessage message) async {
  // 背景 isolate 與主 isolate 記憶體隔離，Firebase 需重新初始化
  // 檢查 apps.isEmpty 避免同一 isolate 內重複初始化拋出 DuplicateApp 異常
  if (Firebase.apps.isEmpty) {
    await Firebase.initializeApp();
  }

  HKLog.logD('[FCM-BG] messageId: ${message.messageId}');
  HKLog.logD('[FCM-BG] hasNotification: ${message.notification != null}');
  HKLog.logD('[FCM-BG] data: ${message.data}');

  // 帶 notification 欄位：OS 已自動顯示系統通知，無需額外處理
  if (message.notification != null) return;

  // FCM 保證 at-least-once 投遞，同一訊息可能觸發多次 handler
  // 用 SharedPreferences 持久化已處理的 messageId 進行去重
  final messageId = message.messageId;
  if (messageId != null) {
    final prefs = await SharedPreferences.getInstance();
    final processed = prefs.getStringList(fcmProcessedIdsKey) ?? [];
    if (processed.contains(messageId)) {
      HKLog.logW('[FCM-BG] 重複訊息，跳過: $messageId');
      return;
    }
    // 記錄已處理，最多保留 50 筆避免無限增長
    processed.add(messageId);
    if (processed.length > 50) processed.removeRange(0, processed.length - 50);
    await prefs.setStringList(fcmProcessedIdsKey, processed);
  }

  final data = message.data;
  final type = data['type'] as String? ?? '';
  final normalizedType = type.toLowerCase();
  final senderName = _firstNonEmpty(data, [
    'sender_name',
    'senderName',
    'from_name',
    'fromName',
  ]);

  if (_isSilentPush(data)) {
    HKLog.logD('[FCM-BG] silent push，略過本地通知顯示');
    return;
  }

  // 後端始終在 data payload 中提供 title / body
  // chat_message 額外 fallback 至 content_preview（向後相容）
  final title = (data['title'] as String? ?? '').isNotEmpty
      ? data['title'] as String
      : (normalizedType == 'chat_message'
            ? (senderName.isNotEmpty ? senderName : '新訊息')
            : '');

  final body = (data['body'] as String? ?? '').isNotEmpty
      ? data['body'] as String
      : (normalizedType == 'chat_message'
            ? data['content_preview'] as String? ?? ''
            : '');

  if (title.isEmpty && body.isEmpty) {
    HKLog.logW('[FCM-BG] title 與 body 均為空，略過顯示');
    return;
  }

  // 背景 isolate 需獨立初始化 plugin，不共享主 isolate 的實例
  final plugin = FlutterLocalNotificationsPlugin();
  await plugin.initialize(
    const InitializationSettings(iOS: DarwinInitializationSettings()),
  );

  final navigationPayload = <String, dynamic>{
    ...data,
    if (messageId != null && messageId.isNotEmpty)
      notificationSourceMessageIdKey: messageId,
  };

  await plugin.show(
    message.hashCode,
    title,
    body,
    const NotificationDetails(
      iOS: DarwinNotificationDetails(
        presentAlert: true,
        presentBadge: true,
        presentSound: true,
      ),
    ),
    // payload 傳入完整 data map（JSON 格式），用戶點擊通知時用於頁面跳轉
    payload: jsonEncode(navigationPayload),
  );

  HKLog.logI('[FCM-BG] 本地通知已顯示: $title');
}

bool _isSilentPush(Map<String, dynamic> data) {
  final mode = (data['push_mode'] ?? data['pushMode'] ?? data['mode'])
      ?.toString();
  if (mode?.toLowerCase() == 'silent') return true;

  final silent = data['silent']?.toString().toLowerCase();
  return silent == 'true' || silent == '1';
}

String _firstNonEmpty(Map<String, dynamic> data, List<String> keys) {
  for (final key in keys) {
    final value = data[key]?.toString().trim();
    if (value != null && value.isNotEmpty) return value;
  }
  return '';
}
