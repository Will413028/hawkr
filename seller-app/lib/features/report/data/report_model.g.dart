// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'report_model.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_ReportRecord _$ReportRecordFromJson(Map<String, dynamic> json) =>
    _ReportRecord(
      id: json['id'] as String? ?? '',
      targetType: json['targetType'] as String? ?? '',
      targetId: json['targetId'] as String? ?? '',
      reasonCode: json['reasonCode'] as String? ?? '',
      description: json['description'] as String? ?? '',
      status: json['status'] as String? ?? '',
      resolvedAction: json['resolvedAction'] as String?,
      resolvedBy: json['resolvedBy'] as String?,
      resolvedAtMs: _parseMsNullable(json['resolvedAtMs']),
      createTimeMs: json['createTimeMs'] == null
          ? 0
          : _parseMs(json['createTimeMs']),
      recentActionCount: json['recentActionCount'] as String? ?? '0',
    );

Map<String, dynamic> _$ReportRecordToJson(_ReportRecord instance) =>
    <String, dynamic>{
      'id': instance.id,
      'targetType': instance.targetType,
      'targetId': instance.targetId,
      'reasonCode': instance.reasonCode,
      'description': instance.description,
      'status': instance.status,
      'resolvedAction': instance.resolvedAction,
      'resolvedBy': instance.resolvedBy,
      'resolvedAtMs': instance.resolvedAtMs,
      'createTimeMs': instance.createTimeMs,
      'recentActionCount': instance.recentActionCount,
    };
