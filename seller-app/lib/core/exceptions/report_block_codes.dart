/// 檢舉／封鎖相關業務碼（backend-payuni#645「檢舉／封鎖」內容治理）。
///
/// 對應 Gateway envelope 的 `code` 欄位（統一回應格式，見 hawkr
/// backend/docs/開發規範/API 設計規範.md）。用於 call-site 層攔截，非全域 interceptor
/// （9202 / 9210 只在特定操作——傳訊息、發留言——才會遇到）。
abstract final class ReportBlockCode {
  /// 傳送訊息／留言被拒 — 我已被對方封鎖（或我已封鎖對方），內容無法送達
  static const int blockedMessageNotAllowed = 9202;

  /// 內容未通過審核 — 貼文／留言／聊天訊息違反內容政策被拒
  static const int moderationRejected = 9210;
}
