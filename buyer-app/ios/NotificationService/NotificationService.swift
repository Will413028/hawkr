import UserNotifications

/// Downloads the push image and attaches it to the visible notification (plan A8 D14).
///
/// FCM puts the `fcm_options.image` URL into the APNs payload's userInfo and the
/// backend sets `mutable-content: 1`, so iOS hands the notification to this extension
/// before it is shown. Any failure delivers the notification without an image.
class NotificationService: UNNotificationServiceExtension {
  private var contentHandler: ((UNNotificationContent) -> Void)?
  private var bestAttemptContent: UNMutableNotificationContent?
  private var downloadTask: URLSessionDownloadTask?

  override func didReceive(
    _ request: UNNotificationRequest,
    withContentHandler contentHandler: @escaping (UNNotificationContent) -> Void
  ) {
    self.contentHandler = contentHandler
    let content = (request.content.mutableCopy() as? UNMutableNotificationContent)
    bestAttemptContent = content

    guard let content = content,
      let options = request.content.userInfo["fcm_options"] as? [String: Any],
      let urlString = options["image"] as? String,
      let url = URL(string: urlString),
      url.scheme == "https"
    else {
      contentHandler(request.content)
      return
    }

    let task = URLSession.shared.downloadTask(with: url) { [weak self] location, response, error in
      guard let self = self else { return }
      defer { self.contentHandler?(self.bestAttemptContent ?? content) }
      guard let location = location, error == nil else { return }
      let ext = Self.fileExtension(for: response?.mimeType, url: url)
      let target = FileManager.default.temporaryDirectory
        .appendingPathComponent(UUID().uuidString)
        .appendingPathExtension(ext)
      do {
        try FileManager.default.moveItem(at: location, to: target)
        let attachment = try UNNotificationAttachment(identifier: "image", url: target)
        self.bestAttemptContent?.attachments = [attachment]
      } catch {
        // Deliver without the image.
      }
    }
    downloadTask = task
    task.resume()
  }

  override func serviceExtensionTimeWillExpire() {
    downloadTask?.cancel()
    if let contentHandler = contentHandler, let content = bestAttemptContent {
      contentHandler(content)
    }
  }

  private static func fileExtension(for mimeType: String?, url: URL) -> String {
    switch mimeType {
    case "image/png": return "png"
    case "image/gif": return "gif"
    case "image/webp": return "webp"
    case "image/jpeg": return "jpg"
    default:
      let ext = url.pathExtension
      return ext.isEmpty ? "jpg" : ext
    }
  }
}
