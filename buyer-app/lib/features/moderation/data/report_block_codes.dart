/// 檢舉／封鎖相關的後端業務碼（bizcode，走 gateway 統一回應 envelope 的 `code` 欄位）。
class ReportBlockCode {
  const ReportBlockCode._();

  /// 傳訊息給已封鎖你或你已封鎖的人。
  static const int blockedMessageNotAllowed = 9202;

  /// 內容命中字典過濾（內容審核拒絕）。
  static const int moderationRejected = 9210;
}

/// 將 [ReportBlockCode] 業務碼轉成使用者可讀的中文訊息。
///
/// [code] 通常來自 `response.data['code']`，型別可能是 int（Dio JSON decode
/// 後的常見型別）或 String（少數手寫 HTTP 呼叫路徑），這裡一律先轉型再比對。
/// 不在已知清單內回傳 `null`，呼叫端應 fallback 到既有的通用錯誤訊息。
String? reportBlockErrorMessage(dynamic code) {
  final intCode = switch (code) {
    int v => v,
    num v => v.toInt(),
    String v => int.tryParse(v),
    _ => null,
  };
  if (intCode == null) return null;
  if (intCode == ReportBlockCode.blockedMessageNotAllowed) {
    return '對方已封鎖您，或您已封鎖對方，無法傳送訊息';
  }
  if (intCode == ReportBlockCode.moderationRejected) {
    return '訊息內容包含不當字詞，請修改後重試';
  }
  return null;
}
