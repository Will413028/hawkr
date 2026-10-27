import 'package:hawkr_seller/core/codemeta/code_meta.dart';
import 'package:freezed_annotation/freezed_annotation.dart';

/// 檢舉相關碼表的值域一律取自生成的 CodeMeta（`core/codemeta/code_meta.dart`），
/// 本檔只放「哪些選項適用於哪種對象」與「畫面上要寫什麼字」這兩類呈現層規則。
///
/// 重新 export 是為了讓既有呼叫端不必同時 import 兩個檔——同時 import
/// `code_meta.dart` 與本檔時拿到的是同一個宣告，不會撞名。
export 'package:hawkr_seller/core/codemeta/code_meta.dart'
    show ReportReasonCode, ReportStatus, ReportTargetType;

part 'report_model.freezed.dart';
part 'report_model.g.dart';

int? _parseMsNullable(dynamic v) {
  if (v == null) return null;
  if (v is int) return v;
  if (v is num) return v.toInt();
  return int.tryParse(v.toString());
}

int _parseMs(dynamic v) => _parseMsNullable(v) ?? 0;

/// 檢舉紀錄（GET /api/v1/app/reports/me 單筆項目，backend-payuni#645）。
///
/// 除 [resolvedAtMs] / [createTimeMs] 外，後端欄位皆為字串（含 id，比照
/// TSID 64-bit BIGINT 以 string 傳輸避免精度問題）。
@freezed
abstract class ReportRecord with _$ReportRecord {
  const factory ReportRecord({
    @Default('') String id,
    @Default('') String targetType,
    @Default('') String targetId,
    @Default('') String reasonCode,
    @Default('') String description,
    @Default('') String status,
    String? resolvedAction,
    String? resolvedBy,
    @JsonKey(fromJson: _parseMsNullable) int? resolvedAtMs,
    @JsonKey(fromJson: _parseMs) @Default(0) int createTimeMs,
    @Default('0') String recentActionCount,
  }) = _ReportRecord;

  factory ReportRecord.fromJson(Map<String, dynamic> json) =>
      _$ReportRecordFromJson(json);
}

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

/// `reason_code` → 中文標籤；未知值原樣顯示（後端新增值時不崩潰）。
String reportReasonLabel(String code) =>
    ReportReasonCode.fromCode(code)?.label ?? code;

/// `target_type` → 中文標籤；未知值原樣顯示（後端新增值時不崩潰）。
///
/// 讀取端（`/reports/me` 回傳的既有紀錄）拿到的是後端字串，所以這裡收 String
/// 而不是 [ReportTargetType]。
///
/// 這個值不是顯示用的分類，它是 social-service 的路由鍵——admin 裁決時的
/// HIDE_TARGET 與 owner 解析都靠它決定要打哪張表
/// （`services/social-service/internal/usecase/dispatcher.go`），送錯值不會被
/// API 擋下，而是安靜地打到**別張表**：`POST_COMMENT` → `cms_comment`
/// （貼文底下的留言）、`SELLER_FORUM` → `cms_seller_forum`（賣家頁「迴響」
/// 留言板）。因此開啟檢舉時必須依「這則內容是從哪個端點讀來的」決定值，
/// 不是依 UI 上的字面叫不叫「留言」。
String reportTargetTypeLabel(String type) =>
    ReportTargetType.fromValue(type)?.label ?? type;

/// `status` → 檢舉人視角的中文標籤。
///
/// 值域來自碼表：switch 不寫 default，碼表日後多一個狀態時這裡編譯不過，
/// 不會像手抄對照表那樣安靜地漏掉。文案則是本畫面自己的——碼表 label 是
/// 治理後台視角（待處理／已隱藏／已警告），檢舉人要看的是「我這張單走到哪」，
/// 用字與買家 App 的我的檢舉一致。
///
/// 未知值原樣顯示：顯示原碼至少看得出有東西，比整格空白好。
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
