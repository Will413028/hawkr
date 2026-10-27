import Flutter
import UIKit
import UserNotifications
import FirebaseCore

@main
@objc class AppDelegate: FlutterAppDelegate {
  override func application(
    _ application: UIApplication,
    didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?
  ) -> Bool {
    // 原生端初始化 Firebase，確保 firebase_messaging 插件註冊時 Firebase 已就緒
    // Dart 端 Firebase.initializeApp() 會 catch duplicate-app，不衝突
    // 沒有 GoogleService-Info.plist，或其內容缺 GOOGLE_APP_ID 或 API_KEY（Firebase 未設定）時不初始化，推播與分析停用
    if let plistPath = Bundle.main.path(forResource: "GoogleService-Info", ofType: "plist"),
       let options = FirebaseOptions(contentsOfFile: plistPath),
       !options.googleAppID.isEmpty,
       !(options.apiKey ?? "").isEmpty {
      FirebaseApp.configure()
    }

    // 傳統方式註冊所有 Flutter 插件（不使用 FlutterImplicitEngineDelegate，
    // 避免延遲註冊導致 APNs token 到達時 firebase_messaging 尚未就緒）
    GeneratedPluginRegistrant.register(with: self)

    return super.application(application, didFinishLaunchingWithOptions: launchOptions)
  }

  // MARK: - UNUserNotificationCenterDelegate

  /// 前台通知：必須呼叫 super 讓 firebase_messaging 處理 → 觸發 onMessage
  /// 顯示選項由 Flutter 端 setForegroundNotificationPresentationOptions 控制
  override func userNotificationCenter(
    _ center: UNUserNotificationCenter,
    willPresent notification: UNNotification,
    withCompletionHandler completionHandler: @escaping (UNNotificationPresentationOptions) -> Void
  ) {
    super.userNotificationCenter(center, willPresent: notification, withCompletionHandler: completionHandler)
  }

  /// 用戶點擊通知：必須呼叫 super 讓 firebase_messaging 處理 → 觸發 onMessageOpenedApp
  override func userNotificationCenter(
    _ center: UNUserNotificationCenter,
    didReceive response: UNNotificationResponse,
    withCompletionHandler completionHandler: @escaping () -> Void
  ) {
    super.userNotificationCenter(center, didReceive: response, withCompletionHandler: completionHandler)
  }

  // MARK: - APNs Token

  /// APNs 註冊成功：必須呼叫 super 讓 firebase_messaging 取得 device token
  override func application(
    _ application: UIApplication,
    didRegisterForRemoteNotificationsWithDeviceToken deviceToken: Data
  ) {
    super.application(application, didRegisterForRemoteNotificationsWithDeviceToken: deviceToken)
  }

  /// APNs 註冊失敗
  override func application(
    _ application: UIApplication,
    didFailToRegisterForRemoteNotificationsWithError error: Error
  ) {
    super.application(application, didFailToRegisterForRemoteNotificationsWithError: error)
  }
}
