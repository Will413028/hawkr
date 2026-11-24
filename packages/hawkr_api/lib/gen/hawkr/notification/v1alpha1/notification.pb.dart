// This is a generated file - do not edit.
//
// Generated from hawkr/notification/v1alpha1/notification.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart'
    as $1;

import '../../common/v1alpha1/image.pb.dart' as $0;
import 'notification.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'notification.pbenum.dart';

/// InboxItem 是收件匣的一則通知，文字已依請求語系組好；唯一例外是 category 為 ANNOUNCEMENT 的公告，title 與 body
/// 是管理員寫的英文單一份，不依請求語系。
class InboxItem extends $pb.GeneratedMessage {
  factory InboxItem({
    $core.String? id,
    NotificationCategory? category,
    InboxPresentation? presentation,
    $core.String? title,
    $core.String? body,
    $core.String? deepLink,
    $0.Image? image,
    $core.bool? read,
    $1.Timestamp? readTime,
    $1.Timestamp? createTime,
  }) {
    final result = InboxItem._();
    if (id != null) result.id = id;
    if (category != null) result.category = category;
    if (presentation != null) result.presentation = presentation;
    if (title != null) result.title = title;
    if (body != null) result.body = body;
    if (deepLink != null) result.deepLink = deepLink;
    if (image != null) result.image = image;
    if (read != null) result.read = read;
    if (readTime != null) result.readTime = readTime;
    if (createTime != null) result.createTime = createTime;
    return result;
  }

  InboxItem._();

