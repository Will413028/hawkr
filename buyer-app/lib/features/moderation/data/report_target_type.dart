import 'package:hawkr_buyer/core/codemeta/code_meta.dart';

/// 檢舉相關碼表的值域一律取自生成的 CodeMeta（`core/codemeta/code_meta.dart`），
/// 本檔是聊天入口（舊 `/app/reports`，A6 前的允許殘留）用的對照；社群檢舉的原因與對象見 `report_target.dart`。
/// 本檔只放「哪些選項適用於哪種對象」與「畫面上要寫什麼字」這兩類呈現層規則。
///
/// 重新 export 是為了讓既有呼叫端不必同時 import 兩個檔——同時 import
/// `code_meta.dart` 與本檔時拿到的是同一個宣告，不會撞名。
export 'package:hawkr_buyer/core/codemeta/code_meta.dart'
    show ReportReasonCode, ReportTargetType;

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
