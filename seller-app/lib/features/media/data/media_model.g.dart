// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'media_model.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_UploadSession _$UploadSessionFromJson(Map<String, dynamic> json) =>
    _UploadSession(
      sessionId: json['sessionId'] as String,
      uploadUrl: json['uploadUrl'] as String? ?? '',
      objectKey: json['objectKey'] as String? ?? '',
    );

Map<String, dynamic> _$UploadSessionToJson(_UploadSession instance) =>
    <String, dynamic>{
      'sessionId': instance.sessionId,
      'uploadUrl': instance.uploadUrl,
      'objectKey': instance.objectKey,
    };

_MediaUploadResult _$MediaUploadResultFromJson(Map<String, dynamic> json) =>
    _MediaUploadResult(
      url: json['url'] as String? ?? '',
      fileUrl: json['fileUrl'] as String? ?? '',
      objectKey: json['objectKey'] as String,
      size: (json['size'] as num).toInt(),
      originalSize: (json['originalSize'] as num?)?.toInt(),
      filename: json['filename'] as String?,
      stagedToken: json['stagedToken'] as String?,
      previewUrl: json['previewUrl'] as String?,
    );

Map<String, dynamic> _$MediaUploadResultToJson(_MediaUploadResult instance) =>
    <String, dynamic>{
      'url': instance.url,
      'fileUrl': instance.fileUrl,
      'objectKey': instance.objectKey,
      'size': instance.size,
      'originalSize': instance.originalSize,
      'filename': instance.filename,
      'stagedToken': instance.stagedToken,
      'previewUrl': instance.previewUrl,
    };

_PrivateUploadResult _$PrivateUploadResultFromJson(Map<String, dynamic> json) =>
    _PrivateUploadResult(
      referenceId: json['referenceId'] as String,
      objectKey: json['objectKey'] as String,
      originalSize: (json['originalSize'] as num).toInt(),
      docType: json['docType'] as String,
      stagedToken: json['stagedToken'] as String?,
    );

Map<String, dynamic> _$PrivateUploadResultToJson(
  _PrivateUploadResult instance,
) => <String, dynamic>{
  'referenceId': instance.referenceId,
  'objectKey': instance.objectKey,
  'originalSize': instance.originalSize,
  'docType': instance.docType,
  'stagedToken': instance.stagedToken,
};

_FinalizedAsset _$FinalizedAssetFromJson(Map<String, dynamic> json) =>
    _FinalizedAsset(
      stagedToken: json['stagedToken'] as String,
      assetId: json['assetId'] as String,
      objectKey: json['objectKey'] as String,
    );

Map<String, dynamic> _$FinalizedAssetToJson(_FinalizedAsset instance) =>
    <String, dynamic>{
      'stagedToken': instance.stagedToken,
      'assetId': instance.assetId,
      'objectKey': instance.objectKey,
    };
