// This is a generated file - do not edit.
//
// Generated from hawkr/media/v1alpha1/media.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:fixnum/fixnum.dart' as $fixnum;
import 'package:protobuf/protobuf.dart' as $pb;
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart'
    as $0;

import 'media.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'media.pbenum.dart';

class BeginUploadRequest extends $pb.GeneratedMessage {
  factory BeginUploadRequest({
    $fixnum.Int64? size,
    $core.String? contentType,
    $core.String? md5,
  }) {
    final result = BeginUploadRequest._();
    if (size != null) result.size = size;
    if (contentType != null) result.contentType = contentType;
    if (md5 != null) result.md5 = md5;
    return result;
  }

  BeginUploadRequest._();

  factory BeginUploadRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BeginUploadRequest()..mergeFromBuffer(data, registry);
  factory BeginUploadRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BeginUploadRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BeginUploadRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.media.v1alpha1'),
      createEmptyInstance: BeginUploadRequest.$_createMessage)
    ..aInt64(1, _omitFieldNames ? '' : 'size')
    ..aOS(2, _omitFieldNames ? '' : 'contentType')
    ..aOS(3, _omitFieldNames ? '' : 'md5')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BeginUploadRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BeginUploadRequest copyWith(void Function(BeginUploadRequest) updates) =>
      super.copyWith((message) => updates(message as BeginUploadRequest))
          as BeginUploadRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use BeginUploadRequest() / BeginUploadRequest.new instead')
  static BeginUploadRequest create() => BeginUploadRequest._();
  static $pb.GeneratedMessage $_createMessage() => BeginUploadRequest._();
  @$core.override
  BeginUploadRequest createEmptyInstance() => BeginUploadRequest._();
  @$core.pragma('dart2js:noInline')
  static BeginUploadRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BeginUploadRequest>(
          BeginUploadRequest.$_createMessage);
  static BeginUploadRequest? _defaultInstance;

  /// 檔案的位元組數；必須大於零且不超過伺服器的上限，超過回 invalid_argument 指向 size，訊息是在地化文字。
  @$pb.TagNumber(1)
  $fixnum.Int64 get size => $_getI64(0);
  @$pb.TagNumber(1)
  set size($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSize() => $_has(0);
  @$pb.TagNumber(1)
  void clearSize() => $_clearField(1);

  /// 檔案的 MIME 型別，只能是 image/jpeg、image/png、image/webp、image/gif；伺服器依內容辨認，宣告的型別只用來簽網址。
  @$pb.TagNumber(2)
  $core.String get contentType => $_getSZ(1);
  @$pb.TagNumber(2)
  set contentType($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasContentType() => $_has(1);
  @$pb.TagNumber(2)
  void clearContentType() => $_clearField(2);

  /// 檔案內容 MD5 的標準 base64（16 位元組）；簽進網址，內容不符時物件儲存拒收。
  @$pb.TagNumber(3)
  $core.String get md5 => $_getSZ(2);
  @$pb.TagNumber(3)
  set md5($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasMd5() => $_has(2);
  @$pb.TagNumber(3)
  void clearMd5() => $_clearField(3);
}

class BeginUploadResponse extends $pb.GeneratedMessage {
  factory BeginUploadResponse({
    $core.String? stagedToken,
    $core.String? uploadUrl,
    $core.Iterable<$core.MapEntry<$core.String, $core.String>>? uploadHeaders,
    $0.Timestamp? expireTime,
  }) {
    final result = BeginUploadResponse._();
    if (stagedToken != null) result.stagedToken = stagedToken;
    if (uploadUrl != null) result.uploadUrl = uploadUrl;
    if (uploadHeaders != null) result.uploadHeaders.addEntries(uploadHeaders);
    if (expireTime != null) result.expireTime = expireTime;
    return result;
  }

  BeginUploadResponse._();

  factory BeginUploadResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BeginUploadResponse()..mergeFromBuffer(data, registry);
  factory BeginUploadResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BeginUploadResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BeginUploadResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.media.v1alpha1'),
      createEmptyInstance: BeginUploadResponse.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'stagedToken')
    ..aOS(2, _omitFieldNames ? '' : 'uploadUrl')
    ..m<$core.String, $core.String>(3, _omitFieldNames ? '' : 'uploadHeaders',
        entryClassName: 'BeginUploadResponse.UploadHeadersEntry',
        keyFieldType: $pb.PbFieldType.OS,
        valueFieldType: $pb.PbFieldType.OS,
        packageName: const $pb.PackageName('hawkr.media.v1alpha1'))
    ..aOM<$0.Timestamp>(4, _omitFieldNames ? '' : 'expireTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BeginUploadResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BeginUploadResponse copyWith(void Function(BeginUploadResponse) updates) =>
      super.copyWith((message) => updates(message as BeginUploadResponse))
          as BeginUploadResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use BeginUploadResponse() / BeginUploadResponse.new instead')
  static BeginUploadResponse create() => BeginUploadResponse._();
  static $pb.GeneratedMessage $_createMessage() => BeginUploadResponse._();
  @$core.override
  BeginUploadResponse createEmptyInstance() => BeginUploadResponse._();
  @$core.pragma('dart2js:noInline')
  static BeginUploadResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BeginUploadResponse>(
          BeginUploadResponse.$_createMessage);
  static BeginUploadResponse? _defaultInstance;

  /// 不透明的憑證，交給 CompleteUpload、GetUpload 與擁有者的 API；只出現這一次。
  @$pb.TagNumber(1)
  $core.String get stagedToken => $_getSZ(0);
  @$pb.TagNumber(1)
  set stagedToken($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStagedToken() => $_has(0);
  @$pb.TagNumber(1)
  void clearStagedToken() => $_clearField(1);

  /// 預簽的 PUT 網址。
  @$pb.TagNumber(2)
  $core.String get uploadUrl => $_getSZ(1);
  @$pb.TagNumber(2)
  set uploadUrl($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasUploadUrl() => $_has(1);
  @$pb.TagNumber(2)
  void clearUploadUrl() => $_clearField(2);

  /// PUT 時必須原樣帶上的請求標頭（已簽入網址的值）；少帶、改值或多帶簽章以外的內容被物件儲存拒收。不含 Content-Length（瀏覽器不允許腳本設定）：
  /// 請求本文必須恰好是 BeginUploadRequest.size 位元組，由傳輸層帶 Content-Length；長度不符被物件儲存拒收。
  @$pb.TagNumber(3)
  $pb.PbMap<$core.String, $core.String> get uploadHeaders => $_getMap(2);

  /// 暫存的到期時間；過了之後 staged token 不能再用。預簽網址的有效期短於它。
  @$pb.TagNumber(4)
  $0.Timestamp get expireTime => $_getN(3);
  @$pb.TagNumber(4)
  set expireTime($0.Timestamp value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasExpireTime() => $_has(3);
  @$pb.TagNumber(4)
  void clearExpireTime() => $_clearField(4);
  @$pb.TagNumber(4)
  $0.Timestamp ensureExpireTime() => $_ensure(3);
}

class CompleteUploadRequest extends $pb.GeneratedMessage {
  factory CompleteUploadRequest({
    $core.String? stagedToken,
  }) {
    final result = CompleteUploadRequest._();
    if (stagedToken != null) result.stagedToken = stagedToken;
    return result;
  }

  CompleteUploadRequest._();

  factory CompleteUploadRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CompleteUploadRequest()..mergeFromBuffer(data, registry);
  factory CompleteUploadRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CompleteUploadRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CompleteUploadRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.media.v1alpha1'),
      createEmptyInstance: CompleteUploadRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'stagedToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CompleteUploadRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CompleteUploadRequest copyWith(
          void Function(CompleteUploadRequest) updates) =>
      super.copyWith((message) => updates(message as CompleteUploadRequest))
          as CompleteUploadRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CompleteUploadRequest() / CompleteUploadRequest.new instead')
  static CompleteUploadRequest create() => CompleteUploadRequest._();
  static $pb.GeneratedMessage $_createMessage() => CompleteUploadRequest._();
  @$core.override
  CompleteUploadRequest createEmptyInstance() => CompleteUploadRequest._();
  @$core.pragma('dart2js:noInline')
  static CompleteUploadRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CompleteUploadRequest>(
          CompleteUploadRequest.$_createMessage);
  static CompleteUploadRequest? _defaultInstance;

  /// BeginUpload 回傳的 staged token。
  @$pb.TagNumber(1)
  $core.String get stagedToken => $_getSZ(0);
  @$pb.TagNumber(1)
  set stagedToken($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStagedToken() => $_has(0);
  @$pb.TagNumber(1)
  void clearStagedToken() => $_clearField(1);
}

class CompleteUploadResponse extends $pb.GeneratedMessage {
  factory CompleteUploadResponse() => CompleteUploadResponse._();

  CompleteUploadResponse._();

  factory CompleteUploadResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CompleteUploadResponse()..mergeFromBuffer(data, registry);
  factory CompleteUploadResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CompleteUploadResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CompleteUploadResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.media.v1alpha1'),
      createEmptyInstance: CompleteUploadResponse.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CompleteUploadResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CompleteUploadResponse copyWith(
          void Function(CompleteUploadResponse) updates) =>
      super.copyWith((message) => updates(message as CompleteUploadResponse))
          as CompleteUploadResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CompleteUploadResponse() / CompleteUploadResponse.new instead')
  static CompleteUploadResponse create() => CompleteUploadResponse._();
  static $pb.GeneratedMessage $_createMessage() => CompleteUploadResponse._();
  @$core.override
  CompleteUploadResponse createEmptyInstance() => CompleteUploadResponse._();
  @$core.pragma('dart2js:noInline')
  static CompleteUploadResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CompleteUploadResponse>(
          CompleteUploadResponse.$_createMessage);
  static CompleteUploadResponse? _defaultInstance;
}

class GetUploadRequest extends $pb.GeneratedMessage {
  factory GetUploadRequest({
    $core.String? stagedToken,
  }) {
    final result = GetUploadRequest._();
    if (stagedToken != null) result.stagedToken = stagedToken;
    return result;
  }

  GetUploadRequest._();

  factory GetUploadRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetUploadRequest()..mergeFromBuffer(data, registry);
  factory GetUploadRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetUploadRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetUploadRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.media.v1alpha1'),
      createEmptyInstance: GetUploadRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'stagedToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetUploadRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetUploadRequest copyWith(void Function(GetUploadRequest) updates) =>
      super.copyWith((message) => updates(message as GetUploadRequest))
          as GetUploadRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GetUploadRequest() / GetUploadRequest.new instead')
  static GetUploadRequest create() => GetUploadRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetUploadRequest._();
  @$core.override
  GetUploadRequest createEmptyInstance() => GetUploadRequest._();
  @$core.pragma('dart2js:noInline')
  static GetUploadRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<GetUploadRequest>(
          GetUploadRequest.$_createMessage);
  static GetUploadRequest? _defaultInstance;

  /// BeginUpload 回傳的 staged token。
  @$pb.TagNumber(1)
  $core.String get stagedToken => $_getSZ(0);
  @$pb.TagNumber(1)
  set stagedToken($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStagedToken() => $_has(0);
  @$pb.TagNumber(1)
  void clearStagedToken() => $_clearField(1);
}

class GetUploadResponse extends $pb.GeneratedMessage {
  factory GetUploadResponse({
    UploadState? state,
    RejectReason? rejectReason,
    $0.Timestamp? expireTime,
  }) {
    final result = GetUploadResponse._();
    if (state != null) result.state = state;
    if (rejectReason != null) result.rejectReason = rejectReason;
    if (expireTime != null) result.expireTime = expireTime;
    return result;
  }

  GetUploadResponse._();

  factory GetUploadResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetUploadResponse()..mergeFromBuffer(data, registry);
  factory GetUploadResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetUploadResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetUploadResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.media.v1alpha1'),
      createEmptyInstance: GetUploadResponse.$_createMessage)
    ..aE<UploadState>(1, _omitFieldNames ? '' : 'state',
        enumValues: UploadState.values)
    ..aE<RejectReason>(2, _omitFieldNames ? '' : 'rejectReason',
        enumValues: RejectReason.values)
    ..aOM<$0.Timestamp>(3, _omitFieldNames ? '' : 'expireTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetUploadResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetUploadResponse copyWith(void Function(GetUploadResponse) updates) =>
      super.copyWith((message) => updates(message as GetUploadResponse))
          as GetUploadResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GetUploadResponse() / GetUploadResponse.new instead')
  static GetUploadResponse create() => GetUploadResponse._();
  static $pb.GeneratedMessage $_createMessage() => GetUploadResponse._();
  @$core.override
  GetUploadResponse createEmptyInstance() => GetUploadResponse._();
  @$core.pragma('dart2js:noInline')
  static GetUploadResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<GetUploadResponse>(
          GetUploadResponse.$_createMessage);
  static GetUploadResponse? _defaultInstance;

  @$pb.TagNumber(1)
  UploadState get state => $_getN(0);
  @$pb.TagNumber(1)
  set state(UploadState value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasState() => $_has(0);
  @$pb.TagNumber(1)
  void clearState() => $_clearField(1);

  /// 只在 state 是 REJECTED 時設值。
  @$pb.TagNumber(2)
  RejectReason get rejectReason => $_getN(1);
  @$pb.TagNumber(2)
  set rejectReason(RejectReason value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasRejectReason() => $_has(1);
  @$pb.TagNumber(2)
  void clearRejectReason() => $_clearField(2);

  /// 暫存的到期時間。
  @$pb.TagNumber(3)
  $0.Timestamp get expireTime => $_getN(2);
  @$pb.TagNumber(3)
  set expireTime($0.Timestamp value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasExpireTime() => $_has(2);
  @$pb.TagNumber(3)
  void clearExpireTime() => $_clearField(3);
  @$pb.TagNumber(3)
  $0.Timestamp ensureExpireTime() => $_ensure(2);
}

class BeginSecureUploadRequest extends $pb.GeneratedMessage {
  factory BeginSecureUploadRequest({
    $fixnum.Int64? size,
  }) {
    final result = BeginSecureUploadRequest._();
    if (size != null) result.size = size;
    return result;
  }

  BeginSecureUploadRequest._();

  factory BeginSecureUploadRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BeginSecureUploadRequest()..mergeFromBuffer(data, registry);
  factory BeginSecureUploadRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BeginSecureUploadRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BeginSecureUploadRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.media.v1alpha1'),
      createEmptyInstance: BeginSecureUploadRequest.$_createMessage)
    ..aInt64(1, _omitFieldNames ? '' : 'size')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BeginSecureUploadRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BeginSecureUploadRequest copyWith(
          void Function(BeginSecureUploadRequest) updates) =>
      super.copyWith((message) => updates(message as BeginSecureUploadRequest))
          as BeginSecureUploadRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use BeginSecureUploadRequest() / BeginSecureUploadRequest.new instead')
  static BeginSecureUploadRequest create() => BeginSecureUploadRequest._();
  static $pb.GeneratedMessage $_createMessage() => BeginSecureUploadRequest._();
  @$core.override
  BeginSecureUploadRequest createEmptyInstance() =>
      BeginSecureUploadRequest._();
  @$core.pragma('dart2js:noInline')
  static BeginSecureUploadRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BeginSecureUploadRequest>(
          BeginSecureUploadRequest.$_createMessage);
  static BeginSecureUploadRequest? _defaultInstance;

  /// 檔案的明文位元組數；必須大於零且不超過伺服器的上限，段總數由它算出。
  @$pb.TagNumber(1)
  $fixnum.Int64 get size => $_getI64(0);
  @$pb.TagNumber(1)
  set size($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSize() => $_has(0);
  @$pb.TagNumber(1)
  void clearSize() => $_clearField(1);
}

class BeginSecureUploadResponse extends $pb.GeneratedMessage {
  factory BeginSecureUploadResponse({
    $core.String? stagedToken,
    $core.int? chunkSize,
    $core.int? chunkCount,
    $0.Timestamp? expireTime,
  }) {
    final result = BeginSecureUploadResponse._();
    if (stagedToken != null) result.stagedToken = stagedToken;
    if (chunkSize != null) result.chunkSize = chunkSize;
    if (chunkCount != null) result.chunkCount = chunkCount;
    if (expireTime != null) result.expireTime = expireTime;
    return result;
  }

  BeginSecureUploadResponse._();

  factory BeginSecureUploadResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BeginSecureUploadResponse()..mergeFromBuffer(data, registry);
  factory BeginSecureUploadResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BeginSecureUploadResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BeginSecureUploadResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.media.v1alpha1'),
      createEmptyInstance: BeginSecureUploadResponse.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'stagedToken')
    ..aI(2, _omitFieldNames ? '' : 'chunkSize')
    ..aI(3, _omitFieldNames ? '' : 'chunkCount')
    ..aOM<$0.Timestamp>(4, _omitFieldNames ? '' : 'expireTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BeginSecureUploadResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BeginSecureUploadResponse copyWith(
          void Function(BeginSecureUploadResponse) updates) =>
      super.copyWith((message) => updates(message as BeginSecureUploadResponse))
          as BeginSecureUploadResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use BeginSecureUploadResponse() / BeginSecureUploadResponse.new instead')
  static BeginSecureUploadResponse create() => BeginSecureUploadResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      BeginSecureUploadResponse._();
  @$core.override
  BeginSecureUploadResponse createEmptyInstance() =>
      BeginSecureUploadResponse._();
  @$core.pragma('dart2js:noInline')
  static BeginSecureUploadResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BeginSecureUploadResponse>(
          BeginSecureUploadResponse.$_createMessage);
  static BeginSecureUploadResponse? _defaultInstance;

  /// 不透明的憑證，放在段上傳與暫存預覽的網址路徑裡，交給擁有者的 API 轉正；只出現這一次。
  @$pb.TagNumber(1)
  $core.String get stagedToken => $_getSZ(0);
  @$pb.TagNumber(1)
  set stagedToken($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStagedToken() => $_has(0);
  @$pb.TagNumber(1)
  void clearStagedToken() => $_clearField(1);

  /// 每段的位元組數；除最後一段外每段剛好這個大小，最後一段是餘數。client 不寫死。
  @$pb.TagNumber(2)
  $core.int get chunkSize => $_getIZ(1);
  @$pb.TagNumber(2)
  set chunkSize($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasChunkSize() => $_has(1);
  @$pb.TagNumber(2)
  void clearChunkSize() => $_clearField(2);

  /// 段總數；段序號是 0 到 chunk_count - 1。
  @$pb.TagNumber(3)
  $core.int get chunkCount => $_getIZ(2);
  @$pb.TagNumber(3)
  set chunkCount($core.int value) => $_setSignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasChunkCount() => $_has(2);
  @$pb.TagNumber(3)
  void clearChunkCount() => $_clearField(3);

  /// 會話的到期時間；過了之後 staged token 不能再用。
  @$pb.TagNumber(4)
  $0.Timestamp get expireTime => $_getN(3);
  @$pb.TagNumber(4)
  set expireTime($0.Timestamp value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasExpireTime() => $_has(3);
  @$pb.TagNumber(4)
  void clearExpireTime() => $_clearField(4);
  @$pb.TagNumber(4)
  $0.Timestamp ensureExpireTime() => $_ensure(3);
}

/// MediaAppService 是登入的買家與賣家上傳公開圖片；管理員等非顧客帳號的身分不能呼叫（permission_denied）。
class MediaAppServiceApi {
  final $pb.RpcClient _client;

  MediaAppServiceApi(this._client);

  /// BeginUpload 宣告要上傳的檔案，回傳預簽的 PUT 網址、client 必須原樣帶上的標頭與 staged token。
  /// 回 invalid_argument 且不發網址的情況：size 不大於零或超過伺服器的上限（訊息是在地化文字，說明上限）；content_type 不是 image/jpeg、
  /// image/png、image/webp、image/gif 之一（HEIC 等由 client 先轉換）；md5 不是 16 位元組的標準 base64。
  /// 每次呼叫建立新的一筆暫存；網址與 staged token 只在這個回應出現一次，沒有用到的暫存由伺服器回收。
  $async.Future<BeginUploadResponse> beginUpload(
          $pb.ClientContext? ctx, BeginUploadRequest request) =>
      _client.invoke<BeginUploadResponse>(ctx, 'MediaAppService', 'BeginUpload',
          request, BeginUploadResponse());

  /// CompleteUpload 在 client 把檔案 PUT 完之後呼叫，伺服器開始非同步處理。可重送：已經完成的重送一律成功。
  /// staged token 不存在或不是呼叫者的回 not_found（兩者不可區分）；暫存已過期回 failed_precondition。
  $async.Future<CompleteUploadResponse> completeUpload(
          $pb.ClientContext? ctx, CompleteUploadRequest request) =>
      _client.invoke<CompleteUploadResponse>(ctx, 'MediaAppService',
          'CompleteUpload', request, CompleteUploadResponse());

  /// GetUpload 讀暫存的處理狀態。staged token 是憑證，所以不標 NO_SIDE_EFFECTS（以 POST 呼叫、不放進網址；規範第三節）；client 輪詢，失敗下一輪再讀。
  /// staged token 不存在或不是呼叫者的回 not_found（兩者不可區分）。
  $async.Future<GetUploadResponse> getUpload(
          $pb.ClientContext? ctx, GetUploadRequest request) =>
      _client.invoke<GetUploadResponse>(
          ctx, 'MediaAppService', 'GetUpload', request, GetUploadResponse());

  /// BeginSecureUpload 開一個機敏檔案的分段上傳會話（B1-D17）。回傳 staged token、段大小與段總數；client 依段大小把檔案切段，
  /// 從第 0 段起依序以 `PUT /api/files/secure_uploads/{staged_token}/chunks/{index}`（原始位元組 body，不經 Connect；規範第十三節）送出。
  /// size 小於一或超過伺服器的上限回 invalid_argument 指向 size（訊息是在地化文字）；伺服器沒有設定加密金鑰回 unavailable。
  /// 每次呼叫建立新的會話；沒有送完的由伺服器回收。
  $async.Future<BeginSecureUploadResponse> beginSecureUpload(
          $pb.ClientContext? ctx, BeginSecureUploadRequest request) =>
      _client.invoke<BeginSecureUploadResponse>(ctx, 'MediaAppService',
          'BeginSecureUpload', request, BeginSecureUploadResponse());
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
