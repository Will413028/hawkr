/// GET /app/reports/me 回傳單筆檢舉紀錄。
/// 除 resolved_at_ms / create_time_ms 為 number 外，其餘欄位皆為 string
/// （TSID 或大數字，避免 JS number 精度問題），這裡一律容錯轉型。
class ReportRecord {
  const ReportRecord({
    required this.id,
    required this.targetType,
    required this.targetId,
    required this.reasonCode,
    this.description,
    required this.status,
    this.resolvedAction,
    this.resolvedBy,
    this.resolvedAtMs,
    required this.createTimeMs,
    this.recentActionCount = 0,
  });

  final String id;
  final String targetType;
  final String targetId;
  final String reasonCode;
  final String? description;
  final String status;
  final String? resolvedAction;
  final String? resolvedBy;
  final int? resolvedAtMs;
  final int createTimeMs;
  final int recentActionCount;

  static int? _parseIntOrNull(dynamic v) {
    if (v == null) return null;
    if (v is int) return v;
    if (v is num) return v.toInt();
    return int.tryParse(v.toString());
  }

  factory ReportRecord.fromJson(Map<String, dynamic> json) {
    return ReportRecord(
      id: (json['id'] ?? '').toString(),
      targetType: (json['targetType'] ?? '').toString(),
      targetId: (json['targetId'] ?? '').toString(),
      reasonCode: (json['reasonCode'] ?? '').toString(),
      description: json['description']?.toString(),
      status: (json['status'] ?? '').toString(),
      resolvedAction: json['resolvedAction']?.toString(),
      resolvedBy: json['resolvedBy']?.toString(),
      resolvedAtMs: _parseIntOrNull(json['resolvedAtMs']),
      createTimeMs: _parseIntOrNull(json['createTimeMs']) ?? 0,
      recentActionCount: _parseIntOrNull(json['recentActionCount']) ?? 0,
    );
  }
}
