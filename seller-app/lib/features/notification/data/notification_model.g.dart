// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'notification_model.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_NotificationItem _$NotificationItemFromJson(Map<String, dynamic> json) =>
    _NotificationItem(
      id: json['id'] == null ? 0 : _parseInt(json['id']),
      type: json['type'] as String? ?? '',
      title: json['title'] as String? ?? '',
      body: json['body'] as String? ?? '',
      refType: json['refType'] as String?,
      refId: json['refId'] as String?,
      isRead: json['isRead'] as bool? ?? false,
      readAt: json['readAt'] == null
          ? null
          : DateTime.parse(json['readAt'] as String),
      createdAt: DateTime.parse(json['createdAt'] as String),
      data: json['data'] as Map<String, dynamic>?,
    );

Map<String, dynamic> _$NotificationItemToJson(_NotificationItem instance) =>
    <String, dynamic>{
      'id': instance.id,
      'type': instance.type,
      'title': instance.title,
      'body': instance.body,
      'refType': instance.refType,
      'refId': instance.refId,
      'isRead': instance.isRead,
      'readAt': instance.readAt?.toIso8601String(),
      'createdAt': instance.createdAt.toIso8601String(),
      'data': instance.data,
    };

_NotificationListResult _$NotificationListResultFromJson(
  Map<String, dynamic> json,
) => _NotificationListResult(
  records:
      (json['records'] as List<dynamic>?)
          ?.map((e) => NotificationItem.fromJson(e as Map<String, dynamic>))
          .toList() ??
      const [],
  total: json['total'] == null ? 0 : _parseInt(json['total']),
  pageSize: json['pageSize'] == null ? 20 : _parseInt(json['pageSize']),
  hasMore: json['hasMore'] as bool? ?? false,
);

Map<String, dynamic> _$NotificationListResultToJson(
  _NotificationListResult instance,
) => <String, dynamic>{
  'records': instance.records,
  'total': instance.total,
  'pageSize': instance.pageSize,
  'hasMore': instance.hasMore,
};

_UnreadCount _$UnreadCountFromJson(Map<String, dynamic> json) =>
    _UnreadCount(count: json['count'] == null ? 0 : _parseInt(json['count']));

Map<String, dynamic> _$UnreadCountToJson(_UnreadCount instance) =>
    <String, dynamic>{'count': instance.count};

_NotificationPreferenceItem _$NotificationPreferenceItemFromJson(
  Map<String, dynamic> json,
) => _NotificationPreferenceItem(
  category: json['category'] as String,
  enabled: json['enabled'] as bool? ?? true,
  title: json['title'] as String? ?? '',
  description: json['description'] as String? ?? '',
  forced: json['forced'] as bool? ?? false,
);

Map<String, dynamic> _$NotificationPreferenceItemToJson(
  _NotificationPreferenceItem instance,
) => <String, dynamic>{
  'category': instance.category,
  'enabled': instance.enabled,
  'title': instance.title,
  'description': instance.description,
  'forced': instance.forced,
};