  factory InboxItem.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      InboxItem()..mergeFromBuffer(data, registry);
  factory InboxItem.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      InboxItem()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'InboxItem',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.notification.v1alpha1'),
      createEmptyInstance: InboxItem.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aE<NotificationCategory>(2, _omitFieldNames ? '' : 'category',
        enumValues: NotificationCategory.values)
    ..aE<InboxPresentation>(3, _omitFieldNames ? '' : 'presentation',
        enumValues: InboxPresentation.values)
    ..aOS(4, _omitFieldNames ? '' : 'title')
    ..aOS(5, _omitFieldNames ? '' : 'body')
    ..aOS(6, _omitFieldNames ? '' : 'deepLink')
    ..aOM<$0.Image>(7, _omitFieldNames ? '' : 'image',
        subBuilder: $0.Image.$_createMessage)
    ..aOB(8, _omitFieldNames ? '' : 'read')
    ..aOM<$1.Timestamp>(9, _omitFieldNames ? '' : 'readTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..aOM<$1.Timestamp>(10, _omitFieldNames ? '' : 'createTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  InboxItem clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  InboxItem copyWith(void Function(InboxItem) updates) =>
      super.copyWith((message) => updates(message as InboxItem)) as InboxItem;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use InboxItem() / InboxItem.new instead')
  static InboxItem create() => InboxItem._();
  static $pb.GeneratedMessage $_createMessage() => InboxItem._();
  @$core.override
  InboxItem createEmptyInstance() => InboxItem._();
  @$core.pragma('dart2js:noInline')
  static InboxItem getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<InboxItem>(InboxItem.$_createMessage);
  static InboxItem? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  NotificationCategory get category => $_getN(1);
  @$pb.TagNumber(2)
  set category(NotificationCategory value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasCategory() => $_has(1);
  @$pb.TagNumber(2)
  void clearCategory() => $_clearField(2);

  @$pb.TagNumber(3)
  InboxPresentation get presentation => $_getN(2);
  @$pb.TagNumber(3)
  set presentation(InboxPresentation value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasPresentation() => $_has(2);
  @$pb.TagNumber(3)
  void clearPresentation() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get title => $_getSZ(3);
  @$pb.TagNumber(4)
  set title($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasTitle() => $_has(3);
  @$pb.TagNumber(4)
  void clearTitle() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get body => $_getSZ(4);
  @$pb.TagNumber(5)
  set body($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasBody() => $_has(4);
  @$pb.TagNumber(5)
  void clearBody() => $_clearField(5);

  /// 點擊後的目的地；空字串表示沒有目的地。格式：website 相對路徑（以 `/` 開頭、不含 host），收件人語系是 en 時
  /// 帶 `/en` 前綴、zh-TW 不帶（多語系 ADR D1）；路徑一定是既有的 website 路由。推播 data 的 `deep_link` 鍵是同一個值。
  /// client 把路徑對應到自己的畫面（universal link／App Links 接受 `/en/…`），不認得的路徑開收件匣。
  /// 路徑表（目的地、參數、受眾、沒有 website 路由的目的地）見計畫附件
  /// `backend/docs/plans/2026-11-23-a8-notification/deep-links.md`。
  @$pb.TagNumber(6)
  $core.String get deepLink => $_getSZ(5);
  @$pb.TagNumber(6)
  set deepLink($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasDeepLink() => $_has(5);
  @$pb.TagNumber(6)
  void clearDeepLink() => $_clearField(6);

  /// 商品類通知的圖片；沒有圖時不設。
  @$pb.TagNumber(7)
  $0.Image get image => $_getN(6);
  @$pb.TagNumber(7)
  set image($0.Image value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasImage() => $_has(6);
  @$pb.TagNumber(7)
  void clearImage() => $_clearField(7);
  @$pb.TagNumber(7)
  $0.Image ensureImage() => $_ensure(6);

  @$pb.TagNumber(8)
  $core.bool get read => $_getBF(7);
  @$pb.TagNumber(8)
  set read($core.bool value) => $_setBool(7, value);
  @$pb.TagNumber(8)
  $core.bool hasRead() => $_has(7);
  @$pb.TagNumber(8)
  void clearRead() => $_clearField(8);

  /// 已讀的時間；未讀不設。
  @$pb.TagNumber(9)
  $1.Timestamp get readTime => $_getN(8);
  @$pb.TagNumber(9)
  set readTime($1.Timestamp value) => $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasReadTime() => $_has(8);
  @$pb.TagNumber(9)
  void clearReadTime() => $_clearField(9);
  @$pb.TagNumber(9)
  $1.Timestamp ensureReadTime() => $_ensure(8);

  @$pb.TagNumber(10)
  $1.Timestamp get createTime => $_getN(9);
  @$pb.TagNumber(10)
  set createTime($1.Timestamp value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasCreateTime() => $_has(9);
  @$pb.TagNumber(10)
  void clearCreateTime() => $_clearField(10);
  @$pb.TagNumber(10)
  $1.Timestamp ensureCreateTime() => $_ensure(9);
}

class ListInboxItemsRequest extends $pb.GeneratedMessage {
  factory ListInboxItemsRequest({
    NotificationAudience? audience,
    NotificationCategory? category,
    $core.int? pageSize,
    $core.String? pageToken,
  }) {
    final result = ListInboxItemsRequest._();
    if (audience != null) result.audience = audience;
    if (category != null) result.category = category;
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    return result;
  }

  ListInboxItemsRequest._();

  factory ListInboxItemsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListInboxItemsRequest()..mergeFromBuffer(data, registry);
  factory ListInboxItemsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListInboxItemsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListInboxItemsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.notification.v1alpha1'),
      createEmptyInstance: ListInboxItemsRequest.$_createMessage)
    ..aE<NotificationAudience>(1, _omitFieldNames ? '' : 'audience',
        enumValues: NotificationAudience.values)
    ..aE<NotificationCategory>(2, _omitFieldNames ? '' : 'category',
        enumValues: NotificationCategory.values)
    ..aI(3, _omitFieldNames ? '' : 'pageSize')
    ..aOS(4, _omitFieldNames ? '' : 'pageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListInboxItemsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListInboxItemsRequest copyWith(
          void Function(ListInboxItemsRequest) updates) =>
      super.copyWith((message) => updates(message as ListInboxItemsRequest))
          as ListInboxItemsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListInboxItemsRequest() / ListInboxItemsRequest.new instead')
  static ListInboxItemsRequest create() => ListInboxItemsRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListInboxItemsRequest._();
  @$core.override
  ListInboxItemsRequest createEmptyInstance() => ListInboxItemsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListInboxItemsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListInboxItemsRequest>(
          ListInboxItemsRequest.$_createMessage);
  static ListInboxItemsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  NotificationAudience get audience => $_getN(0);
  @$pb.TagNumber(1)
  set audience(NotificationAudience value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasAudience() => $_has(0);
  @$pb.TagNumber(1)
  void clearAudience() => $_clearField(1);

  /// 只列這個分類；不設表示全部。分類不屬於 audience 回 invalid_argument。
  @$pb.TagNumber(2)
  NotificationCategory get category => $_getN(1);
  @$pb.TagNumber(2)
  set category(NotificationCategory value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasCategory() => $_has(1);
  @$pb.TagNumber(2)
  void clearCategory() => $_clearField(2);

  /// 預設 20，上限 50。
  @$pb.TagNumber(3)
  $core.int get pageSize => $_getIZ(2);
  @$pb.TagNumber(3)
  set pageSize($core.int value) => $_setSignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPageSize() => $_has(2);
  @$pb.TagNumber(3)
  void clearPageSize() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get pageToken => $_getSZ(3);
  @$pb.TagNumber(4)
  set pageToken($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasPageToken() => $_has(3);
  @$pb.TagNumber(4)
  void clearPageToken() => $_clearField(4);
}

class ListInboxItemsResponse extends $pb.GeneratedMessage {
  factory ListInboxItemsResponse({
    $core.Iterable<InboxItem>? items,
    $core.String? nextPageToken,
    $core.int? totalSize,
  }) {
    final result = ListInboxItemsResponse._();
    if (items != null) result.items.addAll(items);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    if (totalSize != null) result.totalSize = totalSize;
    return result;
  }

  ListInboxItemsResponse._();

  factory ListInboxItemsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListInboxItemsResponse()..mergeFromBuffer(data, registry);
  factory ListInboxItemsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListInboxItemsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListInboxItemsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.notification.v1alpha1'),
      createEmptyInstance: ListInboxItemsResponse.$_createMessage)
    ..pPM<InboxItem>(1, _omitFieldNames ? '' : 'items',
        subBuilder: InboxItem.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..aI(3, _omitFieldNames ? '' : 'totalSize')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListInboxItemsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListInboxItemsResponse copyWith(
          void Function(ListInboxItemsResponse) updates) =>
      super.copyWith((message) => updates(message as ListInboxItemsResponse))
          as ListInboxItemsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListInboxItemsResponse() / ListInboxItemsResponse.new instead')
  static ListInboxItemsResponse create() => ListInboxItemsResponse._();
  static $pb.GeneratedMessage $_createMessage() => ListInboxItemsResponse._();
  @$core.override
  ListInboxItemsResponse createEmptyInstance() => ListInboxItemsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListInboxItemsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListInboxItemsResponse>(
          ListInboxItemsResponse.$_createMessage);
  static ListInboxItemsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<InboxItem> get items => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);

  /// 同一個 audience 與 category 篩選下的全部筆數。
  @$pb.TagNumber(3)
  $core.int get totalSize => $_getIZ(2);
  @$pb.TagNumber(3)
  set totalSize($core.int value) => $_setSignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasTotalSize() => $_has(2);
  @$pb.TagNumber(3)
  void clearTotalSize() => $_clearField(3);
}

class GetUnreadCountRequest extends $pb.GeneratedMessage {
  factory GetUnreadCountRequest({
    NotificationAudience? audience,
  }) {
    final result = GetUnreadCountRequest._();
    if (audience != null) result.audience = audience;
    return result;
  }

  GetUnreadCountRequest._();

  factory GetUnreadCountRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetUnreadCountRequest()..mergeFromBuffer(data, registry);
  factory GetUnreadCountRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetUnreadCountRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetUnreadCountRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.notification.v1alpha1'),
      createEmptyInstance: GetUnreadCountRequest.$_createMessage)
    ..aE<NotificationAudience>(1, _omitFieldNames ? '' : 'audience',
        enumValues: NotificationAudience.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetUnreadCountRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetUnreadCountRequest copyWith(
          void Function(GetUnreadCountRequest) updates) =>
      super.copyWith((message) => updates(message as GetUnreadCountRequest))
          as GetUnreadCountRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetUnreadCountRequest() / GetUnreadCountRequest.new instead')
  static GetUnreadCountRequest create() => GetUnreadCountRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetUnreadCountRequest._();
  @$core.override
  GetUnreadCountRequest createEmptyInstance() => GetUnreadCountRequest._();
  @$core.pragma('dart2js:noInline')
  static GetUnreadCountRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetUnreadCountRequest>(
          GetUnreadCountRequest.$_createMessage);
  static GetUnreadCountRequest? _defaultInstance;

  @$pb.TagNumber(1)
  NotificationAudience get audience => $_getN(0);
  @$pb.TagNumber(1)
  set audience(NotificationAudience value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasAudience() => $_has(0);
  @$pb.TagNumber(1)
  void clearAudience() => $_clearField(1);
}

class GetUnreadCountResponse extends $pb.GeneratedMessage {
  factory GetUnreadCountResponse({
    $core.int? unreadCount,
  }) {
    final result = GetUnreadCountResponse._();
    if (unreadCount != null) result.unreadCount = unreadCount;
    return result;
  }

  GetUnreadCountResponse._();

  factory GetUnreadCountResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetUnreadCountResponse()..mergeFromBuffer(data, registry);
  factory GetUnreadCountResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetUnreadCountResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetUnreadCountResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.notification.v1alpha1'),
      createEmptyInstance: GetUnreadCountResponse.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'unreadCount')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetUnreadCountResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetUnreadCountResponse copyWith(
          void Function(GetUnreadCountResponse) updates) =>
      super.copyWith((message) => updates(message as GetUnreadCountResponse))
          as GetUnreadCountResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetUnreadCountResponse() / GetUnreadCountResponse.new instead')
  static GetUnreadCountResponse create() => GetUnreadCountResponse._();
  static $pb.GeneratedMessage $_createMessage() => GetUnreadCountResponse._();
  @$core.override
  GetUnreadCountResponse createEmptyInstance() => GetUnreadCountResponse._();
  @$core.pragma('dart2js:noInline')
  static GetUnreadCountResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetUnreadCountResponse>(
          GetUnreadCountResponse.$_createMessage);
  static GetUnreadCountResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get unreadCount => $_getIZ(0);
  @$pb.TagNumber(1)
  set unreadCount($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasUnreadCount() => $_has(0);
  @$pb.TagNumber(1)
  void clearUnreadCount() => $_clearField(1);
}

class MarkInboxItemReadRequest extends $pb.GeneratedMessage {
  factory MarkInboxItemReadRequest({
    $core.String? id,
  }) {
    final result = MarkInboxItemReadRequest._();
    if (id != null) result.id = id;
    return result;
  }

  MarkInboxItemReadRequest._();

  factory MarkInboxItemReadRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MarkInboxItemReadRequest()..mergeFromBuffer(data, registry);
  factory MarkInboxItemReadRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MarkInboxItemReadRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MarkInboxItemReadRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.notification.v1alpha1'),
      createEmptyInstance: MarkInboxItemReadRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MarkInboxItemReadRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MarkInboxItemReadRequest copyWith(
          void Function(MarkInboxItemReadRequest) updates) =>
      super.copyWith((message) => updates(message as MarkInboxItemReadRequest))
          as MarkInboxItemReadRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use MarkInboxItemReadRequest() / MarkInboxItemReadRequest.new instead')
  static MarkInboxItemReadRequest create() => MarkInboxItemReadRequest._();
  static $pb.GeneratedMessage $_createMessage() => MarkInboxItemReadRequest._();
  @$core.override
  MarkInboxItemReadRequest createEmptyInstance() =>
      MarkInboxItemReadRequest._();
  @$core.pragma('dart2js:noInline')
  static MarkInboxItemReadRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MarkInboxItemReadRequest>(
          MarkInboxItemReadRequest.$_createMessage);
  static MarkInboxItemReadRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);
}

class MarkInboxItemReadResponse extends $pb.GeneratedMessage {
  factory MarkInboxItemReadResponse() => MarkInboxItemReadResponse._();

  MarkInboxItemReadResponse._();

  factory MarkInboxItemReadResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MarkInboxItemReadResponse()..mergeFromBuffer(data, registry);
  factory MarkInboxItemReadResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MarkInboxItemReadResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MarkInboxItemReadResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.notification.v1alpha1'),
      createEmptyInstance: MarkInboxItemReadResponse.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MarkInboxItemReadResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MarkInboxItemReadResponse copyWith(
          void Function(MarkInboxItemReadResponse) updates) =>
      super.copyWith((message) => updates(message as MarkInboxItemReadResponse))
          as MarkInboxItemReadResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use MarkInboxItemReadResponse() / MarkInboxItemReadResponse.new instead')
  static MarkInboxItemReadResponse create() => MarkInboxItemReadResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      MarkInboxItemReadResponse._();
  @$core.override
  MarkInboxItemReadResponse createEmptyInstance() =>
      MarkInboxItemReadResponse._();
  @$core.pragma('dart2js:noInline')
  static MarkInboxItemReadResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MarkInboxItemReadResponse>(
          MarkInboxItemReadResponse.$_createMessage);
  static MarkInboxItemReadResponse? _defaultInstance;
}

class MarkAllInboxItemsReadRequest extends $pb.GeneratedMessage {
  factory MarkAllInboxItemsReadRequest({
    NotificationAudience? audience,
  }) {
    final result = MarkAllInboxItemsReadRequest._();
    if (audience != null) result.audience = audience;
    return result;
  }

  MarkAllInboxItemsReadRequest._();

  factory MarkAllInboxItemsReadRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MarkAllInboxItemsReadRequest()..mergeFromBuffer(data, registry);
  factory MarkAllInboxItemsReadRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MarkAllInboxItemsReadRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MarkAllInboxItemsReadRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.notification.v1alpha1'),
      createEmptyInstance: MarkAllInboxItemsReadRequest.$_createMessage)
    ..aE<NotificationAudience>(1, _omitFieldNames ? '' : 'audience',
        enumValues: NotificationAudience.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MarkAllInboxItemsReadRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MarkAllInboxItemsReadRequest copyWith(
          void Function(MarkAllInboxItemsReadRequest) updates) =>
      super.copyWith(
              (message) => updates(message as MarkAllInboxItemsReadRequest))
          as MarkAllInboxItemsReadRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use MarkAllInboxItemsReadRequest() / MarkAllInboxItemsReadRequest.new instead')
  static MarkAllInboxItemsReadRequest create() =>
      MarkAllInboxItemsReadRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      MarkAllInboxItemsReadRequest._();
  @$core.override
  MarkAllInboxItemsReadRequest createEmptyInstance() =>
      MarkAllInboxItemsReadRequest._();
  @$core.pragma('dart2js:noInline')
  static MarkAllInboxItemsReadRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MarkAllInboxItemsReadRequest>(
          MarkAllInboxItemsReadRequest.$_createMessage);
  static MarkAllInboxItemsReadRequest? _defaultInstance;

  @$pb.TagNumber(1)
  NotificationAudience get audience => $_getN(0);
  @$pb.TagNumber(1)
  set audience(NotificationAudience value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasAudience() => $_has(0);
  @$pb.TagNumber(1)
  void clearAudience() => $_clearField(1);
}

class MarkAllInboxItemsReadResponse extends $pb.GeneratedMessage {
  factory MarkAllInboxItemsReadResponse({
    $core.int? updatedCount,
  }) {
    final result = MarkAllInboxItemsReadResponse._();
    if (updatedCount != null) result.updatedCount = updatedCount;
    return result;
  }

  MarkAllInboxItemsReadResponse._();

  factory MarkAllInboxItemsReadResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MarkAllInboxItemsReadResponse()..mergeFromBuffer(data, registry);
  factory MarkAllInboxItemsReadResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MarkAllInboxItemsReadResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MarkAllInboxItemsReadResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.notification.v1alpha1'),
      createEmptyInstance: MarkAllInboxItemsReadResponse.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'updatedCount')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MarkAllInboxItemsReadResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MarkAllInboxItemsReadResponse copyWith(
          void Function(MarkAllInboxItemsReadResponse) updates) =>
      super.copyWith(
              (message) => updates(message as MarkAllInboxItemsReadResponse))
          as MarkAllInboxItemsReadResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use MarkAllInboxItemsReadResponse() / MarkAllInboxItemsReadResponse.new instead')
  static MarkAllInboxItemsReadResponse create() =>
      MarkAllInboxItemsReadResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      MarkAllInboxItemsReadResponse._();
  @$core.override
  MarkAllInboxItemsReadResponse createEmptyInstance() =>
      MarkAllInboxItemsReadResponse._();
  @$core.pragma('dart2js:noInline')
  static MarkAllInboxItemsReadResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MarkAllInboxItemsReadResponse>(
          MarkAllInboxItemsReadResponse.$_createMessage);
  static MarkAllInboxItemsReadResponse? _defaultInstance;

  /// 這次改成已讀的筆數。
  @$pb.TagNumber(1)
  $core.int get updatedCount => $_getIZ(0);
  @$pb.TagNumber(1)
  set updatedCount($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasUpdatedCount() => $_has(0);
  @$pb.TagNumber(1)
  void clearUpdatedCount() => $_clearField(1);
}

class BatchDeleteInboxItemsRequest extends $pb.GeneratedMessage {
  factory BatchDeleteInboxItemsRequest({
    $core.Iterable<$core.String>? ids,
  }) {
    final result = BatchDeleteInboxItemsRequest._();
    if (ids != null) result.ids.addAll(ids);
    return result;
  }

  BatchDeleteInboxItemsRequest._();

  factory BatchDeleteInboxItemsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BatchDeleteInboxItemsRequest()..mergeFromBuffer(data, registry);
  factory BatchDeleteInboxItemsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BatchDeleteInboxItemsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BatchDeleteInboxItemsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.notification.v1alpha1'),
      createEmptyInstance: BatchDeleteInboxItemsRequest.$_createMessage)
    ..pPS(1, _omitFieldNames ? '' : 'ids')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BatchDeleteInboxItemsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BatchDeleteInboxItemsRequest copyWith(
          void Function(BatchDeleteInboxItemsRequest) updates) =>
      super.copyWith(
              (message) => updates(message as BatchDeleteInboxItemsRequest))
          as BatchDeleteInboxItemsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use BatchDeleteInboxItemsRequest() / BatchDeleteInboxItemsRequest.new instead')
  static BatchDeleteInboxItemsRequest create() =>
      BatchDeleteInboxItemsRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      BatchDeleteInboxItemsRequest._();
  @$core.override
  BatchDeleteInboxItemsRequest createEmptyInstance() =>
      BatchDeleteInboxItemsRequest._();
  @$core.pragma('dart2js:noInline')
  static BatchDeleteInboxItemsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BatchDeleteInboxItemsRequest>(
          BatchDeleteInboxItemsRequest.$_createMessage);
  static BatchDeleteInboxItemsRequest? _defaultInstance;

  /// 1 到 100 個，不可重複；空、重複或超過 100 個回 invalid_argument。
  @$pb.TagNumber(1)
  $pb.PbList<$core.String> get ids => $_getList(0);
}

class BatchDeleteInboxItemsResponse extends $pb.GeneratedMessage {
  factory BatchDeleteInboxItemsResponse() => BatchDeleteInboxItemsResponse._();

  BatchDeleteInboxItemsResponse._();

  factory BatchDeleteInboxItemsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BatchDeleteInboxItemsResponse()..mergeFromBuffer(data, registry);
  factory BatchDeleteInboxItemsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BatchDeleteInboxItemsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BatchDeleteInboxItemsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.notification.v1alpha1'),
      createEmptyInstance: BatchDeleteInboxItemsResponse.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BatchDeleteInboxItemsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BatchDeleteInboxItemsResponse copyWith(
          void Function(BatchDeleteInboxItemsResponse) updates) =>
      super.copyWith(
              (message) => updates(message as BatchDeleteInboxItemsResponse))
          as BatchDeleteInboxItemsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use BatchDeleteInboxItemsResponse() / BatchDeleteInboxItemsResponse.new instead')
  static BatchDeleteInboxItemsResponse create() =>
      BatchDeleteInboxItemsResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      BatchDeleteInboxItemsResponse._();
  @$core.override
  BatchDeleteInboxItemsResponse createEmptyInstance() =>
      BatchDeleteInboxItemsResponse._();
  @$core.pragma('dart2js:noInline')
  static BatchDeleteInboxItemsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BatchDeleteInboxItemsResponse>(
          BatchDeleteInboxItemsResponse.$_createMessage);
  static BatchDeleteInboxItemsResponse? _defaultInstance;
}

/// PreferenceEntry 是一個 (分類, 渠道) 的偏好。
class PreferenceEntry extends $pb.GeneratedMessage {
  factory PreferenceEntry({
    NotificationCategory? category,
    NotificationChannel? channel,
    $core.bool? enabled,
    $core.bool? canDisable,
  }) {
    final result = PreferenceEntry._();
    if (category != null) result.category = category;
    if (channel != null) result.channel = channel;
    if (enabled != null) result.enabled = enabled;
    if (canDisable != null) result.canDisable = canDisable;
    return result;
  }

  PreferenceEntry._();

  factory PreferenceEntry.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PreferenceEntry()..mergeFromBuffer(data, registry);
  factory PreferenceEntry.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PreferenceEntry()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PreferenceEntry',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.notification.v1alpha1'),
      createEmptyInstance: PreferenceEntry.$_createMessage)
    ..aE<NotificationCategory>(1, _omitFieldNames ? '' : 'category',
        enumValues: NotificationCategory.values)
    ..aE<NotificationChannel>(2, _omitFieldNames ? '' : 'channel',
        enumValues: NotificationChannel.values)
    ..aOB(3, _omitFieldNames ? '' : 'enabled')
    ..aOB(4, _omitFieldNames ? '' : 'canDisable')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PreferenceEntry clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PreferenceEntry copyWith(void Function(PreferenceEntry) updates) =>
      super.copyWith((message) => updates(message as PreferenceEntry))
          as PreferenceEntry;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use PreferenceEntry() / PreferenceEntry.new instead')
  static PreferenceEntry create() => PreferenceEntry._();
  static $pb.GeneratedMessage $_createMessage() => PreferenceEntry._();
  @$core.override
  PreferenceEntry createEmptyInstance() => PreferenceEntry._();
  @$core.pragma('dart2js:noInline')
  static PreferenceEntry getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<PreferenceEntry>(
          PreferenceEntry.$_createMessage);
  static PreferenceEntry? _defaultInstance;

  @$pb.TagNumber(1)
  NotificationCategory get category => $_getN(0);
  @$pb.TagNumber(1)
  set category(NotificationCategory value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasCategory() => $_has(0);
  @$pb.TagNumber(1)
  void clearCategory() => $_clearField(1);

  @$pb.TagNumber(2)
  NotificationChannel get channel => $_getN(1);
  @$pb.TagNumber(2)
  set channel(NotificationChannel value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasChannel() => $_has(1);
  @$pb.TagNumber(2)
  void clearChannel() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.bool get enabled => $_getBF(2);
  @$pb.TagNumber(3)
  set enabled($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasEnabled() => $_has(2);
  @$pb.TagNumber(3)
  void clearEnabled() => $_clearField(3);

  /// false 表示這個 (分類, 渠道) 不能關閉，enabled 一定是 true；client 把開關設為不可操作。
  @$pb.TagNumber(4)
  $core.bool get canDisable => $_getBF(3);
  @$pb.TagNumber(4)
  set canDisable($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(4)
  $core.bool hasCanDisable() => $_has(3);
  @$pb.TagNumber(4)
  void clearCanDisable() => $_clearField(4);
}

class GetNotificationPreferencesRequest extends $pb.GeneratedMessage {
  factory GetNotificationPreferencesRequest({
    NotificationAudience? audience,
  }) {
    final result = GetNotificationPreferencesRequest._();
    if (audience != null) result.audience = audience;
    return result;
  }

  GetNotificationPreferencesRequest._();

  factory GetNotificationPreferencesRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetNotificationPreferencesRequest()..mergeFromBuffer(data, registry);
  factory GetNotificationPreferencesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetNotificationPreferencesRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetNotificationPreferencesRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.notification.v1alpha1'),
      createEmptyInstance: GetNotificationPreferencesRequest.$_createMessage)
    ..aE<NotificationAudience>(1, _omitFieldNames ? '' : 'audience',
        enumValues: NotificationAudience.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetNotificationPreferencesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetNotificationPreferencesRequest copyWith(
          void Function(GetNotificationPreferencesRequest) updates) =>
      super.copyWith((message) =>
              updates(message as GetNotificationPreferencesRequest))
          as GetNotificationPreferencesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetNotificationPreferencesRequest() / GetNotificationPreferencesRequest.new instead')
  static GetNotificationPreferencesRequest create() =>
      GetNotificationPreferencesRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      GetNotificationPreferencesRequest._();
  @$core.override
  GetNotificationPreferencesRequest createEmptyInstance() =>
      GetNotificationPreferencesRequest._();
  @$core.pragma('dart2js:noInline')
  static GetNotificationPreferencesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetNotificationPreferencesRequest>(
          GetNotificationPreferencesRequest.$_createMessage);
  static GetNotificationPreferencesRequest? _defaultInstance;

  @$pb.TagNumber(1)
  NotificationAudience get audience => $_getN(0);
  @$pb.TagNumber(1)
  set audience(NotificationAudience value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasAudience() => $_has(0);
  @$pb.TagNumber(1)
  void clearAudience() => $_clearField(1);
}

class GetNotificationPreferencesResponse extends $pb.GeneratedMessage {
  factory GetNotificationPreferencesResponse({
    $core.Iterable<PreferenceEntry>? entries,
  }) {
    final result = GetNotificationPreferencesResponse._();
    if (entries != null) result.entries.addAll(entries);
    return result;
  }

  GetNotificationPreferencesResponse._();

  factory GetNotificationPreferencesResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetNotificationPreferencesResponse()..mergeFromBuffer(data, registry);
  factory GetNotificationPreferencesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetNotificationPreferencesResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetNotificationPreferencesResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.notification.v1alpha1'),
      createEmptyInstance: GetNotificationPreferencesResponse.$_createMessage)
    ..pPM<PreferenceEntry>(1, _omitFieldNames ? '' : 'entries',
        subBuilder: PreferenceEntry.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetNotificationPreferencesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetNotificationPreferencesResponse copyWith(
          void Function(GetNotificationPreferencesResponse) updates) =>
      super.copyWith((message) =>
              updates(message as GetNotificationPreferencesResponse))
          as GetNotificationPreferencesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetNotificationPreferencesResponse() / GetNotificationPreferencesResponse.new instead')
  static GetNotificationPreferencesResponse create() =>
      GetNotificationPreferencesResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      GetNotificationPreferencesResponse._();
  @$core.override
  GetNotificationPreferencesResponse createEmptyInstance() =>
      GetNotificationPreferencesResponse._();
  @$core.pragma('dart2js:noInline')
  static GetNotificationPreferencesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetNotificationPreferencesResponse>(
          GetNotificationPreferencesResponse.$_createMessage);
  static GetNotificationPreferencesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<PreferenceEntry> get entries => $_getList(0);
}

class UpdateNotificationPreferenceRequest extends $pb.GeneratedMessage {
  factory UpdateNotificationPreferenceRequest({
    NotificationAudience? audience,
    NotificationCategory? category,
    NotificationChannel? channel,
    $core.bool? enabled,
  }) {
    final result = UpdateNotificationPreferenceRequest._();
    if (audience != null) result.audience = audience;
    if (category != null) result.category = category;
    if (channel != null) result.channel = channel;
    if (enabled != null) result.enabled = enabled;
    return result;
  }

  UpdateNotificationPreferenceRequest._();

  factory UpdateNotificationPreferenceRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateNotificationPreferenceRequest()..mergeFromBuffer(data, registry);
  factory UpdateNotificationPreferenceRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateNotificationPreferenceRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateNotificationPreferenceRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.notification.v1alpha1'),
      createEmptyInstance: UpdateNotificationPreferenceRequest.$_createMessage)
    ..aE<NotificationAudience>(1, _omitFieldNames ? '' : 'audience',
        enumValues: NotificationAudience.values)
    ..aE<NotificationCategory>(2, _omitFieldNames ? '' : 'category',
        enumValues: NotificationCategory.values)
    ..aE<NotificationChannel>(3, _omitFieldNames ? '' : 'channel',
        enumValues: NotificationChannel.values)
    ..aOB(4, _omitFieldNames ? '' : 'enabled')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateNotificationPreferenceRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateNotificationPreferenceRequest copyWith(
          void Function(UpdateNotificationPreferenceRequest) updates) =>
      super.copyWith((message) =>
              updates(message as UpdateNotificationPreferenceRequest))
          as UpdateNotificationPreferenceRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use UpdateNotificationPreferenceRequest() / UpdateNotificationPreferenceRequest.new instead')
  static UpdateNotificationPreferenceRequest create() =>
      UpdateNotificationPreferenceRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      UpdateNotificationPreferenceRequest._();
  @$core.override
  UpdateNotificationPreferenceRequest createEmptyInstance() =>
      UpdateNotificationPreferenceRequest._();
  @$core.pragma('dart2js:noInline')
  static UpdateNotificationPreferenceRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
              UpdateNotificationPreferenceRequest>(
          UpdateNotificationPreferenceRequest.$_createMessage);
  static UpdateNotificationPreferenceRequest? _defaultInstance;

  @$pb.TagNumber(1)
  NotificationAudience get audience => $_getN(0);
  @$pb.TagNumber(1)
  set audience(NotificationAudience value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasAudience() => $_has(0);
  @$pb.TagNumber(1)
  void clearAudience() => $_clearField(1);

  @$pb.TagNumber(2)
  NotificationCategory get category => $_getN(1);
  @$pb.TagNumber(2)
  set category(NotificationCategory value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasCategory() => $_has(1);
  @$pb.TagNumber(2)
  void clearCategory() => $_clearField(2);

  @$pb.TagNumber(3)
  NotificationChannel get channel => $_getN(2);
  @$pb.TagNumber(3)
  set channel(NotificationChannel value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasChannel() => $_has(2);
  @$pb.TagNumber(3)
  void clearChannel() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.bool get enabled => $_getBF(3);
  @$pb.TagNumber(4)
  set enabled($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(4)
  $core.bool hasEnabled() => $_has(3);
  @$pb.TagNumber(4)
  void clearEnabled() => $_clearField(4);
}

class UpdateNotificationPreferenceResponse extends $pb.GeneratedMessage {
  factory UpdateNotificationPreferenceResponse({
    PreferenceEntry? entry,
  }) {
    final result = UpdateNotificationPreferenceResponse._();
    if (entry != null) result.entry = entry;
    return result;
  }

  UpdateNotificationPreferenceResponse._();

  factory UpdateNotificationPreferenceResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateNotificationPreferenceResponse()..mergeFromBuffer(data, registry);
  factory UpdateNotificationPreferenceResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateNotificationPreferenceResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateNotificationPreferenceResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.notification.v1alpha1'),
      createEmptyInstance: UpdateNotificationPreferenceResponse.$_createMessage)
    ..aOM<PreferenceEntry>(1, _omitFieldNames ? '' : 'entry',
        subBuilder: PreferenceEntry.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateNotificationPreferenceResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateNotificationPreferenceResponse copyWith(
          void Function(UpdateNotificationPreferenceResponse) updates) =>
      super.copyWith((message) =>
              updates(message as UpdateNotificationPreferenceResponse))
          as UpdateNotificationPreferenceResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use UpdateNotificationPreferenceResponse() / UpdateNotificationPreferenceResponse.new instead')
  static UpdateNotificationPreferenceResponse create() =>
      UpdateNotificationPreferenceResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      UpdateNotificationPreferenceResponse._();
  @$core.override
  UpdateNotificationPreferenceResponse createEmptyInstance() =>
      UpdateNotificationPreferenceResponse._();
  @$core.pragma('dart2js:noInline')
  static UpdateNotificationPreferenceResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
              UpdateNotificationPreferenceResponse>(
          UpdateNotificationPreferenceResponse.$_createMessage);
  static UpdateNotificationPreferenceResponse? _defaultInstance;

  @$pb.TagNumber(1)
  PreferenceEntry get entry => $_getN(0);
  @$pb.TagNumber(1)
  set entry(PreferenceEntry value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasEntry() => $_has(0);
  @$pb.TagNumber(1)
  void clearEntry() => $_clearField(1);
  @$pb.TagNumber(1)
  PreferenceEntry ensureEntry() => $_ensure(0);
}

class RegisterDeviceTokenRequest extends $pb.GeneratedMessage {
  factory RegisterDeviceTokenRequest({
    $core.String? token,
    NotificationApp? app,
  }) {
    final result = RegisterDeviceTokenRequest._();
    if (token != null) result.token = token;
    if (app != null) result.app = app;
    return result;
  }

  RegisterDeviceTokenRequest._();

  factory RegisterDeviceTokenRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RegisterDeviceTokenRequest()..mergeFromBuffer(data, registry);
  factory RegisterDeviceTokenRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RegisterDeviceTokenRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RegisterDeviceTokenRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.notification.v1alpha1'),
      createEmptyInstance: RegisterDeviceTokenRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'token')
    ..aE<NotificationApp>(2, _omitFieldNames ? '' : 'app',
        enumValues: NotificationApp.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegisterDeviceTokenRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegisterDeviceTokenRequest copyWith(
          void Function(RegisterDeviceTokenRequest) updates) =>
      super.copyWith(
              (message) => updates(message as RegisterDeviceTokenRequest))
          as RegisterDeviceTokenRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use RegisterDeviceTokenRequest() / RegisterDeviceTokenRequest.new instead')
  static RegisterDeviceTokenRequest create() => RegisterDeviceTokenRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      RegisterDeviceTokenRequest._();
  @$core.override
  RegisterDeviceTokenRequest createEmptyInstance() =>
      RegisterDeviceTokenRequest._();
  @$core.pragma('dart2js:noInline')
  static RegisterDeviceTokenRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RegisterDeviceTokenRequest>(
          RegisterDeviceTokenRequest.$_createMessage);
  static RegisterDeviceTokenRequest? _defaultInstance;

  /// 推播服務給的裝置 token。
  @$pb.TagNumber(1)
  $core.String get token => $_getSZ(0);
  @$pb.TagNumber(1)
  set token($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasToken() => $_has(0);
  @$pb.TagNumber(1)
  void clearToken() => $_clearField(1);

  @$pb.TagNumber(2)
  NotificationApp get app => $_getN(1);
  @$pb.TagNumber(2)
  set app(NotificationApp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasApp() => $_has(1);
  @$pb.TagNumber(2)
  void clearApp() => $_clearField(2);
}

class RegisterDeviceTokenResponse extends $pb.GeneratedMessage {
  factory RegisterDeviceTokenResponse() => RegisterDeviceTokenResponse._();

  RegisterDeviceTokenResponse._();

  factory RegisterDeviceTokenResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RegisterDeviceTokenResponse()..mergeFromBuffer(data, registry);
  factory RegisterDeviceTokenResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RegisterDeviceTokenResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RegisterDeviceTokenResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.notification.v1alpha1'),
      createEmptyInstance: RegisterDeviceTokenResponse.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegisterDeviceTokenResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegisterDeviceTokenResponse copyWith(
          void Function(RegisterDeviceTokenResponse) updates) =>
      super.copyWith(
              (message) => updates(message as RegisterDeviceTokenResponse))
          as RegisterDeviceTokenResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use RegisterDeviceTokenResponse() / RegisterDeviceTokenResponse.new instead')
  static RegisterDeviceTokenResponse create() =>
      RegisterDeviceTokenResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      RegisterDeviceTokenResponse._();
  @$core.override
  RegisterDeviceTokenResponse createEmptyInstance() =>
      RegisterDeviceTokenResponse._();
  @$core.pragma('dart2js:noInline')
  static RegisterDeviceTokenResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RegisterDeviceTokenResponse>(
          RegisterDeviceTokenResponse.$_createMessage);
  static RegisterDeviceTokenResponse? _defaultInstance;
}

class DeleteDeviceTokenRequest extends $pb.GeneratedMessage {
  factory DeleteDeviceTokenRequest({
    $core.String? token,
  }) {
    final result = DeleteDeviceTokenRequest._();
    if (token != null) result.token = token;
    return result;
  }

  DeleteDeviceTokenRequest._();

  factory DeleteDeviceTokenRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteDeviceTokenRequest()..mergeFromBuffer(data, registry);
  factory DeleteDeviceTokenRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteDeviceTokenRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteDeviceTokenRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.notification.v1alpha1'),
      createEmptyInstance: DeleteDeviceTokenRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'token')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteDeviceTokenRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteDeviceTokenRequest copyWith(
          void Function(DeleteDeviceTokenRequest) updates) =>
      super.copyWith((message) => updates(message as DeleteDeviceTokenRequest))
          as DeleteDeviceTokenRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use DeleteDeviceTokenRequest() / DeleteDeviceTokenRequest.new instead')
  static DeleteDeviceTokenRequest create() => DeleteDeviceTokenRequest._();
  static $pb.GeneratedMessage $_createMessage() => DeleteDeviceTokenRequest._();
  @$core.override
  DeleteDeviceTokenRequest createEmptyInstance() =>
      DeleteDeviceTokenRequest._();
  @$core.pragma('dart2js:noInline')
  static DeleteDeviceTokenRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteDeviceTokenRequest>(
          DeleteDeviceTokenRequest.$_createMessage);
  static DeleteDeviceTokenRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get token => $_getSZ(0);
  @$pb.TagNumber(1)
  set token($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasToken() => $_has(0);
  @$pb.TagNumber(1)
  void clearToken() => $_clearField(1);
}

class DeleteDeviceTokenResponse extends $pb.GeneratedMessage {
  factory DeleteDeviceTokenResponse() => DeleteDeviceTokenResponse._();

  DeleteDeviceTokenResponse._();

  factory DeleteDeviceTokenResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteDeviceTokenResponse()..mergeFromBuffer(data, registry);
  factory DeleteDeviceTokenResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteDeviceTokenResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteDeviceTokenResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.notification.v1alpha1'),
      createEmptyInstance: DeleteDeviceTokenResponse.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteDeviceTokenResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteDeviceTokenResponse copyWith(
          void Function(DeleteDeviceTokenResponse) updates) =>
      super.copyWith((message) => updates(message as DeleteDeviceTokenResponse))
          as DeleteDeviceTokenResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use DeleteDeviceTokenResponse() / DeleteDeviceTokenResponse.new instead')
  static DeleteDeviceTokenResponse create() => DeleteDeviceTokenResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      DeleteDeviceTokenResponse._();
  @$core.override
  DeleteDeviceTokenResponse createEmptyInstance() =>
      DeleteDeviceTokenResponse._();
  @$core.pragma('dart2js:noInline')
  static DeleteDeviceTokenResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteDeviceTokenResponse>(
          DeleteDeviceTokenResponse.$_createMessage);
  static DeleteDeviceTokenResponse? _defaultInstance;
}

class WatchNotificationsRequest extends $pb.GeneratedMessage {
  factory WatchNotificationsRequest({
    NotificationAudience? audience,
  }) {
    final result = WatchNotificationsRequest._();
    if (audience != null) result.audience = audience;
    return result;
  }

  WatchNotificationsRequest._();

  factory WatchNotificationsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      WatchNotificationsRequest()..mergeFromBuffer(data, registry);
  factory WatchNotificationsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      WatchNotificationsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'WatchNotificationsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.notification.v1alpha1'),
      createEmptyInstance: WatchNotificationsRequest.$_createMessage)
    ..aE<NotificationAudience>(1, _omitFieldNames ? '' : 'audience',
        enumValues: NotificationAudience.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  WatchNotificationsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  WatchNotificationsRequest copyWith(
          void Function(WatchNotificationsRequest) updates) =>
      super.copyWith((message) => updates(message as WatchNotificationsRequest))
          as WatchNotificationsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use WatchNotificationsRequest() / WatchNotificationsRequest.new instead')
  static WatchNotificationsRequest create() => WatchNotificationsRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      WatchNotificationsRequest._();
  @$core.override
  WatchNotificationsRequest createEmptyInstance() =>
      WatchNotificationsRequest._();
  @$core.pragma('dart2js:noInline')
  static WatchNotificationsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<WatchNotificationsRequest>(
          WatchNotificationsRequest.$_createMessage);
  static WatchNotificationsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  NotificationAudience get audience => $_getN(0);
  @$pb.TagNumber(1)
  set audience(NotificationAudience value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasAudience() => $_has(0);
  @$pb.TagNumber(1)
  void clearAudience() => $_clearField(1);
}

enum WatchNotificationsResponse_Event { inbox, sellerWorkspace, notSet }

/// WatchNotificationsResponse 是串流的一則訊號。
class WatchNotificationsResponse extends $pb.GeneratedMessage {
  factory WatchNotificationsResponse({
    InboxSignal? inbox,
    SellerWorkspaceSignal? sellerWorkspace,
  }) {
    final result = WatchNotificationsResponse._();
    if (inbox != null) result.inbox = inbox;
    if (sellerWorkspace != null) result.sellerWorkspace = sellerWorkspace;
    return result;
  }

  WatchNotificationsResponse._();

  factory WatchNotificationsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      WatchNotificationsResponse()..mergeFromBuffer(data, registry);
  factory WatchNotificationsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      WatchNotificationsResponse()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, WatchNotificationsResponse_Event>
      _WatchNotificationsResponse_EventByTag = {
    1: WatchNotificationsResponse_Event.inbox,
    2: WatchNotificationsResponse_Event.sellerWorkspace,
    0: WatchNotificationsResponse_Event.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'WatchNotificationsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.notification.v1alpha1'),
      createEmptyInstance: WatchNotificationsResponse.$_createMessage)
    ..oo(0, [1, 2])
    ..aOM<InboxSignal>(1, _omitFieldNames ? '' : 'inbox',
        subBuilder: InboxSignal.$_createMessage)
    ..aOM<SellerWorkspaceSignal>(2, _omitFieldNames ? '' : 'sellerWorkspace',
        subBuilder: SellerWorkspaceSignal.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  WatchNotificationsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  WatchNotificationsResponse copyWith(
          void Function(WatchNotificationsResponse) updates) =>
      super.copyWith(
              (message) => updates(message as WatchNotificationsResponse))
          as WatchNotificationsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use WatchNotificationsResponse() / WatchNotificationsResponse.new instead')
  static WatchNotificationsResponse create() => WatchNotificationsResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      WatchNotificationsResponse._();
  @$core.override
  WatchNotificationsResponse createEmptyInstance() =>
      WatchNotificationsResponse._();
  @$core.pragma('dart2js:noInline')
  static WatchNotificationsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<WatchNotificationsResponse>(
          WatchNotificationsResponse.$_createMessage);
  static WatchNotificationsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  WatchNotificationsResponse_Event whichEvent() =>
      _WatchNotificationsResponse_EventByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  void clearEvent() => $_clearField($_whichOneof(0));

  @$pb.TagNumber(1)
  InboxSignal get inbox => $_getN(0);
  @$pb.TagNumber(1)
  set inbox(InboxSignal value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasInbox() => $_has(0);
  @$pb.TagNumber(1)
  void clearInbox() => $_clearField(1);
  @$pb.TagNumber(1)
  InboxSignal ensureInbox() => $_ensure(0);

  /// 只在 STORE 受眾的串流出現。
  @$pb.TagNumber(2)
  SellerWorkspaceSignal get sellerWorkspace => $_getN(1);
  @$pb.TagNumber(2)
  set sellerWorkspace(SellerWorkspaceSignal value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasSellerWorkspace() => $_has(1);
  @$pb.TagNumber(2)
  void clearSellerWorkspace() => $_clearField(2);
  @$pb.TagNumber(2)
  SellerWorkspaceSignal ensureSellerWorkspace() => $_ensure(1);
}

/// InboxSignal 是收件匣有變化的訊號：目前的未讀數與最新一則通知的 ID，client 收到後重讀列表。
class InboxSignal extends $pb.GeneratedMessage {
  factory InboxSignal({
    $core.int? unreadCount,
    $core.String? latestInboxItemId,
    $core.Iterable<StateRefreshHint>? stateRefreshHints,
    $core.Iterable<$core.String>? newInboxItemIds,
    $core.bool? newItemsTruncated,
  }) {
    final result = InboxSignal._();
    if (unreadCount != null) result.unreadCount = unreadCount;
    if (latestInboxItemId != null) result.latestInboxItemId = latestInboxItemId;
    if (stateRefreshHints != null)
      result.stateRefreshHints.addAll(stateRefreshHints);
    if (newInboxItemIds != null) result.newInboxItemIds.addAll(newInboxItemIds);
    if (newItemsTruncated != null) result.newItemsTruncated = newItemsTruncated;
    return result;
  }

  InboxSignal._();

  factory InboxSignal.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      InboxSignal()..mergeFromBuffer(data, registry);
  factory InboxSignal.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      InboxSignal()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'InboxSignal',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.notification.v1alpha1'),
      createEmptyInstance: InboxSignal.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'unreadCount')
    ..aOS(2, _omitFieldNames ? '' : 'latestInboxItemId')
    ..pc<StateRefreshHint>(
        3, _omitFieldNames ? '' : 'stateRefreshHints', $pb.PbFieldType.KE,
        valueOf: StateRefreshHint.valueOf,
        enumValues: StateRefreshHint.values,
        defaultEnumValue: StateRefreshHint.STATE_REFRESH_HINT_UNSPECIFIED)
    ..pPS(4, _omitFieldNames ? '' : 'newInboxItemIds')
    ..aOB(5, _omitFieldNames ? '' : 'newItemsTruncated')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  InboxSignal clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  InboxSignal copyWith(void Function(InboxSignal) updates) =>
      super.copyWith((message) => updates(message as InboxSignal))
          as InboxSignal;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use InboxSignal() / InboxSignal.new instead')
  static InboxSignal create() => InboxSignal._();
  static $pb.GeneratedMessage $_createMessage() => InboxSignal._();
  @$core.override
  InboxSignal createEmptyInstance() => InboxSignal._();
  @$core.pragma('dart2js:noInline')
  static InboxSignal getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<InboxSignal>(
          InboxSignal.$_createMessage);
  static InboxSignal? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get unreadCount => $_getIZ(0);
  @$pb.TagNumber(1)
  set unreadCount($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasUnreadCount() => $_has(0);
  @$pb.TagNumber(1)
  void clearUnreadCount() => $_clearField(1);

  /// 最新一則通知的 ID；收件匣是空的時不設。
  @$pb.TagNumber(2)
  $core.String get latestInboxItemId => $_getSZ(1);
  @$pb.TagNumber(2)
  set latestInboxItemId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasLatestInboxItemId() => $_has(1);
  @$pb.TagNumber(2)
  void clearLatestInboxItemId() => $_clearField(2);

  /// 引起這次訊號的通知所帶的狀態重讀提示。訊號可能把多則通知合併，所以是這些通知的提示的聯集（不重複）；
  /// 空表示不需要重讀（連線建立時的第一則也是）。不會含 STATE_REFRESH_HINT_UNSPECIFIED。
  @$pb.TagNumber(3)
  $pb.PbList<StateRefreshHint> get stateRefreshHints => $_getList(2);

  /// 上一則訊號之後在這條串流上新建的通知、或被聚合更新（合併進同一則未讀通知）的收件匣項目 ID，依到達順序、不重複；
  /// client 只對這些 ID 做 toast，不比對列表。聚合更新會讓同一個 ID 再出現，client 以 ID 去重並更新既有的 toast。
  /// 連線建立與重連的第一則、只有已讀或刪除變化時為空。每個 ID 在一條串流上至多送達一次：斷線期間的缺口不補送，收件匣與未讀數仍以列表為準。
  @$pb.TagNumber(4)
  $pb.PbList<$core.String> get newInboxItemIds => $_getList(3);

  /// 為 true 表示累積的新通知超過上限（20 則）：new_inbox_item_ids 為空，client 重讀列表、不 toast。
  @$pb.TagNumber(5)
  $core.bool get newItemsTruncated => $_getBF(4);
  @$pb.TagNumber(5)
  set newItemsTruncated($core.bool value) => $_setBool(4, value);
  @$pb.TagNumber(5)
  $core.bool hasNewItemsTruncated() => $_has(4);
  @$pb.TagNumber(5)
  void clearNewItemsTruncated() => $_clearField(5);
}

/// SellerWorkspaceSignal 是賣家工作區的資料有變化的訊號：client 重讀對應的列表與統計，不帶變化的內容。
class SellerWorkspaceSignal extends $pb.GeneratedMessage {
  factory SellerWorkspaceSignal({
    SellerWorkspaceChange? change,
  }) {
    final result = SellerWorkspaceSignal._();
    if (change != null) result.change = change;
    return result;
  }

  SellerWorkspaceSignal._();

  factory SellerWorkspaceSignal.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SellerWorkspaceSignal()..mergeFromBuffer(data, registry);
  factory SellerWorkspaceSignal.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SellerWorkspaceSignal()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SellerWorkspaceSignal',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.notification.v1alpha1'),
      createEmptyInstance: SellerWorkspaceSignal.$_createMessage)
    ..aE<SellerWorkspaceChange>(1, _omitFieldNames ? '' : 'change',
        enumValues: SellerWorkspaceChange.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SellerWorkspaceSignal clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SellerWorkspaceSignal copyWith(
          void Function(SellerWorkspaceSignal) updates) =>
      super.copyWith((message) => updates(message as SellerWorkspaceSignal))
          as SellerWorkspaceSignal;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SellerWorkspaceSignal() / SellerWorkspaceSignal.new instead')
  static SellerWorkspaceSignal create() => SellerWorkspaceSignal._();
  static $pb.GeneratedMessage $_createMessage() => SellerWorkspaceSignal._();
  @$core.override
  SellerWorkspaceSignal createEmptyInstance() => SellerWorkspaceSignal._();
  @$core.pragma('dart2js:noInline')
  static SellerWorkspaceSignal getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SellerWorkspaceSignal>(
          SellerWorkspaceSignal.$_createMessage);
  static SellerWorkspaceSignal? _defaultInstance;

  @$pb.TagNumber(1)
  SellerWorkspaceChange get change => $_getN(0);
  @$pb.TagNumber(1)
  set change(SellerWorkspaceChange value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasChange() => $_has(0);
  @$pb.TagNumber(1)
  void clearChange() => $_clearField(1);
}

/// NotificationAppService 是登入的帳號讀寫自己的通知。沒有登入回 unauthenticated；管理員（含代行）回 permission_denied。
class NotificationAppServiceApi {
  final $pb.RpcClient _client;

  NotificationAppServiceApi(this._client);

  /// ListInboxItems 列出呼叫者在指定受眾的收件匣，新到舊（建立時間，再依 id 新到舊）。聚合過的通知（例如多人按讚）是一列，
  /// 時間是最後一次合併的時間。page_token 不透明，換了 audience 或 category 就不能沿用。
  /// total_size 是同一個篩選下的全部筆數，不是整個收件匣。
  $async.Future<ListInboxItemsResponse> listInboxItems(
          $pb.ClientContext? ctx, ListInboxItemsRequest request) =>
      _client.invoke<ListInboxItemsResponse>(ctx, 'NotificationAppService',
          'ListInboxItems', request, ListInboxItemsResponse());

  /// GetUnreadCount 回傳指定受眾的未讀數（不分分類）。
  $async.Future<GetUnreadCountResponse> getUnreadCount(
          $pb.ClientContext? ctx, GetUnreadCountRequest request) =>
      _client.invoke<GetUnreadCountResponse>(ctx, 'NotificationAppService',
          'GetUnreadCount', request, GetUnreadCountResponse());

  /// MarkInboxItemRead 把一則通知標成已讀；已經是已讀時成功，不改已讀時間。通知不存在或不是呼叫者的回 not_found，兩者的回應完全相同。
  $async.Future<MarkInboxItemReadResponse> markInboxItemRead(
          $pb.ClientContext? ctx, MarkInboxItemReadRequest request) =>
      _client.invoke<MarkInboxItemReadResponse>(ctx, 'NotificationAppService',
          'MarkInboxItemRead', request, MarkInboxItemReadResponse());

  /// MarkAllInboxItemsRead 把指定受眾的全部未讀通知標成已讀，回傳這次改成已讀的筆數。
  $async.Future<MarkAllInboxItemsReadResponse> markAllInboxItemsRead(
          $pb.ClientContext? ctx, MarkAllInboxItemsReadRequest request) =>
      _client.invoke<MarkAllInboxItemsReadResponse>(
          ctx,
          'NotificationAppService',
          'MarkAllInboxItemsRead',
          request,
          MarkAllInboxItemsReadResponse());

  /// BatchDeleteInboxItems 刪除通知。ids 為空、重複或超過 100 個回 invalid_argument。ids 中任何一個不存在或不是呼叫者的，
  /// 整個請求回 not_found、一則都不刪（兩種情況的回應相同）。
  $async.Future<BatchDeleteInboxItemsResponse> batchDeleteInboxItems(
          $pb.ClientContext? ctx, BatchDeleteInboxItemsRequest request) =>
      _client.invoke<BatchDeleteInboxItemsResponse>(
          ctx,
          'NotificationAppService',
          'BatchDeleteInboxItems',
          request,
          BatchDeleteInboxItemsResponse());

  /// GetNotificationPreferences 列出指定受眾的開關。entries 只包含註冊表實際會在該 (受眾, 分類, 渠道) 送達的組合，
  /// 不是分類與渠道的笛卡兒積（例如沒有型別走 email 的分類就沒有 email 那一項）；沒有設定過的是預設開。
  $async.Future<GetNotificationPreferencesResponse> getNotificationPreferences(
          $pb.ClientContext? ctx, GetNotificationPreferencesRequest request) =>
      _client.invoke<GetNotificationPreferencesResponse>(
          ctx,
          'NotificationAppService',
          'GetNotificationPreferences',
          request,
          GetNotificationPreferencesResponse());

  /// UpdateNotificationPreference 設定一個 (分類, 渠道) 的開關，回傳更新後的那一項。分類不屬於該受眾、該 (分類, 渠道) 不在
  /// GetNotificationPreferences 的 entries 內（註冊表不會在這個組合送達），或該組合不可關閉（PreferenceEntry.can_disable 為 false）
  /// 而 enabled 是 false，回 invalid_argument。重複設定同一個值成功。
  $async.Future<UpdateNotificationPreferenceResponse>
      updateNotificationPreference($pb.ClientContext? ctx,
              UpdateNotificationPreferenceRequest request) =>
          _client.invoke<UpdateNotificationPreferenceResponse>(
              ctx,
              'NotificationAppService',
              'UpdateNotificationPreference',
              request,
              UpdateNotificationPreferenceResponse());

  /// RegisterDeviceToken 註冊 App 的推播 token。App 每次啟動都呼叫，伺服器更新最後註冊時間；同一個 token 換帳號登入時改綁給呼叫者。
  /// 一個 token 只綁一個帳號與一支 App。
  $async.Future<RegisterDeviceTokenResponse> registerDeviceToken(
          $pb.ClientContext? ctx, RegisterDeviceTokenRequest request) =>
      _client.invoke<RegisterDeviceTokenResponse>(ctx, 'NotificationAppService',
          'RegisterDeviceToken', request, RegisterDeviceTokenResponse());

  /// DeleteDeviceToken 刪除呼叫者的一個 token（登出時呼叫）。token 不存在或屬於別的帳號也回成功，不洩漏它是否存在。
  $async.Future<DeleteDeviceTokenResponse> deleteDeviceToken(
          $pb.ClientContext? ctx, DeleteDeviceTokenRequest request) =>
      _client.invoke<DeleteDeviceTokenResponse>(ctx, 'NotificationAppService',
          'DeleteDeviceToken', request, DeleteDeviceTokenResponse());

  /// WatchNotifications 是指定受眾的即時訊號串流（server streaming）。串流只傳「該重讀了」的訊號，不傳通知內容，也沒有續傳游標：
  /// 連線建立後先送一則 inbox 訊號（目前的未讀數與最新一則的 ID），之後每有變化再送；client 收到訊號就重新呼叫 ListInboxItems 與其他列表，
  /// 斷線重連後也先重讀一次。訊號可能合併或遺漏，資料以列表為準。
  /// STORE 受眾的串流另外承載賣家工作區事件（seller_workspace）。STORE 受眾但呼叫者沒有現任店家回 not_found；
  /// 每個帳號同時開著的串流有上限，超過回 resource_exhausted。
  $async.Future<WatchNotificationsResponse> watchNotifications(
          $pb.ClientContext? ctx, WatchNotificationsRequest request) =>
      _client.invoke<WatchNotificationsResponse>(ctx, 'NotificationAppService',
          'WatchNotifications', request, WatchNotificationsResponse());
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
