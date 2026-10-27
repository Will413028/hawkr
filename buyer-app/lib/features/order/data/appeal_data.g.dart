// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'appeal_data.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_AppealEvent _$AppealEventFromJson(Map<String, dynamic> json) => _AppealEvent(
  eventType: json['eventType'] as String? ?? '',
  timestamp: json['timestamp'] as String? ?? '',
  label: json['label'] as String? ?? '',
  description: json['description'] as String? ?? '',
);

Map<String, dynamic> _$AppealEventToJson(_AppealEvent instance) =>
    <String, dynamic>{
      'eventType': instance.eventType,
      'timestamp': instance.timestamp,
      'label': instance.label,
      'description': instance.description,
    };

_PlatformDecision _$PlatformDecisionFromJson(Map<String, dynamic> json) =>
    _PlatformDecision(
      outcome: json['outcome'] as String? ?? '',
      refundAmount: json['refundAmount'] as String? ?? '',
      reason: json['reason'] as String? ?? '',
      decidedAt: json['decidedAt'] as String? ?? '',
    );

Map<String, dynamic> _$PlatformDecisionToJson(_PlatformDecision instance) =>
    <String, dynamic>{
      'outcome': instance.outcome,
      'refundAmount': instance.refundAmount,
      'reason': instance.reason,
      'decidedAt': instance.decidedAt,
    };

_ArbitrationData _$ArbitrationDataFromJson(Map<String, dynamic> json) =>
    _ArbitrationData(
      state: json['state'] as String? ?? '',
      timeline:
          (json['timeline'] as List<dynamic>?)
              ?.map((e) => AppealEvent.fromJson(e as Map<String, dynamic>))
              .toList() ??
          const [],
      decision: json['decision'] == null
          ? null
          : PlatformDecision.fromJson(json['decision'] as Map<String, dynamic>),
    );

Map<String, dynamic> _$ArbitrationDataToJson(_ArbitrationData instance) =>
    <String, dynamic>{
      'state': instance.state,
      'timeline': instance.timeline,
      'decision': instance.decision,
    };
