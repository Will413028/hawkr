// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'ai_style_model.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_AiStyle _$AiStyleFromJson(Map<String, dynamic> json) => _AiStyle(
  id: json['id'] as String,
  name: json['name'] as String,
  label: json['label'] as String,
  prompt: json['prompt'] as String,
  thumbnailKey: json['thumbnailKey'] as String? ?? '',
  sortOrder: json['sortOrder'] as String? ?? '0',
);

Map<String, dynamic> _$AiStyleToJson(_AiStyle instance) => <String, dynamic>{
  'id': instance.id,
  'name': instance.name,
  'label': instance.label,
  'prompt': instance.prompt,
  'thumbnailKey': instance.thumbnailKey,
  'sortOrder': instance.sortOrder,
};

_AiStylesResponse _$AiStylesResponseFromJson(Map<String, dynamic> json) =>
    _AiStylesResponse(
      styles:
          (json['styles'] as List<dynamic>?)
              ?.map((e) => AiStyle.fromJson(e as Map<String, dynamic>))
              .toList() ??
          const [],
      defaultPrompt: json['defaultPrompt'] as String? ?? '',
      weeklyUsage: json['weeklyUsage'] as String? ?? '0',
      weeklyLimit: json['weeklyLimit'] as String? ?? '10',
    );

Map<String, dynamic> _$AiStylesResponseToJson(_AiStylesResponse instance) =>
    <String, dynamic>{
      'styles': instance.styles,
      'defaultPrompt': instance.defaultPrompt,
      'weeklyUsage': instance.weeklyUsage,
      'weeklyLimit': instance.weeklyLimit,
    };
