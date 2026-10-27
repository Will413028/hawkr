import Flutter
import UIKit
import FirebaseCore
import FirebaseMessaging

// MARK: - HKLog Helper
private func HKLog(_ message: String, file: String = #file, function: String = #function, line: Int = #line) {
    #if DEBUG
    let fileName = (file as NSString).lastPathComponent
    print("[HKLog][\(fileName):\(line)] \(message)")
    #endif
}

@main
@objc class AppDelegate: FlutterAppDelegate {
  override func application(
    _ application: UIApplication,
    didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?
  ) -> Bool {
    HKLog("[AppDelegate] didFinishLaunchingWithOptions")

    // 沒有 GoogleService-Info.plist，或其內容缺 GOOGLE_APP_ID 或 API_KEY（Firebase 未設定）時不初始化，推播與分析停用
    if let plistPath = Bundle.main.path(forResource: "GoogleService-Info", ofType: "plist"),
       let options = FirebaseOptions(contentsOfFile: plistPath),
       !options.googleAppID.isEmpty,
       !(options.apiKey ?? "").isEmpty {
      FirebaseApp.configure()
    }
    GeneratedPluginRegistrant.register(with: self)
    HKLog("[AppDelegate] Plugin registration complete")

    // Register Apple Vision OCR Plugin
    AppleVisionOcrPlugin.register(with: self.registrar(forPlugin: "AppleVisionOcrPlugin")!)
    HKLog("[AppDelegate] AppleVisionOcrPlugin registered")

    return super.application(application, didFinishLaunchingWithOptions: launchOptions)
  }

  @available(iOS 10.0, *)
  override func userNotificationCenter(_ center: UNUserNotificationCenter,
                                       willPresent notification: UNNotification,
                                       withCompletionHandler completionHandler: @escaping (UNNotificationPresentationOptions) -> Void) {
    HKLog("[Notification] willPresent: \(notification.request.identifier)")
    super.userNotificationCenter(center, willPresent: notification, withCompletionHandler: completionHandler)
  }

  @available(iOS 10.0, *)
  override func userNotificationCenter(_ center: UNUserNotificationCenter,
                                       didReceive response: UNNotificationResponse,
                                       withCompletionHandler completionHandler: @escaping () -> Void) {
    HKLog("[Notification] didReceive response: \(response.notification.request.identifier)")
    super.userNotificationCenter(center, didReceive: response, withCompletionHandler: completionHandler)
  }

  override func application(_ application: UIApplication, didRegisterForRemoteNotificationsWithDeviceToken deviceToken: Data) {
    let tokenString = deviceToken.map { String(format: "%02.2hhx", $0) }.joined()
    HKLog("[APNs] Registered for remote notifications, token: \(tokenString.prefix(20))...")
    super.application(application, didRegisterForRemoteNotificationsWithDeviceToken: deviceToken)
  }

  override func application(_ application: UIApplication, didFailToRegisterForRemoteNotificationsWithError error: Error) {
    HKLog("[APNs] Failed to register for remote notifications: \(error.localizedDescription)")
    super.application(application, didFailToRegisterForRemoteNotificationsWithError: error)
  }
}
