import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart' show kIsWeb;
import '../../../../shared/utils/hk_log.dart';

/// FCM 裝置註冊
///
/// 職責範圍：
/// - 向後端註冊當前裝置的 FCM Token（POST /app/notifications/device-token）
/// - 登出時從後端移除當前裝置 Token（DELETE /app/notifications/device-token, body: token）
/// - 自動附帶 platform 欄位（ios / web）供後端識別裝置類型
///
/// 注入的 Dio 必須是 App 的中央 `dioProvider`：憑證（`Authorization: Bearer`）只由它的
/// `AuthInterceptor` 在送出時從 session 帶上，這裡不持有、也不自己帶 session token。
///
/// Token 註冊不阻塞登入/登出主流程，但必須檢查 HTTP status 並把非 2xx 寫入 log。回應一律收下
/// （`validateStatus`），不拋 [DioException]：登出與強制登出時 session 可能已失效，註銷收到的
/// 401 是預期結果，不能再觸發一次強制登出。
///
/// 典型呼叫鏈：
/// - 登入成功 → [PushNotificationManager.onUserLoggedIn] → [registerDevice]
/// - Token 自動刷新 → [PushNotificationManager._registerDevice] → [registerDevice]
/// - 登出前 → [PushNotificationManager.onUserLoggedOut] → [removeDevice]
class NotificationDeviceRegistrar {
  NotificationDeviceRegistrar(this._dio);

  static const String _devicesPath = '/app/notifications/device-token';

  final Dio _dio;

  static String get _platform {
    if (kIsWeb) return 'web';
    return 'ios';
  }

  /// 註冊裝置 FCM Token 到後端
  Future<void> registerDevice({required String fcmToken}) async {
    // 沒有 FCM token 的註冊注定無效，不送出比送出去再記一行 error log 有意義。
    if (fcmToken.isEmpty) {
      HKLog.logW(
        '[NotificationService] registerDevice skipped: empty fcmToken',
      );
      return;
    }
    try {
      final response = await _dio.post<dynamic>(
        _devicesPath,
        data: {'token': fcmToken, 'platform': _platform},
        options: Options(validateStatus: (_) => true),
      );
      final status = response.statusCode ?? 0;
      if (status >= 200 && status < 300) {
        HKLog.logI('[NotificationService] Device registered (status=$status)');
      } else {
        HKLog.logE(
          '[NotificationService] registerDevice failed '
          'status=$status body=${response.data}',
        );
      }
    } catch (e, s) {
      HKLog.logE('[NotificationService] registerDevice error', e, s);
    }
  }

  /// 從後端移除裝置 FCM Token
  ///
  /// Backend 路由是 `DELETE /app/notifications/device-token`，
  /// token 透過 body 傳遞（不是 path param）。
  Future<void> removeDevice({required String fcmToken}) async {
    if (fcmToken.isEmpty) {
      HKLog.logW('[NotificationService] removeDevice skipped: empty fcmToken');
      return;
    }
    try {
      final response = await _dio.delete<dynamic>(
        _devicesPath,
        data: {'token': fcmToken},
        options: Options(validateStatus: (_) => true),
      );
      final status = response.statusCode ?? 0;
      if (status >= 200 && status < 300) {
        HKLog.logI('[NotificationService] Device removed (status=$status)');
      } else {
        HKLog.logE(
          '[NotificationService] removeDevice failed '
          'status=$status body=${response.data}',
        );
      }
    } catch (e, s) {
      HKLog.logE('[NotificationService] removeDevice error', e, s);
    }
  }
}
