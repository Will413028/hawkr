// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'media_model.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_MediaUploadResult _$MediaUploadResultFromJson(Map<String, dynamic> json) =>
    _MediaUploadResult(
      stagedToken: json['stagedToken'] as String,
      size: (json['size'] as num).toInt(),
    );

Map<String, dynamic> _$MediaUploadResultToJson(_MediaUploadResult instance) =>
    <String, dynamic>{
      'stagedToken': instance.stagedToken,
      'size': instance.size,
    };
