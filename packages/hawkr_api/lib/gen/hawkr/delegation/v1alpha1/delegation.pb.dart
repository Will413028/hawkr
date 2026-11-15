// This is a generated file - do not edit.
//
// Generated from hawkr/delegation/v1alpha1/delegation.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;
import 'package:protobuf/well_known_types/google/protobuf/field_mask.pb.dart'
    as $1;
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart'
    as $2;

import '../../social/v1alpha1/social.pb.dart' as $0;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

/// DelegatedOperation 是每個代行寫入都要帶的稽核資料。
class DelegatedOperation extends $pb.GeneratedMessage {
  factory DelegatedOperation({
    $core.String? requestId,
    $core.String? operationReason,
    $core.bool? notifySeller,
  }) {
    final result = DelegatedOperation._();
    if (requestId != null) result.requestId = requestId;
    if (operationReason != null) result.operationReason = operationReason;
    if (notifySeller != null) result.notifySeller = notifySeller;
    return result;
  }

  DelegatedOperation._();

  factory DelegatedOperation.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DelegatedOperation()..mergeFromBuffer(data, registry);
  factory DelegatedOperation.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DelegatedOperation()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DelegatedOperation',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.delegation.v1alpha1'),
      createEmptyInstance: DelegatedOperation.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'requestId')
    ..aOS(2, _omitFieldNames ? '' : 'operationReason')
    ..aOB(3, _omitFieldNames ? '' : 'notifySeller')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DelegatedOperation clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DelegatedOperation copyWith(void Function(DelegatedOperation) updates) =>
      super.copyWith((message) => updates(message as DelegatedOperation))
          as DelegatedOperation;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use DelegatedOperation() / DelegatedOperation.new instead')
  static DelegatedOperation create() => DelegatedOperation._();
  static $pb.GeneratedMessage $_createMessage() => DelegatedOperation._();
  @$core.override
  DelegatedOperation createEmptyInstance() => DelegatedOperation._();
  @$core.pragma('dart2js:noInline')
  static DelegatedOperation getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DelegatedOperation>(
          DelegatedOperation.$_createMessage);
  static DelegatedOperation? _defaultInstance;

  /// UUIDv4（AIP-155）：同一個代行 session 內以同一個 request_id 重送，回第一次的結果，不重做寫入也不再寫稽核；同一個 request_id 用在不同的請求內容回 failed_precondition。必填。
  @$pb.TagNumber(1)
  $core.String get requestId => $_getSZ(0);
  @$pb.TagNumber(1)
  set requestId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRequestId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRequestId() => $_clearField(1);

  /// 操作原因，去除首尾空白後 1 到 100 個字元，不得含控制字元；寫進稽核，必填。
  @$pb.TagNumber(2)
  $core.String get operationReason => $_getSZ(1);
  @$pb.TagNumber(2)
  set operationReason($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasOperationReason() => $_has(1);
  @$pb.TagNumber(2)
  void clearOperationReason() => $_clearField(2);

  /// 是否在完成後通知店主；不設是通知。
  @$pb.TagNumber(3)
  $core.bool get notifySeller => $_getBF(2);
  @$pb.TagNumber(3)
  set notifySeller($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasNotifySeller() => $_has(2);
  @$pb.TagNumber(3)
  void clearNotifySeller() => $_clearField(3);
}

class ListPostsRequest extends $pb.GeneratedMessage {
  factory ListPostsRequest({
    $core.int? pageSize,
    $core.String? pageToken,
    $0.PostStatus? status,
  }) {
    final result = ListPostsRequest._();
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    if (status != null) result.status = status;
    return result;
  }

  ListPostsRequest._();

  factory ListPostsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListPostsRequest()..mergeFromBuffer(data, registry);
  factory ListPostsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListPostsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListPostsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.delegation.v1alpha1'),
      createEmptyInstance: ListPostsRequest.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'pageSize')
    ..aOS(2, _omitFieldNames ? '' : 'pageToken')
    ..aE<$0.PostStatus>(3, _omitFieldNames ? '' : 'status',
        enumValues: $0.PostStatus.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListPostsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListPostsRequest copyWith(void Function(ListPostsRequest) updates) =>
      super.copyWith((message) => updates(message as ListPostsRequest))
          as ListPostsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ListPostsRequest() / ListPostsRequest.new instead')
  static ListPostsRequest create() => ListPostsRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListPostsRequest._();
  @$core.override
  ListPostsRequest createEmptyInstance() => ListPostsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListPostsRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ListPostsRequest>(
          ListPostsRequest.$_createMessage);
  static ListPostsRequest? _defaultInstance;

  /// 預設 20，上限 50。
  @$pb.TagNumber(1)
  $core.int get pageSize => $_getIZ(0);
  @$pb.TagNumber(1)
  set pageSize($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPageSize() => $_has(0);
  @$pb.TagNumber(1)
  void clearPageSize() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get pageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set pageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearPageToken() => $_clearField(2);

  /// 只列這個狀態的貼文；不設（UNSPECIFIED）是全部。page_token 只能用在同一個 status。
  @$pb.TagNumber(3)
  $0.PostStatus get status => $_getN(2);
  @$pb.TagNumber(3)
  set status($0.PostStatus value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasStatus() => $_has(2);
  @$pb.TagNumber(3)
  void clearStatus() => $_clearField(3);
}

class ListPostsResponse extends $pb.GeneratedMessage {
  factory ListPostsResponse({
    $core.Iterable<$0.Post>? posts,
    $core.String? nextPageToken,
  }) {
    final result = ListPostsResponse._();
    if (posts != null) result.posts.addAll(posts);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    return result;
  }

  ListPostsResponse._();

  factory ListPostsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListPostsResponse()..mergeFromBuffer(data, registry);
  factory ListPostsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListPostsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListPostsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.delegation.v1alpha1'),
      createEmptyInstance: ListPostsResponse.$_createMessage)
    ..pPM<$0.Post>(1, _omitFieldNames ? '' : 'posts',
        subBuilder: $0.Post.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListPostsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListPostsResponse copyWith(void Function(ListPostsResponse) updates) =>
      super.copyWith((message) => updates(message as ListPostsResponse))
          as ListPostsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ListPostsResponse() / ListPostsResponse.new instead')
  static ListPostsResponse create() => ListPostsResponse._();
  static $pb.GeneratedMessage $_createMessage() => ListPostsResponse._();
  @$core.override
  ListPostsResponse createEmptyInstance() => ListPostsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListPostsResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ListPostsResponse>(
          ListPostsResponse.$_createMessage);
  static ListPostsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<$0.Post> get posts => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);
}

class GetPostRequest extends $pb.GeneratedMessage {
  factory GetPostRequest({
    $core.String? postId,
  }) {
    final result = GetPostRequest._();
    if (postId != null) result.postId = postId;
    return result;
  }

  GetPostRequest._();

  factory GetPostRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetPostRequest()..mergeFromBuffer(data, registry);
  factory GetPostRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetPostRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetPostRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.delegation.v1alpha1'),
      createEmptyInstance: GetPostRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'postId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPostRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPostRequest copyWith(void Function(GetPostRequest) updates) =>
      super.copyWith((message) => updates(message as GetPostRequest))
          as GetPostRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GetPostRequest() / GetPostRequest.new instead')
  static GetPostRequest create() => GetPostRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetPostRequest._();
  @$core.override
  GetPostRequest createEmptyInstance() => GetPostRequest._();
  @$core.pragma('dart2js:noInline')
  static GetPostRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<GetPostRequest>(
          GetPostRequest.$_createMessage);
  static GetPostRequest? _defaultInstance;

  /// 貼文 ID。
  @$pb.TagNumber(1)
  $core.String get postId => $_getSZ(0);
  @$pb.TagNumber(1)
  set postId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPostId() => $_has(0);
  @$pb.TagNumber(1)
  void clearPostId() => $_clearField(1);
}

class GetPostResponse extends $pb.GeneratedMessage {
  factory GetPostResponse({
    $0.Post? post,
  }) {
    final result = GetPostResponse._();
    if (post != null) result.post = post;
    return result;
  }

  GetPostResponse._();

  factory GetPostResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetPostResponse()..mergeFromBuffer(data, registry);
  factory GetPostResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetPostResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetPostResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.delegation.v1alpha1'),
      createEmptyInstance: GetPostResponse.$_createMessage)
    ..aOM<$0.Post>(1, _omitFieldNames ? '' : 'post',
        subBuilder: $0.Post.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPostResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPostResponse copyWith(void Function(GetPostResponse) updates) =>
      super.copyWith((message) => updates(message as GetPostResponse))
          as GetPostResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GetPostResponse() / GetPostResponse.new instead')
  static GetPostResponse create() => GetPostResponse._();
  static $pb.GeneratedMessage $_createMessage() => GetPostResponse._();
  @$core.override
  GetPostResponse createEmptyInstance() => GetPostResponse._();
  @$core.pragma('dart2js:noInline')
  static GetPostResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<GetPostResponse>(
          GetPostResponse.$_createMessage);
  static GetPostResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $0.Post get post => $_getN(0);
  @$pb.TagNumber(1)
  set post($0.Post value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPost() => $_has(0);
  @$pb.TagNumber(1)
  void clearPost() => $_clearField(1);
  @$pb.TagNumber(1)
  $0.Post ensurePost() => $_ensure(0);
}

class CreatePostRequest extends $pb.GeneratedMessage {
  factory CreatePostRequest({
    DelegatedOperation? operation,
    $core.String? body,
    $core.String? imageStagedToken,
    $core.String? productId,
    $core.Iterable<$core.String>? hashtagInputs,
  }) {
    final result = CreatePostRequest._();
    if (operation != null) result.operation = operation;
    if (body != null) result.body = body;
    if (imageStagedToken != null) result.imageStagedToken = imageStagedToken;
    if (productId != null) result.productId = productId;
    if (hashtagInputs != null) result.hashtagInputs.addAll(hashtagInputs);
    return result;
  }

  CreatePostRequest._();

  factory CreatePostRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreatePostRequest()..mergeFromBuffer(data, registry);
  factory CreatePostRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreatePostRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreatePostRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.delegation.v1alpha1'),
      createEmptyInstance: CreatePostRequest.$_createMessage)
    ..aOM<DelegatedOperation>(1, _omitFieldNames ? '' : 'operation',
        subBuilder: DelegatedOperation.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'body')
    ..aOS(3, _omitFieldNames ? '' : 'imageStagedToken')
    ..aOS(4, _omitFieldNames ? '' : 'productId')
    ..pPS(5, _omitFieldNames ? '' : 'hashtagInputs')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreatePostRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreatePostRequest copyWith(void Function(CreatePostRequest) updates) =>
      super.copyWith((message) => updates(message as CreatePostRequest))
          as CreatePostRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use CreatePostRequest() / CreatePostRequest.new instead')
  static CreatePostRequest create() => CreatePostRequest._();
  static $pb.GeneratedMessage $_createMessage() => CreatePostRequest._();
  @$core.override
  CreatePostRequest createEmptyInstance() => CreatePostRequest._();
  @$core.pragma('dart2js:noInline')
  static CreatePostRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<CreatePostRequest>(
          CreatePostRequest.$_createMessage);
  static CreatePostRequest? _defaultInstance;

  @$pb.TagNumber(1)
  DelegatedOperation get operation => $_getN(0);
  @$pb.TagNumber(1)
  set operation(DelegatedOperation value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasOperation() => $_has(0);
  @$pb.TagNumber(1)
  void clearOperation() => $_clearField(1);
  @$pb.TagNumber(1)
  DelegatedOperation ensureOperation() => $_ensure(0);

  /// 內文，去除首尾空白後最多 2200 個字元；草稿可以是空的。
  @$pb.TagNumber(2)
  $core.String get body => $_getSZ(1);
  @$pb.TagNumber(2)
  set body($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasBody() => $_has(1);
  @$pb.TagNumber(2)
  void clearBody() => $_clearField(2);

  /// 圖片的 staged token（media 上傳完成後拿到的）。
  @$pb.TagNumber(3)
  $core.String get imageStagedToken => $_getSZ(2);
  @$pb.TagNumber(3)
  set imageStagedToken($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasImageStagedToken() => $_has(2);
  @$pb.TagNumber(3)
  void clearImageStagedToken() => $_clearField(3);

  /// 貼文帶的商品 ID；商品要存在、上架中而且屬於這家店。
  @$pb.TagNumber(4)
  $core.String get productId => $_getSZ(3);
  @$pb.TagNumber(4)
  set productId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasProductId() => $_has(3);
  @$pb.TagNumber(4)
  void clearProductId() => $_clearField(4);

  /// 標籤（使用者輸入的樣子，可帶開頭的 #）。
  @$pb.TagNumber(5)
  $pb.PbList<$core.String> get hashtagInputs => $_getList(4);
}

class CreatePostResponse extends $pb.GeneratedMessage {
  factory CreatePostResponse({
    $0.Post? post,
    $core.String? auditLogId,
  }) {
    final result = CreatePostResponse._();
    if (post != null) result.post = post;
    if (auditLogId != null) result.auditLogId = auditLogId;
    return result;
  }

  CreatePostResponse._();

  factory CreatePostResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreatePostResponse()..mergeFromBuffer(data, registry);
  factory CreatePostResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreatePostResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreatePostResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.delegation.v1alpha1'),
      createEmptyInstance: CreatePostResponse.$_createMessage)
    ..aOM<$0.Post>(1, _omitFieldNames ? '' : 'post',
        subBuilder: $0.Post.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'auditLogId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreatePostResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreatePostResponse copyWith(void Function(CreatePostResponse) updates) =>
      super.copyWith((message) => updates(message as CreatePostResponse))
          as CreatePostResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use CreatePostResponse() / CreatePostResponse.new instead')
  static CreatePostResponse create() => CreatePostResponse._();
  static $pb.GeneratedMessage $_createMessage() => CreatePostResponse._();
  @$core.override
  CreatePostResponse createEmptyInstance() => CreatePostResponse._();
  @$core.pragma('dart2js:noInline')
  static CreatePostResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreatePostResponse>(
          CreatePostResponse.$_createMessage);
  static CreatePostResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $0.Post get post => $_getN(0);
  @$pb.TagNumber(1)
  set post($0.Post value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPost() => $_has(0);
  @$pb.TagNumber(1)
  void clearPost() => $_clearField(1);
  @$pb.TagNumber(1)
  $0.Post ensurePost() => $_ensure(0);

  /// 這次寫入的稽核列 ID（重送時是第一次的）。
  @$pb.TagNumber(2)
  $core.String get auditLogId => $_getSZ(1);
  @$pb.TagNumber(2)
  set auditLogId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAuditLogId() => $_has(1);
  @$pb.TagNumber(2)
  void clearAuditLogId() => $_clearField(2);
}

class UpdatePostRequest extends $pb.GeneratedMessage {
  factory UpdatePostRequest({
    DelegatedOperation? operation,
    $0.Post? post,
    $1.FieldMask? updateMask,
  }) {
    final result = UpdatePostRequest._();
    if (operation != null) result.operation = operation;
    if (post != null) result.post = post;
    if (updateMask != null) result.updateMask = updateMask;
    return result;
  }

  UpdatePostRequest._();

  factory UpdatePostRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdatePostRequest()..mergeFromBuffer(data, registry);
  factory UpdatePostRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdatePostRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdatePostRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.delegation.v1alpha1'),
      createEmptyInstance: UpdatePostRequest.$_createMessage)
    ..aOM<DelegatedOperation>(1, _omitFieldNames ? '' : 'operation',
        subBuilder: DelegatedOperation.$_createMessage)
    ..aOM<$0.Post>(2, _omitFieldNames ? '' : 'post',
        subBuilder: $0.Post.$_createMessage)
    ..aOM<$1.FieldMask>(3, _omitFieldNames ? '' : 'updateMask',
        subBuilder: $1.FieldMask.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdatePostRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdatePostRequest copyWith(void Function(UpdatePostRequest) updates) =>
      super.copyWith((message) => updates(message as UpdatePostRequest))
          as UpdatePostRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use UpdatePostRequest() / UpdatePostRequest.new instead')
  static UpdatePostRequest create() => UpdatePostRequest._();
  static $pb.GeneratedMessage $_createMessage() => UpdatePostRequest._();
  @$core.override
  UpdatePostRequest createEmptyInstance() => UpdatePostRequest._();
  @$core.pragma('dart2js:noInline')
  static UpdatePostRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<UpdatePostRequest>(
          UpdatePostRequest.$_createMessage);
  static UpdatePostRequest? _defaultInstance;

  @$pb.TagNumber(1)
  DelegatedOperation get operation => $_getN(0);
  @$pb.TagNumber(1)
  set operation(DelegatedOperation value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasOperation() => $_has(0);
  @$pb.TagNumber(1)
  void clearOperation() => $_clearField(1);
  @$pb.TagNumber(1)
  DelegatedOperation ensureOperation() => $_ensure(0);

  /// id 與 etag 必填。可以修改的欄位是 body、image_staged_token、product_id、hashtag_inputs，update_mask 要列至少一個；沒列的欄位保留目前的值。
  @$pb.TagNumber(2)
  $0.Post get post => $_getN(1);
  @$pb.TagNumber(2)
  set post($0.Post value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasPost() => $_has(1);
  @$pb.TagNumber(2)
  void clearPost() => $_clearField(2);
  @$pb.TagNumber(2)
  $0.Post ensurePost() => $_ensure(1);

  @$pb.TagNumber(3)
  $1.FieldMask get updateMask => $_getN(2);
  @$pb.TagNumber(3)
  set updateMask($1.FieldMask value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasUpdateMask() => $_has(2);
  @$pb.TagNumber(3)
  void clearUpdateMask() => $_clearField(3);
  @$pb.TagNumber(3)
  $1.FieldMask ensureUpdateMask() => $_ensure(2);
}

class UpdatePostResponse extends $pb.GeneratedMessage {
  factory UpdatePostResponse({
    $0.Post? post,
    $core.String? auditLogId,
  }) {
    final result = UpdatePostResponse._();
    if (post != null) result.post = post;
    if (auditLogId != null) result.auditLogId = auditLogId;
    return result;
  }

  UpdatePostResponse._();

  factory UpdatePostResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdatePostResponse()..mergeFromBuffer(data, registry);
  factory UpdatePostResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdatePostResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdatePostResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.delegation.v1alpha1'),
      createEmptyInstance: UpdatePostResponse.$_createMessage)
    ..aOM<$0.Post>(1, _omitFieldNames ? '' : 'post',
        subBuilder: $0.Post.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'auditLogId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdatePostResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdatePostResponse copyWith(void Function(UpdatePostResponse) updates) =>
      super.copyWith((message) => updates(message as UpdatePostResponse))
          as UpdatePostResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use UpdatePostResponse() / UpdatePostResponse.new instead')
  static UpdatePostResponse create() => UpdatePostResponse._();
  static $pb.GeneratedMessage $_createMessage() => UpdatePostResponse._();
  @$core.override
  UpdatePostResponse createEmptyInstance() => UpdatePostResponse._();
  @$core.pragma('dart2js:noInline')
  static UpdatePostResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdatePostResponse>(
          UpdatePostResponse.$_createMessage);
  static UpdatePostResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $0.Post get post => $_getN(0);
  @$pb.TagNumber(1)
  set post($0.Post value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPost() => $_has(0);
  @$pb.TagNumber(1)
  void clearPost() => $_clearField(1);
  @$pb.TagNumber(1)
  $0.Post ensurePost() => $_ensure(0);

  @$pb.TagNumber(2)
  $core.String get auditLogId => $_getSZ(1);
  @$pb.TagNumber(2)
  set auditLogId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAuditLogId() => $_has(1);
  @$pb.TagNumber(2)
  void clearAuditLogId() => $_clearField(2);
}

class PublishPostRequest extends $pb.GeneratedMessage {
  factory PublishPostRequest({
    DelegatedOperation? operation,
    $core.String? postId,
    $core.String? etag,
  }) {
    final result = PublishPostRequest._();
    if (operation != null) result.operation = operation;
    if (postId != null) result.postId = postId;
    if (etag != null) result.etag = etag;
    return result;
  }

  PublishPostRequest._();

  factory PublishPostRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PublishPostRequest()..mergeFromBuffer(data, registry);
  factory PublishPostRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PublishPostRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PublishPostRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.delegation.v1alpha1'),
      createEmptyInstance: PublishPostRequest.$_createMessage)
    ..aOM<DelegatedOperation>(1, _omitFieldNames ? '' : 'operation',
        subBuilder: DelegatedOperation.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'postId')
    ..aOS(3, _omitFieldNames ? '' : 'etag')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PublishPostRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PublishPostRequest copyWith(void Function(PublishPostRequest) updates) =>
      super.copyWith((message) => updates(message as PublishPostRequest))
          as PublishPostRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use PublishPostRequest() / PublishPostRequest.new instead')
  static PublishPostRequest create() => PublishPostRequest._();
  static $pb.GeneratedMessage $_createMessage() => PublishPostRequest._();
  @$core.override
  PublishPostRequest createEmptyInstance() => PublishPostRequest._();
  @$core.pragma('dart2js:noInline')
  static PublishPostRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PublishPostRequest>(
          PublishPostRequest.$_createMessage);
  static PublishPostRequest? _defaultInstance;

  @$pb.TagNumber(1)
  DelegatedOperation get operation => $_getN(0);
  @$pb.TagNumber(1)
  set operation(DelegatedOperation value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasOperation() => $_has(0);
  @$pb.TagNumber(1)
  void clearOperation() => $_clearField(1);
  @$pb.TagNumber(1)
  DelegatedOperation ensureOperation() => $_ensure(0);

  /// 貼文 ID。
  @$pb.TagNumber(2)
  $core.String get postId => $_getSZ(1);
  @$pb.TagNumber(2)
  set postId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPostId() => $_has(1);
  @$pb.TagNumber(2)
  void clearPostId() => $_clearField(2);

  /// 讀取時拿到的 etag，必填。
  @$pb.TagNumber(3)
  $core.String get etag => $_getSZ(2);
  @$pb.TagNumber(3)
  set etag($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasEtag() => $_has(2);
  @$pb.TagNumber(3)
  void clearEtag() => $_clearField(3);
}

class PublishPostResponse extends $pb.GeneratedMessage {
  factory PublishPostResponse({
    $0.Post? post,
    $core.String? auditLogId,
  }) {
    final result = PublishPostResponse._();
    if (post != null) result.post = post;
    if (auditLogId != null) result.auditLogId = auditLogId;
    return result;
  }

  PublishPostResponse._();

  factory PublishPostResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PublishPostResponse()..mergeFromBuffer(data, registry);
  factory PublishPostResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PublishPostResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PublishPostResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.delegation.v1alpha1'),
      createEmptyInstance: PublishPostResponse.$_createMessage)
    ..aOM<$0.Post>(1, _omitFieldNames ? '' : 'post',
        subBuilder: $0.Post.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'auditLogId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PublishPostResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PublishPostResponse copyWith(void Function(PublishPostResponse) updates) =>
      super.copyWith((message) => updates(message as PublishPostResponse))
          as PublishPostResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use PublishPostResponse() / PublishPostResponse.new instead')
  static PublishPostResponse create() => PublishPostResponse._();
  static $pb.GeneratedMessage $_createMessage() => PublishPostResponse._();
  @$core.override
  PublishPostResponse createEmptyInstance() => PublishPostResponse._();
  @$core.pragma('dart2js:noInline')
  static PublishPostResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PublishPostResponse>(
          PublishPostResponse.$_createMessage);
  static PublishPostResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $0.Post get post => $_getN(0);
  @$pb.TagNumber(1)
  set post($0.Post value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPost() => $_has(0);
  @$pb.TagNumber(1)
  void clearPost() => $_clearField(1);
  @$pb.TagNumber(1)
  $0.Post ensurePost() => $_ensure(0);

  @$pb.TagNumber(2)
  $core.String get auditLogId => $_getSZ(1);
  @$pb.TagNumber(2)
  set auditLogId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAuditLogId() => $_has(1);
  @$pb.TagNumber(2)
  void clearAuditLogId() => $_clearField(2);
}

class UnpublishPostRequest extends $pb.GeneratedMessage {
  factory UnpublishPostRequest({
    DelegatedOperation? operation,
    $core.String? postId,
    $core.String? etag,
  }) {
    final result = UnpublishPostRequest._();
    if (operation != null) result.operation = operation;
    if (postId != null) result.postId = postId;
    if (etag != null) result.etag = etag;
    return result;
  }

  UnpublishPostRequest._();

  factory UnpublishPostRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnpublishPostRequest()..mergeFromBuffer(data, registry);
  factory UnpublishPostRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnpublishPostRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UnpublishPostRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.delegation.v1alpha1'),
      createEmptyInstance: UnpublishPostRequest.$_createMessage)
    ..aOM<DelegatedOperation>(1, _omitFieldNames ? '' : 'operation',
        subBuilder: DelegatedOperation.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'postId')
    ..aOS(3, _omitFieldNames ? '' : 'etag')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnpublishPostRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnpublishPostRequest copyWith(void Function(UnpublishPostRequest) updates) =>
      super.copyWith((message) => updates(message as UnpublishPostRequest))
          as UnpublishPostRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use UnpublishPostRequest() / UnpublishPostRequest.new instead')
  static UnpublishPostRequest create() => UnpublishPostRequest._();
  static $pb.GeneratedMessage $_createMessage() => UnpublishPostRequest._();
  @$core.override
  UnpublishPostRequest createEmptyInstance() => UnpublishPostRequest._();
  @$core.pragma('dart2js:noInline')
  static UnpublishPostRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UnpublishPostRequest>(
          UnpublishPostRequest.$_createMessage);
  static UnpublishPostRequest? _defaultInstance;

  @$pb.TagNumber(1)
  DelegatedOperation get operation => $_getN(0);
  @$pb.TagNumber(1)
  set operation(DelegatedOperation value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasOperation() => $_has(0);
  @$pb.TagNumber(1)
  void clearOperation() => $_clearField(1);
  @$pb.TagNumber(1)
  DelegatedOperation ensureOperation() => $_ensure(0);

  /// 貼文 ID。
  @$pb.TagNumber(2)
  $core.String get postId => $_getSZ(1);
  @$pb.TagNumber(2)
  set postId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPostId() => $_has(1);
  @$pb.TagNumber(2)
  void clearPostId() => $_clearField(2);

  /// 讀取時拿到的 etag，必填。
  @$pb.TagNumber(3)
  $core.String get etag => $_getSZ(2);
  @$pb.TagNumber(3)
  set etag($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasEtag() => $_has(2);
  @$pb.TagNumber(3)
  void clearEtag() => $_clearField(3);
}

class UnpublishPostResponse extends $pb.GeneratedMessage {
  factory UnpublishPostResponse({
    $0.Post? post,
    $core.String? auditLogId,
  }) {
    final result = UnpublishPostResponse._();
    if (post != null) result.post = post;
    if (auditLogId != null) result.auditLogId = auditLogId;
    return result;
  }

  UnpublishPostResponse._();

  factory UnpublishPostResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnpublishPostResponse()..mergeFromBuffer(data, registry);
  factory UnpublishPostResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnpublishPostResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UnpublishPostResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.delegation.v1alpha1'),
      createEmptyInstance: UnpublishPostResponse.$_createMessage)
    ..aOM<$0.Post>(1, _omitFieldNames ? '' : 'post',
        subBuilder: $0.Post.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'auditLogId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnpublishPostResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnpublishPostResponse copyWith(
          void Function(UnpublishPostResponse) updates) =>
      super.copyWith((message) => updates(message as UnpublishPostResponse))
          as UnpublishPostResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use UnpublishPostResponse() / UnpublishPostResponse.new instead')
  static UnpublishPostResponse create() => UnpublishPostResponse._();
  static $pb.GeneratedMessage $_createMessage() => UnpublishPostResponse._();
  @$core.override
  UnpublishPostResponse createEmptyInstance() => UnpublishPostResponse._();
  @$core.pragma('dart2js:noInline')
  static UnpublishPostResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UnpublishPostResponse>(
          UnpublishPostResponse.$_createMessage);
  static UnpublishPostResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $0.Post get post => $_getN(0);
  @$pb.TagNumber(1)
  set post($0.Post value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPost() => $_has(0);
  @$pb.TagNumber(1)
  void clearPost() => $_clearField(1);
  @$pb.TagNumber(1)
  $0.Post ensurePost() => $_ensure(0);

  @$pb.TagNumber(2)
  $core.String get auditLogId => $_getSZ(1);
  @$pb.TagNumber(2)
  set auditLogId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAuditLogId() => $_has(1);
  @$pb.TagNumber(2)
  void clearAuditLogId() => $_clearField(2);
}

class SchedulePostRequest extends $pb.GeneratedMessage {
  factory SchedulePostRequest({
    DelegatedOperation? operation,
    $core.String? postId,
    $core.String? etag,
    $2.Timestamp? publishTime,
  }) {
    final result = SchedulePostRequest._();
    if (operation != null) result.operation = operation;
    if (postId != null) result.postId = postId;
    if (etag != null) result.etag = etag;
    if (publishTime != null) result.publishTime = publishTime;
    return result;
  }

  SchedulePostRequest._();

  factory SchedulePostRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SchedulePostRequest()..mergeFromBuffer(data, registry);
  factory SchedulePostRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SchedulePostRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SchedulePostRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.delegation.v1alpha1'),
      createEmptyInstance: SchedulePostRequest.$_createMessage)
    ..aOM<DelegatedOperation>(1, _omitFieldNames ? '' : 'operation',
        subBuilder: DelegatedOperation.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'postId')
    ..aOS(3, _omitFieldNames ? '' : 'etag')
    ..aOM<$2.Timestamp>(4, _omitFieldNames ? '' : 'publishTime',
        subBuilder: $2.Timestamp.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SchedulePostRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SchedulePostRequest copyWith(void Function(SchedulePostRequest) updates) =>
      super.copyWith((message) => updates(message as SchedulePostRequest))
          as SchedulePostRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use SchedulePostRequest() / SchedulePostRequest.new instead')
  static SchedulePostRequest create() => SchedulePostRequest._();
  static $pb.GeneratedMessage $_createMessage() => SchedulePostRequest._();
  @$core.override
  SchedulePostRequest createEmptyInstance() => SchedulePostRequest._();
  @$core.pragma('dart2js:noInline')
  static SchedulePostRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SchedulePostRequest>(
          SchedulePostRequest.$_createMessage);
  static SchedulePostRequest? _defaultInstance;

  @$pb.TagNumber(1)
  DelegatedOperation get operation => $_getN(0);
  @$pb.TagNumber(1)
  set operation(DelegatedOperation value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasOperation() => $_has(0);
  @$pb.TagNumber(1)
  void clearOperation() => $_clearField(1);
  @$pb.TagNumber(1)
  DelegatedOperation ensureOperation() => $_ensure(0);

  /// 貼文 ID。
  @$pb.TagNumber(2)
  $core.String get postId => $_getSZ(1);
  @$pb.TagNumber(2)
  set postId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPostId() => $_has(1);
  @$pb.TagNumber(2)
  void clearPostId() => $_clearField(2);

  /// 讀取時拿到的 etag，必填。
  @$pb.TagNumber(3)
  $core.String get etag => $_getSZ(2);
  @$pb.TagNumber(3)
  set etag($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasEtag() => $_has(2);
  @$pb.TagNumber(3)
  void clearEtag() => $_clearField(3);

  /// 發布的時間點，距現在 5 分鐘到 90 天。
  @$pb.TagNumber(4)
  $2.Timestamp get publishTime => $_getN(3);
  @$pb.TagNumber(4)
  set publishTime($2.Timestamp value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasPublishTime() => $_has(3);
  @$pb.TagNumber(4)
  void clearPublishTime() => $_clearField(4);
  @$pb.TagNumber(4)
  $2.Timestamp ensurePublishTime() => $_ensure(3);
}

class SchedulePostResponse extends $pb.GeneratedMessage {
  factory SchedulePostResponse({
    $0.Post? post,
    $2.Timestamp? publishTime,
    $core.String? auditLogId,
  }) {
    final result = SchedulePostResponse._();
    if (post != null) result.post = post;
    if (publishTime != null) result.publishTime = publishTime;
    if (auditLogId != null) result.auditLogId = auditLogId;
    return result;
  }

  SchedulePostResponse._();

  factory SchedulePostResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SchedulePostResponse()..mergeFromBuffer(data, registry);
  factory SchedulePostResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SchedulePostResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SchedulePostResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.delegation.v1alpha1'),
      createEmptyInstance: SchedulePostResponse.$_createMessage)
    ..aOM<$0.Post>(1, _omitFieldNames ? '' : 'post',
        subBuilder: $0.Post.$_createMessage)
    ..aOM<$2.Timestamp>(2, _omitFieldNames ? '' : 'publishTime',
        subBuilder: $2.Timestamp.$_createMessage)
    ..aOS(3, _omitFieldNames ? '' : 'auditLogId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SchedulePostResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SchedulePostResponse copyWith(void Function(SchedulePostResponse) updates) =>
      super.copyWith((message) => updates(message as SchedulePostResponse))
          as SchedulePostResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SchedulePostResponse() / SchedulePostResponse.new instead')
  static SchedulePostResponse create() => SchedulePostResponse._();
  static $pb.GeneratedMessage $_createMessage() => SchedulePostResponse._();
  @$core.override
  SchedulePostResponse createEmptyInstance() => SchedulePostResponse._();
  @$core.pragma('dart2js:noInline')
  static SchedulePostResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SchedulePostResponse>(
          SchedulePostResponse.$_createMessage);
  static SchedulePostResponse? _defaultInstance;

  /// 排程後的貼文（含新的 etag）。
  @$pb.TagNumber(1)
  $0.Post get post => $_getN(0);
  @$pb.TagNumber(1)
  set post($0.Post value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPost() => $_has(0);
  @$pb.TagNumber(1)
  void clearPost() => $_clearField(1);
  @$pb.TagNumber(1)
  $0.Post ensurePost() => $_ensure(0);

  /// 排定的發布時間。
  @$pb.TagNumber(2)
  $2.Timestamp get publishTime => $_getN(1);
  @$pb.TagNumber(2)
  set publishTime($2.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasPublishTime() => $_has(1);
  @$pb.TagNumber(2)
  void clearPublishTime() => $_clearField(2);
  @$pb.TagNumber(2)
  $2.Timestamp ensurePublishTime() => $_ensure(1);

  @$pb.TagNumber(3)
  $core.String get auditLogId => $_getSZ(2);
  @$pb.TagNumber(3)
  set auditLogId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasAuditLogId() => $_has(2);
  @$pb.TagNumber(3)
  void clearAuditLogId() => $_clearField(3);
}

/// DelegationDelegatedService 是代行營運中的平台管理員：憑證是 X-HK-Operation-Session 標頭裡的代行 session，不讀顧客或管理員的身分。
/// 回應一律不可快取（private, no-store）。目前只有貼文的方法；商品與訂單在 A10 加進這個 service。
class DelegationDelegatedServiceApi {
  final $pb.RpcClient _client;

  DelegationDelegatedServiceApi(this._client);

  /// ListPosts 列出代行店家自己的貼文（含草稿與下架，不含已刪除），依建立時間新到舊（同一時間以 ID 排序）；可依 status 篩選。需要能力 posts.read。
  $async.Future<ListPostsResponse> listPosts(
          $pb.ClientContext? ctx, ListPostsRequest request) =>
      _client.invoke<ListPostsResponse>(ctx, 'DelegationDelegatedService',
          'ListPosts', request, ListPostsResponse());

  /// GetPost 讀代行店家的一篇貼文（任何狀態，不含已刪除）；別家的貼文與不存在一樣回 not_found。回應帶 status 與 etag。需要能力 posts.read。
  $async.Future<GetPostResponse> getPost(
          $pb.ClientContext? ctx, GetPostRequest request) =>
      _client.invoke<GetPostResponse>(ctx, 'DelegationDelegatedService',
          'GetPost', request, GetPostResponse());

  /// CreatePost 以代行店家建立草稿貼文（建立一律是草稿；要發布再呼叫 PublishPost）。同一個 session 以同一個 request_id 重送回第一次建立的貼文目前的樣子，
  /// 不重複建立、不重複寫稽核；內容不同回 failed_precondition。需要能力 posts.write。
  $async.Future<CreatePostResponse> createPost(
          $pb.ClientContext? ctx, CreatePostRequest request) =>
      _client.invoke<CreatePostResponse>(ctx, 'DelegationDelegatedService',
          'CreatePost', request, CreatePostResponse());

  /// UpdatePost 修改代行店家的貼文（update_mask 列 body、image_staged_token、product_id、hashtag_inputs 的子集）；etag 必填，與目前版本不符回 aborted 與
  /// VERSION_CONFLICT，不寫入也不留稽核。重送規則同 CreatePost。需要能力 posts.write。
  $async.Future<UpdatePostResponse> updatePost(
          $pb.ClientContext? ctx, UpdatePostRequest request) =>
      _client.invoke<UpdatePostResponse>(ctx, 'DelegationDelegatedService',
          'UpdatePost', request, UpdatePostResponse());

  /// PublishPost 立即發布草稿或下架的貼文；etag 必填。重送規則同 CreatePost。需要能力 posts.publish。
  $async.Future<PublishPostResponse> publishPost(
          $pb.ClientContext? ctx, PublishPostRequest request) =>
      _client.invoke<PublishPostResponse>(ctx, 'DelegationDelegatedService',
          'PublishPost', request, PublishPostResponse());

  /// UnpublishPost 下架已發布的貼文；etag 必填。重送規則同 CreatePost。需要能力 posts.publish。
  $async.Future<UnpublishPostResponse> unpublishPost(
          $pb.ClientContext? ctx, UnpublishPostRequest request) =>
      _client.invoke<UnpublishPostResponse>(ctx, 'DelegationDelegatedService',
          'UnpublishPost', request, UnpublishPostResponse());

  /// SchedulePost 排程發布：貼文要是草稿或已下架，publish_time 距現在 5 分鐘到 90 天；貼文已有待發布的排程時，舊的被取代。排程屬於店家，session 結束後仍會
  /// 在到點時重驗授權並發布（授權已撤回則取消）。稽核寫 POST_SCHEDULE，到點發布時另寫一列 POST_PUBLISH。重送規則同 CreatePost。需要能力 posts.publish。
  $async.Future<SchedulePostResponse> schedulePost(
          $pb.ClientContext? ctx, SchedulePostRequest request) =>
      _client.invoke<SchedulePostResponse>(ctx, 'DelegationDelegatedService',
          'SchedulePost', request, SchedulePostResponse());
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
