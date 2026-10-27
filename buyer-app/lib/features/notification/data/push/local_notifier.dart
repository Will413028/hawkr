import 'package:flutter_local_notifications/flutter_local_notifications.dart';

import '../../../../shared/utils/hk_log.dart';

/// 本地通知服務（Stateless Service）
///
/// 封裝 [FlutterLocalNotificationsPlugin] 的初始化、
/// 通知顯示及冷啟動詳情查詢。
///
/// 職責範圍：
/// - 包裝 flutter_local_notifications SDK
/// - 顯示本地通知（靜默推播的 UI 呈現由此負責）
/// - 不儲存任何應用層狀態
class LocalNotificationService {
  final FlutterLocalNotificationsPlugin _plugin;

  const LocalNotificationService(this._plugin);

  /// 初始化 plugin 並設置本地通知點擊回調
  ///
  /// [onTap]：用戶點擊本地通知時觸發，[NotificationResponse.payload] 為
  /// background handler 寫入的 jsonEncode(data)。
  Future<void> initialize({
    required void Function(NotificationResponse) onTap,
  }) async {
    try {
      await _plugin.initialize(
        const InitializationSettings(
          iOS: DarwinInitializationSettings(
            // 通知權限由 PushNotificationManager.requestPermission() 統一申請
            requestAlertPermission: false,
            requestBadgePermission: false,
            requestSoundPermission: false,
          ),
        ),
        onDidReceiveNotificationResponse: onTap,
      );

      HKLog.logD('[LocalNotification] 初始化完成');
    } catch (e, s) {
      HKLog.logE('[LocalNotification] 初始化失敗', e, s);
    }
  }

  /// 取得通知冷啟動詳情
  ///
  /// 用於「靜默推播 → 本地通知 → 用戶點擊 → App 冷啟動」路徑的跳轉處理。
  Future<NotificationAppLaunchDetails?> getLaunchDetails() {
    return _plugin.getNotificationAppLaunchDetails();
  }
}
