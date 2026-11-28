import 'package:freezed_annotation/freezed_annotation.dart';

part 'shipment_anomaly_model.freezed.dart';
part 'shipment_anomaly_model.g.dart';

/// 後端 total / page 等回傳為字串（e.g. "5"），統一轉 int。
int _parseInt(dynamic value) =>
    value is int ? value : int.tryParse(value?.toString() ?? '') ?? 0;

/// ⚠️ 這裡**不再**有手抄的 `ShipmentAnomalyType`。
///
/// 它曾經停在 7 個值而 CodeMeta 有 11 個（4 個新值缺席），`labelOf` 對認不得的
/// 碼值 `return value` ⇒ 賣家畫面直接出現 raw code。
/// 而那份漂移沒有任何 gate 擋得住：CodeMeta 的檢查只看生成物，不看手寫 class。
///
/// 碼值與 label 一律走 `core/codemeta/code_meta.dart` 的生成 enum，
/// 顯示規則走 `core/codemeta/shipment_anomaly_presentation.dart`。

/// Severity 對應 backend `ShipmentAnomalySeverity`。
class ShipmentAnomalySeverity {
  ShipmentAnomalySeverity._();

  static const String high = 'HIGH';
  static const String medium = 'MEDIUM';
  static const String low = 'LOW';

  /// ⚠️ label 一律走 `anomalySeverityLabelOf`（CodeMeta 生成的 label）。
  /// 這裡只保留**值常數**——UI 的篩選 chip 以字串比對，換成 enum 會牽動整個畫面。
}

/// Ack action 對應 backend `ShipmentAnomalyAckAction`。
class ShipmentAnomalyAckAction {
  ShipmentAnomalyAckAction._();

  static const String acknowledged = 'ACKNOWLEDGED';
  static const String ignored = 'IGNORED';
}

@freezed
abstract class ShipmentAnomaly with _$ShipmentAnomaly {
  const factory ShipmentAnomaly({
    @Default('') String shipmentId,
    @Default('') String orderNo,
    @Default('') String anomalyType,
    @Default('') String anomalyTypeLabel,
    @Default('') String severity,
    @Default('') String severityLabel,
    @Default('') String shippingMethod,
    @Default('') String shipmentStatus,
    @Default('') String detectedAt,
    @Default('') String ackAction,
    // Gateway 欄位名稱為 actedAt。
    @Default('') String actedAt,
  }) = _ShipmentAnomaly;

  factory ShipmentAnomaly.fromJson(Map<String, dynamic> json) =>
      _$ShipmentAnomalyFromJson(json);
}

@freezed
abstract class ShipmentAnomalyListResult with _$ShipmentAnomalyListResult {
  const factory ShipmentAnomalyListResult({
    @Default([]) List<ShipmentAnomaly> records,
    @Default(0) @JsonKey(fromJson: _parseInt) int total,
  }) = _ShipmentAnomalyListResult;

  factory ShipmentAnomalyListResult.fromJson(Map<String, dynamic> json) =>
      _$ShipmentAnomalyListResultFromJson(json);
}

@freezed
abstract class ShipmentAnomalyStats with _$ShipmentAnomalyStats {
  const factory ShipmentAnomalyStats({
    @Default(0) @JsonKey(fromJson: _parseInt) int total,
    // Gateway 回傳 highSeverityCount、mediumSeverityCount 與 lowSeverityCount。
    @Default(0)
    @JsonKey(name: 'highSeverityCount', fromJson: _parseInt)
    int highCount,
    @Default(0)
    @JsonKey(name: 'mediumSeverityCount', fromJson: _parseInt)
    int mediumCount,
    @Default(0)
    @JsonKey(name: 'lowSeverityCount', fromJson: _parseInt)
    int lowCount,
  }) = _ShipmentAnomalyStats;

  factory ShipmentAnomalyStats.fromJson(Map<String, dynamic> json) =>
      _$ShipmentAnomalyStatsFromJson(json);
}
