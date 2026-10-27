import 'package:hawkr_buyer/core/codemeta/code_meta.dart';

/// 檢舉相關碼表的值域一律取自生成的 CodeMeta（`core/codemeta/code_meta.dart`），
/// 本檔只放「哪些選項適用於哪種對象」與「畫面上要寫什麼字」這兩類呈現層規則。
///
/// 重新 export 是為了讓既有呼叫端不必同時 import 兩個檔——同時 import
/// `code_meta.dart` 與本檔時拿到的是同一個宣告，不會撞名。
export 'package:hawkr_buyer/core/codemeta/code_meta.dart'
    show ReportReasonCode, ReportStatus, ReportTargetType;

/// 依 [targetType] 決定要顯示哪些檢舉原因選項。
/// 檢舉「使用者」時不顯示食品安全疑慮（該原因僅適用於商品相關內容）。
List<ReportReasonCode> reportReasonsFor(ReportTargetType targetType) {
  if (targetType == ReportTargetType.user) {
    return ReportReasonCode.values
        .where((r) => r != ReportReasonCode.foodSafety)
        .toList();
  }
  return ReportReasonCode.values;
}

/// 選單上的原因文案 = 碼表 label + 表單提示。
///
/// 「（請說明）」是提示使用者選了這項要補描述（[ReportSheetWidget] 會把送出鈕
/// 鎖住），屬於這張表單自己的文案，不是碼表的一部分，所以在呈現層組。
String reportReasonOptionLabel(ReportReasonCode reason) =>
    reason == ReportReasonCode.other ? '${reason.label}（請說明）' : reason.label;

/// `reason_code` → 中文；認不得的值原樣回傳。
String reportReasonLabel(String reasonCode) =>
    ReportReasonCode.fromCode(reasonCode)?.label ?? reasonCode;

/// `target_type` → 中文；認不得的值原樣回傳。
String reportTargetTypeLabel(String targetType) =>
    ReportTargetType.fromValue(targetType)?.label ?? targetType;

/// `status` → 檢舉人視角的中文。
///
/// 值域來自碼表：switch 不寫 default，碼表日後多一個狀態時這裡編譯不過，
/// 不會像手抄對照表那樣安靜地漏掉。文案則是本畫面自己的——碼表 label 是
/// 治理後台視角（待處理／已隱藏／已警告），檢舉人要看的是「我這張單走到哪」。
///
/// 認不得的值原樣回傳：顯示原碼至少看得出有東西，比整格空白好。
String reportStatusLabel(String status) {
  final normalized = status.trim().toUpperCase();
  if (normalized.isEmpty) return '審核中';
  final meta = ReportStatus.fromValue(normalized);
  if (meta == null) return status;
  return switch (meta) {
    ReportStatus.pending => '審核中',
    ReportStatus.appealed => '申訴中',
    ReportStatus.resolvedHidden => '已處理（已隱藏）',
    ReportStatus.resolvedWarned => '已處理（已警告）',
    ReportStatus.resolvedBanned => '已處理（已停權）',
    ReportStatus.dismissed => '已駁回',
  };
}
