// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'shipment_anomaly_model.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_ShipmentAnomaly _$ShipmentAnomalyFromJson(Map<String, dynamic> json) =>
    _ShipmentAnomaly(
      shipmentId: json['shipmentId'] as String? ?? '',
      orderNo: json['orderNo'] as String? ?? '',
      anomalyType: json['anomalyType'] as String? ?? '',
      anomalyTypeLabel: json['anomalyTypeLabel'] as String? ?? '',
      severity: json['severity'] as String? ?? '',
      severityLabel: json['severityLabel'] as String? ?? '',
      shippingMethod: json['shippingMethod'] as String? ?? '',
      shipmentStatus: json['shipmentStatus'] as String? ?? '',
      detectedAt: json['detectedAt'] as String? ?? '',
      ackAction: json['ackAction'] as String? ?? '',
      actedAt: json['actedAt'] as String? ?? '',
    );

Map<String, dynamic> _$ShipmentAnomalyToJson(_ShipmentAnomaly instance) =>
    <String, dynamic>{
      'shipmentId': instance.shipmentId,
      'orderNo': instance.orderNo,
      'anomalyType': instance.anomalyType,
      'anomalyTypeLabel': instance.anomalyTypeLabel,
      'severity': instance.severity,
      'severityLabel': instance.severityLabel,
      'shippingMethod': instance.shippingMethod,
      'shipmentStatus': instance.shipmentStatus,
      'detectedAt': instance.detectedAt,
      'ackAction': instance.ackAction,
      'actedAt': instance.actedAt,
    };

_ShipmentAnomalyListResult _$ShipmentAnomalyListResultFromJson(
  Map<String, dynamic> json,
) => _ShipmentAnomalyListResult(
  records:
      (json['records'] as List<dynamic>?)
          ?.map((e) => ShipmentAnomaly.fromJson(e as Map<String, dynamic>))
          .toList() ??
      const [],
  total: json['total'] == null ? 0 : _parseInt(json['total']),
);

Map<String, dynamic> _$ShipmentAnomalyListResultToJson(
  _ShipmentAnomalyListResult instance,
) => <String, dynamic>{'records': instance.records, 'total': instance.total};

_ShipmentAnomalyStats _$ShipmentAnomalyStatsFromJson(
  Map<String, dynamic> json,
) => _ShipmentAnomalyStats(
  total: json['total'] == null ? 0 : _parseInt(json['total']),
  highCount: json['highSeverityCount'] == null
      ? 0
      : _parseInt(json['highSeverityCount']),
  mediumCount: json['mediumSeverityCount'] == null
      ? 0
      : _parseInt(json['mediumSeverityCount']),
  lowCount: json['lowSeverityCount'] == null
      ? 0
      : _parseInt(json['lowSeverityCount']),
);

Map<String, dynamic> _$ShipmentAnomalyStatsToJson(
  _ShipmentAnomalyStats instance,
) => <String, dynamic>{
  'total': instance.total,
  'highSeverityCount': instance.highCount,
  'mediumSeverityCount': instance.mediumCount,
  'lowSeverityCount': instance.lowCount,
};
