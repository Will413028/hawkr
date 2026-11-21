// This is a generated file - do not edit.
//
// Generated from hawkr/imageai/v1alpha1/imageai.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

import '../../common/v1alpha1/image.pb.dart' as $0;
import 'imageai.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'imageai.pbenum.dart';

/// AiStyle 是前台看到的修圖風格。
class AiStyle extends $pb.GeneratedMessage {
  factory AiStyle({
    $core.String? id,
    $core.String? name,
    $core.String? label,
    $core.String? prompt,
    $0.Image? thumbnail,
    $core.int? displayOrder,
  }) {
    final result = AiStyle._();
    if (id != null) result.id = id;
    if (name != null) result.name = name;
    if (label != null) result.label = label;
    if (prompt != null) result.prompt = prompt;
    if (thumbnail != null) result.thumbnail = thumbnail;
    if (displayOrder != null) result.displayOrder = displayOrder;
    return result;
  }

  AiStyle._();

  factory AiStyle.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AiStyle()..mergeFromBuffer(data, registry);
  factory AiStyle.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AiStyle()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AiStyle',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.imageai.v1alpha1'),
      createEmptyInstance: AiStyle.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'name')
    ..aOS(3, _omitFieldNames ? '' : 'label')
    ..aOS(4, _omitFieldNames ? '' : 'prompt')
    ..aOM<$0.Image>(5, _omitFieldNames ? '' : 'thumbnail',
        subBuilder: $0.Image.$_createMessage)
    ..aI(6, _omitFieldNames ? '' : 'displayOrder')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AiStyle clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AiStyle copyWith(void Function(AiStyle) updates) =>
      super.copyWith((message) => updates(message as AiStyle)) as AiStyle;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use AiStyle() / AiStyle.new instead')
  static AiStyle create() => AiStyle._();
  static $pb.GeneratedMessage $_createMessage() => AiStyle._();
  @$core.override
  AiStyle createEmptyInstance() => AiStyle._();
  @$core.pragma('dart2js:noInline')
  static AiStyle getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AiStyle>(AiStyle.$_createMessage);
  static AiStyle? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  /// 風格的識別名稱。
  @$pb.TagNumber(2)
  $core.String get name => $_getSZ(1);
  @$pb.TagNumber(2)
  set name($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasName() => $_has(1);
  @$pb.TagNumber(2)
  void clearName() => $_clearField(2);

  /// 顯示用的名稱。
  @$pb.TagNumber(3)
  $core.String get label => $_getSZ(2);
  @$pb.TagNumber(3)
  set label($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasLabel() => $_has(2);
  @$pb.TagNumber(3)
  void clearLabel() => $_clearField(3);

  /// 套用這個風格時送給模型的提示詞。
  @$pb.TagNumber(4)
  $core.String get prompt => $_getSZ(3);
  @$pb.TagNumber(4)
  set prompt($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasPrompt() => $_has(3);
  @$pb.TagNumber(4)
  void clearPrompt() => $_clearField(4);

  @$pb.TagNumber(5)
  $0.Image get thumbnail => $_getN(4);
  @$pb.TagNumber(5)
  set thumbnail($0.Image value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasThumbnail() => $_has(4);
  @$pb.TagNumber(5)
  void clearThumbnail() => $_clearField(5);
  @$pb.TagNumber(5)
  $0.Image ensureThumbnail() => $_ensure(4);

  /// 顯示順序，小的在前。
  @$pb.TagNumber(6)
  $core.int get displayOrder => $_getIZ(5);
  @$pb.TagNumber(6)
  set displayOrder($core.int value) => $_setSignedInt32(5, value);
  @$pb.TagNumber(6)
  $core.bool hasDisplayOrder() => $_has(5);
  @$pb.TagNumber(6)
  void clearDisplayOrder() => $_clearField(6);
}

class TransformImageRequest extends $pb.GeneratedMessage {
  factory TransformImageRequest({
    $core.List<$core.int>? imageData,
    $core.String? mimeType,
    $core.String? prompt,
  }) {
    final result = TransformImageRequest._();
    if (imageData != null) result.imageData = imageData;
    if (mimeType != null) result.mimeType = mimeType;
    if (prompt != null) result.prompt = prompt;
    return result;
  }

  TransformImageRequest._();

  factory TransformImageRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TransformImageRequest()..mergeFromBuffer(data, registry);
  factory TransformImageRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TransformImageRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TransformImageRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.imageai.v1alpha1'),
      createEmptyInstance: TransformImageRequest.$_createMessage)
    ..a<$core.List<$core.int>>(
        1, _omitFieldNames ? '' : 'imageData', $pb.PbFieldType.OY)
    ..aOS(2, _omitFieldNames ? '' : 'mimeType')
    ..aOS(3, _omitFieldNames ? '' : 'prompt')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TransformImageRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TransformImageRequest copyWith(
          void Function(TransformImageRequest) updates) =>
      super.copyWith((message) => updates(message as TransformImageRequest))
          as TransformImageRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use TransformImageRequest() / TransformImageRequest.new instead')
  static TransformImageRequest create() => TransformImageRequest._();
  static $pb.GeneratedMessage $_createMessage() => TransformImageRequest._();
  @$core.override
  TransformImageRequest createEmptyInstance() => TransformImageRequest._();
  @$core.pragma('dart2js:noInline')
  static TransformImageRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<TransformImageRequest>(
          TransformImageRequest.$_createMessage);
  static TransformImageRequest? _defaultInstance;

  /// 原圖的內容。
  @$pb.TagNumber(1)
  $core.List<$core.int> get imageData => $_getN(0);
  @$pb.TagNumber(1)
  set imageData($core.List<$core.int> value) => $_setBytes(0, value);
  @$pb.TagNumber(1)
  $core.bool hasImageData() => $_has(0);
  @$pb.TagNumber(1)
  void clearImageData() => $_clearField(1);

  /// 原圖的格式，例如 image/jpeg、image/png。
  @$pb.TagNumber(2)
  $core.String get mimeType => $_getSZ(1);
  @$pb.TagNumber(2)
  set mimeType($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasMimeType() => $_has(1);
  @$pb.TagNumber(2)
  void clearMimeType() => $_clearField(2);

  /// 修圖提示詞，通常取自 AiStyle.prompt 或 ListAiStylesResponse.default_prompt。
  @$pb.TagNumber(3)
  $core.String get prompt => $_getSZ(2);
  @$pb.TagNumber(3)
  set prompt($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPrompt() => $_has(2);
  @$pb.TagNumber(3)
  void clearPrompt() => $_clearField(3);
}

class TransformImageResponse extends $pb.GeneratedMessage {
  factory TransformImageResponse({
    $core.List<$core.int>? imageData,
    $core.String? mimeType,
    $core.int? weeklyUsage,
    $core.int? weeklyLimit,
  }) {
    final result = TransformImageResponse._();
    if (imageData != null) result.imageData = imageData;
    if (mimeType != null) result.mimeType = mimeType;
    if (weeklyUsage != null) result.weeklyUsage = weeklyUsage;
    if (weeklyLimit != null) result.weeklyLimit = weeklyLimit;
    return result;
  }

  TransformImageResponse._();

  factory TransformImageResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TransformImageResponse()..mergeFromBuffer(data, registry);
  factory TransformImageResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TransformImageResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TransformImageResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.imageai.v1alpha1'),
      createEmptyInstance: TransformImageResponse.$_createMessage)
    ..a<$core.List<$core.int>>(
        1, _omitFieldNames ? '' : 'imageData', $pb.PbFieldType.OY)
    ..aOS(2, _omitFieldNames ? '' : 'mimeType')
    ..aI(3, _omitFieldNames ? '' : 'weeklyUsage')
    ..aI(4, _omitFieldNames ? '' : 'weeklyLimit')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TransformImageResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TransformImageResponse copyWith(
          void Function(TransformImageResponse) updates) =>
      super.copyWith((message) => updates(message as TransformImageResponse))
          as TransformImageResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use TransformImageResponse() / TransformImageResponse.new instead')
  static TransformImageResponse create() => TransformImageResponse._();
  static $pb.GeneratedMessage $_createMessage() => TransformImageResponse._();
  @$core.override
  TransformImageResponse createEmptyInstance() => TransformImageResponse._();
  @$core.pragma('dart2js:noInline')
  static TransformImageResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<TransformImageResponse>(
          TransformImageResponse.$_createMessage);
  static TransformImageResponse? _defaultInstance;

  /// 修好的圖片內容。
  @$pb.TagNumber(1)
  $core.List<$core.int> get imageData => $_getN(0);
  @$pb.TagNumber(1)
  set imageData($core.List<$core.int> value) => $_setBytes(0, value);
  @$pb.TagNumber(1)
  $core.bool hasImageData() => $_has(0);
  @$pb.TagNumber(1)
  void clearImageData() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get mimeType => $_getSZ(1);
  @$pb.TagNumber(2)
  set mimeType($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasMimeType() => $_has(1);
  @$pb.TagNumber(2)
  void clearMimeType() => $_clearField(2);

  /// 這次修圖之後，本週已使用的次數與上限。
  @$pb.TagNumber(3)
  $core.int get weeklyUsage => $_getIZ(2);
  @$pb.TagNumber(3)
  set weeklyUsage($core.int value) => $_setSignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasWeeklyUsage() => $_has(2);
  @$pb.TagNumber(3)
  void clearWeeklyUsage() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.int get weeklyLimit => $_getIZ(3);
  @$pb.TagNumber(4)
  set weeklyLimit($core.int value) => $_setSignedInt32(3, value);
  @$pb.TagNumber(4)
  $core.bool hasWeeklyLimit() => $_has(3);
  @$pb.TagNumber(4)
  void clearWeeklyLimit() => $_clearField(4);
}

class ListAiStylesRequest extends $pb.GeneratedMessage {
  factory ListAiStylesRequest({
    AiStyleCategory? category,
  }) {
    final result = ListAiStylesRequest._();
    if (category != null) result.category = category;
    return result;
  }

  ListAiStylesRequest._();

  factory ListAiStylesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListAiStylesRequest()..mergeFromBuffer(data, registry);
  factory ListAiStylesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListAiStylesRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListAiStylesRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.imageai.v1alpha1'),
      createEmptyInstance: ListAiStylesRequest.$_createMessage)
    ..aE<AiStyleCategory>(1, _omitFieldNames ? '' : 'category',
        enumValues: AiStyleCategory.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListAiStylesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListAiStylesRequest copyWith(void Function(ListAiStylesRequest) updates) =>
      super.copyWith((message) => updates(message as ListAiStylesRequest))
          as ListAiStylesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use ListAiStylesRequest() / ListAiStylesRequest.new instead')
  static ListAiStylesRequest create() => ListAiStylesRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListAiStylesRequest._();
  @$core.override
  ListAiStylesRequest createEmptyInstance() => ListAiStylesRequest._();
  @$core.pragma('dart2js:noInline')
  static ListAiStylesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListAiStylesRequest>(
          ListAiStylesRequest.$_createMessage);
  static ListAiStylesRequest? _defaultInstance;

  /// 沒有設定時是 AI_STYLE_CATEGORY_PRODUCT；不接受 UNSPECIFIED。
  @$pb.TagNumber(1)
  AiStyleCategory get category => $_getN(0);
  @$pb.TagNumber(1)
  set category(AiStyleCategory value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasCategory() => $_has(0);
  @$pb.TagNumber(1)
  void clearCategory() => $_clearField(1);
}

class ListAiStylesResponse extends $pb.GeneratedMessage {
  factory ListAiStylesResponse({
    $core.String? defaultPrompt,
    $core.int? weeklyUsage,
    $core.int? weeklyLimit,
    $core.Iterable<AiStyle>? styles,
  }) {
    final result = ListAiStylesResponse._();
    if (defaultPrompt != null) result.defaultPrompt = defaultPrompt;
    if (weeklyUsage != null) result.weeklyUsage = weeklyUsage;
    if (weeklyLimit != null) result.weeklyLimit = weeklyLimit;
    if (styles != null) result.styles.addAll(styles);
    return result;
  }

  ListAiStylesResponse._();

  factory ListAiStylesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListAiStylesResponse()..mergeFromBuffer(data, registry);
  factory ListAiStylesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListAiStylesResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListAiStylesResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.imageai.v1alpha1'),
      createEmptyInstance: ListAiStylesResponse.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'defaultPrompt')
    ..aI(2, _omitFieldNames ? '' : 'weeklyUsage')
    ..aI(3, _omitFieldNames ? '' : 'weeklyLimit')
    ..pPM<AiStyle>(4, _omitFieldNames ? '' : 'styles',
        subBuilder: AiStyle.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListAiStylesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListAiStylesResponse copyWith(void Function(ListAiStylesResponse) updates) =>
      super.copyWith((message) => updates(message as ListAiStylesResponse))
          as ListAiStylesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListAiStylesResponse() / ListAiStylesResponse.new instead')
  static ListAiStylesResponse create() => ListAiStylesResponse._();
  static $pb.GeneratedMessage $_createMessage() => ListAiStylesResponse._();
  @$core.override
  ListAiStylesResponse createEmptyInstance() => ListAiStylesResponse._();
  @$core.pragma('dart2js:noInline')
  static ListAiStylesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListAiStylesResponse>(
          ListAiStylesResponse.$_createMessage);
  static ListAiStylesResponse? _defaultInstance;

  /// 沒有選風格時使用的提示詞。
  @$pb.TagNumber(1)
  $core.String get defaultPrompt => $_getSZ(0);
  @$pb.TagNumber(1)
  set defaultPrompt($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasDefaultPrompt() => $_has(0);
  @$pb.TagNumber(1)
  void clearDefaultPrompt() => $_clearField(1);

  /// 本週已使用的次數與上限（呼叫者的店家）。
  @$pb.TagNumber(2)
  $core.int get weeklyUsage => $_getIZ(1);
  @$pb.TagNumber(2)
  set weeklyUsage($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasWeeklyUsage() => $_has(1);
  @$pb.TagNumber(2)
  void clearWeeklyUsage() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.int get weeklyLimit => $_getIZ(2);
  @$pb.TagNumber(3)
  set weeklyLimit($core.int value) => $_setSignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasWeeklyLimit() => $_has(2);
  @$pb.TagNumber(3)
  void clearWeeklyLimit() => $_clearField(3);

  /// 依 display_order 排列；不分頁：風格由營運維護，數量有限。
  @$pb.TagNumber(4)
  $pb.PbList<AiStyle> get styles => $_getList(3);
}

/// ImageAiAppService 是登入的賣家的修圖方法。店家取自呼叫者目前的店家，帳號沒有賣家角色時回 permission_denied。
class ImageAiAppServiceApi {
  final $pb.RpcClient _client;

  ImageAiAppServiceApi(this._client);

  /// TransformImage 以提示詞修一張圖，成功時扣一次本週額度。
  ///
  /// 錯誤：本週額度用完回 resource_exhausted，reason 是 IMAGE_QUOTA_EXCEEDED；invalid_argument 的 reason 是
  /// IMAGE_SAFETY_BLOCKED 與 IMAGE_RECITATION_BLOCKED（輸入要換：換圖或改提示詞，不要重試）；unavailable 的 reason 是
  /// IMAGE_NOT_PRODUCED（模型沒有產出圖片，重試可能成功），模型供應商無法呼叫也回 unavailable。
  /// 同時進行的修圖超過上限回 resource_exhausted，帶 RetryInfo。
  /// 呼叫失敗時額度退回。
  $async.Future<TransformImageResponse> transformImage(
          $pb.ClientContext? ctx, TransformImageRequest request) =>
      _client.invoke<TransformImageResponse>(ctx, 'ImageAiAppService',
          'TransformImage', request, TransformImageResponse());

  /// ListAiStyles 列出某個用途的啟用中風格，連同本週的額度使用量。
  $async.Future<ListAiStylesResponse> listAiStyles(
          $pb.ClientContext? ctx, ListAiStylesRequest request) =>
      _client.invoke<ListAiStylesResponse>(ctx, 'ImageAiAppService',
          'ListAiStyles', request, ListAiStylesResponse());
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
