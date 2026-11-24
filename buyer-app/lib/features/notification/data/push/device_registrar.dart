import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.pb.dart'
    as pb;
import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.pbenum.dart';

import '../../../../shared/utils/hk_log.dart';

/// FCM 裝置 token 的註冊與註銷（`NotificationAppService`）。
///
/// 每次 App 啟動（登入狀態還原或登入完成）、token 更新與回前景都註冊；伺服器更新最後註冊時間，同一個 token
/// 換帳號登入時改綁。請求不帶 `platform`，只帶 App 類型。登出前註銷，要在清除 session token 之前送出。
///
/// 失敗不阻塞登入與登出，只寫 log：登出與強制登出時 session 可能已失效。
class NotificationDeviceRegistrar {
  NotificationDeviceRegistrar(this._client);

  final NotificationAppServiceClient _client;

  Future<void> registerDevice({required String fcmToken}) async {
    if (fcmToken.isEmpty) {
      HKLog.logW('[NotificationService] registerDevice skipped: empty token');
      return;
    }
    try {
      await _client.registerDeviceToken(
        pb.RegisterDeviceTokenRequest(
          token: fcmToken,
          app: NotificationApp.NOTIFICATION_APP_BUYER_APP,
        ),
      );
      HKLog.logI('[NotificationService] Device registered');
    } catch (e, s) {
      HKLog.logE('[NotificationService] registerDevice error', e, s);
    }
  }

  Future<void> removeDevice({required String fcmToken}) async {
    if (fcmToken.isEmpty) {
      HKLog.logW('[NotificationService] removeDevice skipped: empty token');
      return;
    }
    try {
      await _client.deleteDeviceToken(
        pb.DeleteDeviceTokenRequest(token: fcmToken),
      );
      HKLog.logI('[NotificationService] Device removed');
    } catch (e, s) {
      HKLog.logE('[NotificationService] removeDevice error', e, s);
    }
  }
}
