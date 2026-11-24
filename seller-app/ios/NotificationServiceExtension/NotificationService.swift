import UserNotifications

/// 推播圖片（D14）：伺服器的 FCM payload 帶 `fcm_options.image`（HTTPS URL）與 `mutable-content: 1`，
/// 這個 extension 在通知顯示前下載圖片並附加。下載失敗或逾時就照原樣顯示文字。
final class NotificationService: UNNotificationServiceExtension {
  private var contentHandler: ((UNNotificationContent) -> Void)?
  private var bestAttempt: UNMutableNotificationContent?
  private var download: URLSessionDownloadTask?
  private let lock = NSLock()
  private var delivered = false

  override func didReceive(
    _ request: UNNotificationRequest,
    withContentHandler contentHandler: @escaping (UNNotificationContent) -> Void
  ) {
    self.contentHandler = contentHandler
    bestAttempt = request.content.mutableCopy() as? UNMutableNotificationContent

    guard let url = NotificationService.imageURL(from: request.content.userInfo) else {
      deliver(request.content)
      return
    }

    download = URLSession.shared.downloadTask(with: url) { [weak self] location, response, error in
      guard let self = self else { return }
      if error == nil, let location = location,
        let attachment = NotificationService.attachment(from: location, response: response, url: url)
      {
        self.bestAttempt?.attachments = [attachment]
      }
      self.deliver(self.bestAttempt ?? request.content)
    }
    download?.resume()
  }

  override func serviceExtensionTimeWillExpire() {
    download?.cancel()
    if let content = bestAttempt {
      deliver(content)
    }
  }

  private func deliver(_ content: UNNotificationContent) {
    lock.lock()
    defer { lock.unlock() }
    if delivered { return }
    delivered = true
    contentHandler?(content)
  }

  /// `fcm_options.image` 必須是 HTTPS URL；其他值當作沒有圖片。
  private static func imageURL(from userInfo: [AnyHashable: Any]) -> URL? {
    guard let options = userInfo["fcm_options"] as? [String: Any],
      let raw = options["image"] as? String,
      let url = URL(string: raw),
      url.scheme?.lowercased() == "https"
    else { return nil }
    return url
  }

  /// 附件檔需要副檔名才能被系統辨識類型，所以搬到帶副檔名的暫存檔。
  private static func attachment(
    from location: URL, response: URLResponse?, url: URL
  ) -> UNNotificationAttachment? {
    let ext = fileExtension(mimeType: response?.mimeType, url: url)
    let target = URL(fileURLWithPath: NSTemporaryDirectory())
      .appendingPathComponent(UUID().uuidString)
      .appendingPathExtension(ext)
    do {
      try FileManager.default.moveItem(at: location, to: target)
      return try UNNotificationAttachment(identifier: "image", url: target, options: nil)
    } catch {
      return nil
    }
  }

  private static func fileExtension(mimeType: String?, url: URL) -> String {
    switch mimeType?.lowercased() {
    case "image/png": return "png"
    case "image/gif": return "gif"
    case "image/webp": return "webp"
    case "image/jpeg", "image/jpg": return "jpg"
    default:
      let ext = url.pathExtension.lowercased()
      return ext.isEmpty ? "jpg" : ext
    }
  }
}
