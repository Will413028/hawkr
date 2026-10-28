// This is a generated file - do not edit.
//
// Generated from hawkr/content/v1alpha1/content.proto.

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
import 'content.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'content.pbenum.dart';

/// Banner 是首頁的一則 banner。
class Banner extends $pb.GeneratedMessage {
  factory Banner({
    $core.String? id,
    $core.String? title,
    $core.String? buttonText,
    $0.Image? previewImage,
    $0.Image? popupImage,
    BannerAction? action,
  }) {
    final result = Banner._();
    if (id != null) result.id = id;
    if (title != null) result.title = title;
    if (buttonText != null) result.buttonText = buttonText;
    if (previewImage != null) result.previewImage = previewImage;
    if (popupImage != null) result.popupImage = popupImage;
    if (action != null) result.action = action;
    return result;
  }

  Banner._();

  factory Banner.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Banner()..mergeFromBuffer(data, registry);
  factory Banner.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Banner()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Banner',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.content.v1alpha1'),
      createEmptyInstance: Banner.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'title')
    ..aOS(3, _omitFieldNames ? '' : 'buttonText')
    ..aOM<$0.Image>(4, _omitFieldNames ? '' : 'previewImage',
        subBuilder: $0.Image.$_createMessage)
    ..aOM<$0.Image>(5, _omitFieldNames ? '' : 'popupImage',
        subBuilder: $0.Image.$_createMessage)
    ..aOM<BannerAction>(6, _omitFieldNames ? '' : 'action',
        subBuilder: BannerAction.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Banner clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Banner copyWith(void Function(Banner) updates) =>
      super.copyWith((message) => updates(message as Banner)) as Banner;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Banner() / Banner.new instead')
  static Banner create() => Banner._();
  static $pb.GeneratedMessage $_createMessage() => Banner._();
  @$core.override
  Banner createEmptyInstance() => Banner._();
  @$core.pragma('dart2js:noInline')
  static Banner getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Banner>(Banner.$_createMessage);
  static Banner? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get title => $_getSZ(1);
  @$pb.TagNumber(2)
  set title($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasTitle() => $_has(1);
  @$pb.TagNumber(2)
  void clearTitle() => $_clearField(2);

  /// 按鈕文字；空字串表示不顯示按鈕。
  @$pb.TagNumber(3)
  $core.String get buttonText => $_getSZ(2);
  @$pb.TagNumber(3)
  set buttonText($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasButtonText() => $_has(2);
  @$pb.TagNumber(3)
  void clearButtonText() => $_clearField(3);

  /// 輪播上的預覽圖。
  @$pb.TagNumber(4)
  $0.Image get previewImage => $_getN(3);
  @$pb.TagNumber(4)
  set previewImage($0.Image value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasPreviewImage() => $_has(3);
  @$pb.TagNumber(4)
  void clearPreviewImage() => $_clearField(4);
  @$pb.TagNumber(4)
  $0.Image ensurePreviewImage() => $_ensure(3);

  /// 彈窗的大圖；點擊行為是 popup 時顯示。
  @$pb.TagNumber(5)
  $0.Image get popupImage => $_getN(4);
  @$pb.TagNumber(5)
  set popupImage($0.Image value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasPopupImage() => $_has(4);
  @$pb.TagNumber(5)
  void clearPopupImage() => $_clearField(5);
  @$pb.TagNumber(5)
  $0.Image ensurePopupImage() => $_ensure(4);

  /// 點擊之後的行為。
  @$pb.TagNumber(6)
  BannerAction get action => $_getN(5);
  @$pb.TagNumber(6)
  set action(BannerAction value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasAction() => $_has(5);
  @$pb.TagNumber(6)
  void clearAction() => $_clearField(6);
  @$pb.TagNumber(6)
  BannerAction ensureAction() => $_ensure(5);
}

/// Popup 顯示 banner 的彈窗大圖，不帶其他值。
class BannerAction_Popup extends $pb.GeneratedMessage {
  factory BannerAction_Popup() => BannerAction_Popup._();

  BannerAction_Popup._();

  factory BannerAction_Popup.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BannerAction_Popup()..mergeFromBuffer(data, registry);
  factory BannerAction_Popup.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BannerAction_Popup()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BannerAction.Popup',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.content.v1alpha1'),
      createEmptyInstance: BannerAction_Popup.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BannerAction_Popup clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BannerAction_Popup copyWith(void Function(BannerAction_Popup) updates) =>
      super.copyWith((message) => updates(message as BannerAction_Popup))
          as BannerAction_Popup;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use BannerAction_Popup() / BannerAction_Popup.new instead')
  static BannerAction_Popup create() => BannerAction_Popup._();
  static $pb.GeneratedMessage $_createMessage() => BannerAction_Popup._();
  @$core.override
  BannerAction_Popup createEmptyInstance() => BannerAction_Popup._();
  @$core.pragma('dart2js:noInline')
  static BannerAction_Popup getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BannerAction_Popup>(
          BannerAction_Popup.$_createMessage);
  static BannerAction_Popup? _defaultInstance;
}

enum BannerAction_Kind { popup, url, productId, postId, notSet }

/// BannerAction 是點擊 banner 之後的行為，一定是下列之一。
class BannerAction extends $pb.GeneratedMessage {
  factory BannerAction({
    BannerAction_Popup? popup,
    $core.String? url,
    $core.String? productId,
    $core.String? postId,
  }) {
    final result = BannerAction._();
    if (popup != null) result.popup = popup;
    if (url != null) result.url = url;
    if (productId != null) result.productId = productId;
    if (postId != null) result.postId = postId;
    return result;
  }

  BannerAction._();

  factory BannerAction.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BannerAction()..mergeFromBuffer(data, registry);
  factory BannerAction.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BannerAction()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, BannerAction_Kind> _BannerAction_KindByTag =
      {
    1: BannerAction_Kind.popup,
    2: BannerAction_Kind.url,
    3: BannerAction_Kind.productId,
    4: BannerAction_Kind.postId,
    0: BannerAction_Kind.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BannerAction',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.content.v1alpha1'),
      createEmptyInstance: BannerAction.$_createMessage)
    ..oo(0, [1, 2, 3, 4])
    ..aOM<BannerAction_Popup>(1, _omitFieldNames ? '' : 'popup',
        subBuilder: BannerAction_Popup.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'url')
    ..aOS(3, _omitFieldNames ? '' : 'productId')
    ..aOS(4, _omitFieldNames ? '' : 'postId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BannerAction clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BannerAction copyWith(void Function(BannerAction) updates) =>
      super.copyWith((message) => updates(message as BannerAction))
          as BannerAction;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use BannerAction() / BannerAction.new instead')
  static BannerAction create() => BannerAction._();
  static $pb.GeneratedMessage $_createMessage() => BannerAction._();
  @$core.override
  BannerAction createEmptyInstance() => BannerAction._();
  @$core.pragma('dart2js:noInline')
  static BannerAction getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<BannerAction>(
          BannerAction.$_createMessage);
  static BannerAction? _defaultInstance;

  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  @$pb.TagNumber(3)
  @$pb.TagNumber(4)
  BannerAction_Kind whichKind() => _BannerAction_KindByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  @$pb.TagNumber(3)
  @$pb.TagNumber(4)
  void clearKind() => $_clearField($_whichOneof(0));

  @$pb.TagNumber(1)
  BannerAction_Popup get popup => $_getN(0);
  @$pb.TagNumber(1)
  set popup(BannerAction_Popup value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPopup() => $_has(0);
  @$pb.TagNumber(1)
  void clearPopup() => $_clearField(1);
  @$pb.TagNumber(1)
  BannerAction_Popup ensurePopup() => $_ensure(0);

  /// 開啟這個外部網址，只接受 http:// 或 https:// 開頭。
  @$pb.TagNumber(2)
  $core.String get url => $_getSZ(1);
  @$pb.TagNumber(2)
  set url($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasUrl() => $_has(1);
  @$pb.TagNumber(2)
  void clearUrl() => $_clearField(2);

  /// 開啟這個商品的頁面。
  @$pb.TagNumber(3)
  $core.String get productId => $_getSZ(2);
  @$pb.TagNumber(3)
  set productId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasProductId() => $_has(2);
  @$pb.TagNumber(3)
  void clearProductId() => $_clearField(3);

  /// 開啟這篇貼文的頁面。
  @$pb.TagNumber(4)
  $core.String get postId => $_getSZ(3);
  @$pb.TagNumber(4)
  set postId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasPostId() => $_has(3);
  @$pb.TagNumber(4)
  void clearPostId() => $_clearField(4);
}

class ListLiveBannersRequest extends $pb.GeneratedMessage {
  factory ListLiveBannersRequest({
    $core.int? pageSize,
    $core.String? pageToken,
  }) {
    final result = ListLiveBannersRequest._();
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    return result;
  }

  ListLiveBannersRequest._();

  factory ListLiveBannersRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListLiveBannersRequest()..mergeFromBuffer(data, registry);
  factory ListLiveBannersRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListLiveBannersRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListLiveBannersRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.content.v1alpha1'),
      createEmptyInstance: ListLiveBannersRequest.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'pageSize')
    ..aOS(2, _omitFieldNames ? '' : 'pageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListLiveBannersRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListLiveBannersRequest copyWith(
          void Function(ListLiveBannersRequest) updates) =>
      super.copyWith((message) => updates(message as ListLiveBannersRequest))
          as ListLiveBannersRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListLiveBannersRequest() / ListLiveBannersRequest.new instead')
  static ListLiveBannersRequest create() => ListLiveBannersRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListLiveBannersRequest._();
  @$core.override
  ListLiveBannersRequest createEmptyInstance() => ListLiveBannersRequest._();
  @$core.pragma('dart2js:noInline')
  static ListLiveBannersRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListLiveBannersRequest>(
          ListLiveBannersRequest.$_createMessage);
  static ListLiveBannersRequest? _defaultInstance;

  /// 每頁最多幾則。0 表示 50；超過 100 時以 100 計；負數回 INVALID_ARGUMENT。
  @$pb.TagNumber(1)
  $core.int get pageSize => $_getIZ(0);
  @$pb.TagNumber(1)
  set pageSize($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPageSize() => $_has(0);
  @$pb.TagNumber(1)
  void clearPageSize() => $_clearField(1);

  /// 上一頁回應的 next_page_token；第一頁留空。其他欄位要與取得這個 token 的請求相同。
  @$pb.TagNumber(2)
  $core.String get pageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set pageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearPageToken() => $_clearField(2);
}

class ListLiveBannersResponse extends $pb.GeneratedMessage {
  factory ListLiveBannersResponse({
    $core.Iterable<Banner>? banners,
    $core.String? nextPageToken,
  }) {
    final result = ListLiveBannersResponse._();
    if (banners != null) result.banners.addAll(banners);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    return result;
  }

  ListLiveBannersResponse._();

  factory ListLiveBannersResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListLiveBannersResponse()..mergeFromBuffer(data, registry);
  factory ListLiveBannersResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListLiveBannersResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListLiveBannersResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.content.v1alpha1'),
      createEmptyInstance: ListLiveBannersResponse.$_createMessage)
    ..pPM<Banner>(1, _omitFieldNames ? '' : 'banners',
        subBuilder: Banner.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListLiveBannersResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListLiveBannersResponse copyWith(
          void Function(ListLiveBannersResponse) updates) =>
      super.copyWith((message) => updates(message as ListLiveBannersResponse))
          as ListLiveBannersResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListLiveBannersResponse() / ListLiveBannersResponse.new instead')
  static ListLiveBannersResponse create() => ListLiveBannersResponse._();
  static $pb.GeneratedMessage $_createMessage() => ListLiveBannersResponse._();
  @$core.override
  ListLiveBannersResponse createEmptyInstance() => ListLiveBannersResponse._();
  @$core.pragma('dart2js:noInline')
  static ListLiveBannersResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListLiveBannersResponse>(
          ListLiveBannersResponse.$_createMessage);
  static ListLiveBannersResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Banner> get banners => $_getList(0);

  /// 下一頁的 token；空字串表示這是最後一頁。
  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);
}

class CheckAppVersionRequest extends $pb.GeneratedMessage {
  factory CheckAppVersionRequest({
    AppType? appType,
    $core.String? clientVersion,
  }) {
    final result = CheckAppVersionRequest._();
    if (appType != null) result.appType = appType;
    if (clientVersion != null) result.clientVersion = clientVersion;
    return result;
  }

  CheckAppVersionRequest._();

  factory CheckAppVersionRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CheckAppVersionRequest()..mergeFromBuffer(data, registry);
  factory CheckAppVersionRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CheckAppVersionRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CheckAppVersionRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.content.v1alpha1'),
      createEmptyInstance: CheckAppVersionRequest.$_createMessage)
    ..aE<AppType>(1, _omitFieldNames ? '' : 'appType',
        enumValues: AppType.values)
    ..aOS(3, _omitFieldNames ? '' : 'clientVersion')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CheckAppVersionRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CheckAppVersionRequest copyWith(
          void Function(CheckAppVersionRequest) updates) =>
      super.copyWith((message) => updates(message as CheckAppVersionRequest))
          as CheckAppVersionRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CheckAppVersionRequest() / CheckAppVersionRequest.new instead')
  static CheckAppVersionRequest create() => CheckAppVersionRequest._();
  static $pb.GeneratedMessage $_createMessage() => CheckAppVersionRequest._();
  @$core.override
  CheckAppVersionRequest createEmptyInstance() => CheckAppVersionRequest._();
  @$core.pragma('dart2js:noInline')
  static CheckAppVersionRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CheckAppVersionRequest>(
          CheckAppVersionRequest.$_createMessage);
  static CheckAppVersionRequest? _defaultInstance;

  @$pb.TagNumber(1)
  AppType get appType => $_getN(0);
  @$pb.TagNumber(1)
  set appType(AppType value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasAppType() => $_has(0);
  @$pb.TagNumber(1)
  void clearAppType() => $_clearField(1);

  /// App 的版本 x.y.z，不含 build number。格式不對時不要求更新。
  @$pb.TagNumber(3)
  $core.String get clientVersion => $_getSZ(1);
  @$pb.TagNumber(3)
  set clientVersion($core.String value) => $_setString(1, value);
  @$pb.TagNumber(3)
  $core.bool hasClientVersion() => $_has(1);
  @$pb.TagNumber(3)
  void clearClientVersion() => $_clearField(3);
}

class CheckAppVersionResponse extends $pb.GeneratedMessage {
  factory CheckAppVersionResponse({
    $core.bool? updateRequired,
    $core.String? minimumVersion,
    $core.String? storeUrl,
    $core.String? noticeTitle,
    $core.String? noticeBody,
  }) {
    final result = CheckAppVersionResponse._();
    if (updateRequired != null) result.updateRequired = updateRequired;
    if (minimumVersion != null) result.minimumVersion = minimumVersion;
    if (storeUrl != null) result.storeUrl = storeUrl;
    if (noticeTitle != null) result.noticeTitle = noticeTitle;
    if (noticeBody != null) result.noticeBody = noticeBody;
    return result;
  }

  CheckAppVersionResponse._();

  factory CheckAppVersionResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CheckAppVersionResponse()..mergeFromBuffer(data, registry);
  factory CheckAppVersionResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CheckAppVersionResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CheckAppVersionResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.content.v1alpha1'),
      createEmptyInstance: CheckAppVersionResponse.$_createMessage)
    ..aOB(1, _omitFieldNames ? '' : 'updateRequired')
    ..aOS(2, _omitFieldNames ? '' : 'minimumVersion')
    ..aOS(3, _omitFieldNames ? '' : 'storeUrl')
    ..aOS(4, _omitFieldNames ? '' : 'noticeTitle')
    ..aOS(5, _omitFieldNames ? '' : 'noticeBody')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CheckAppVersionResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CheckAppVersionResponse copyWith(
          void Function(CheckAppVersionResponse) updates) =>
      super.copyWith((message) => updates(message as CheckAppVersionResponse))
          as CheckAppVersionResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CheckAppVersionResponse() / CheckAppVersionResponse.new instead')
  static CheckAppVersionResponse create() => CheckAppVersionResponse._();
  static $pb.GeneratedMessage $_createMessage() => CheckAppVersionResponse._();
  @$core.override
  CheckAppVersionResponse createEmptyInstance() => CheckAppVersionResponse._();
  @$core.pragma('dart2js:noInline')
  static CheckAppVersionResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CheckAppVersionResponse>(
          CheckAppVersionResponse.$_createMessage);
  static CheckAppVersionResponse? _defaultInstance;

  /// 是否必須更新。沒有設定門檻、或版本格式不對時一律是 false。
  @$pb.TagNumber(1)
  $core.bool get updateRequired => $_getBF(0);
  @$pb.TagNumber(1)
  set updateRequired($core.bool value) => $_setBool(0, value);
  @$pb.TagNumber(1)
  $core.bool hasUpdateRequired() => $_has(0);
  @$pb.TagNumber(1)
  void clearUpdateRequired() => $_clearField(1);

  /// 最低可用版本；沒有設定時是空字串。
  @$pb.TagNumber(2)
  $core.String get minimumVersion => $_getSZ(1);
  @$pb.TagNumber(2)
  set minimumVersion($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasMinimumVersion() => $_has(1);
  @$pb.TagNumber(2)
  void clearMinimumVersion() => $_clearField(2);

  /// 前往更新的商店網址；空字串表示沒有設定，App 隱藏「前往更新」按鈕。
  @$pb.TagNumber(3)
  $core.String get storeUrl => $_getSZ(2);
  @$pb.TagNumber(3)
  set storeUrl($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasStoreUrl() => $_has(2);
  @$pb.TagNumber(3)
  void clearStoreUrl() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get noticeTitle => $_getSZ(3);
  @$pb.TagNumber(4)
  set noticeTitle($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasNoticeTitle() => $_has(3);
  @$pb.TagNumber(4)
  void clearNoticeTitle() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get noticeBody => $_getSZ(4);
  @$pb.TagNumber(5)
  set noticeBody($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasNoticeBody() => $_has(4);
  @$pb.TagNumber(5)
  void clearNoticeBody() => $_clearField(5);
}

/// ContentPublicService 是前台讀取的內容，不需要登入。
class ContentPublicServiceApi {
  final $pb.RpcClient _client;

  ContentPublicServiceApi(this._client);

  /// ListLiveBanners 列出此刻要在首頁顯示的 banner：啟用中、而且在顯示期間 [開始, 結束) 內，依顯示順序排列。
  /// 啟用中的 banner 最多 20 則（寫入時限制），第一頁（預設 50 則）就是全部，client 不必翻頁。
  $async.Future<ListLiveBannersResponse> listLiveBanners(
          $pb.ClientContext? ctx, ListLiveBannersRequest request) =>
      _client.invoke<ListLiveBannersResponse>(ctx, 'ContentPublicService',
          'ListLiveBanners', request, ListLiveBannersResponse());

  /// CheckAppVersion 判斷 App 是否低於最低版本、必須強制更新（設計書 54）。版本比較只在後端做，
  /// App 只看 update_required。App 收到任何錯誤都要放行（fail-open）。
  $async.Future<CheckAppVersionResponse> checkAppVersion(
          $pb.ClientContext? ctx, CheckAppVersionRequest request) =>
      _client.invoke<CheckAppVersionResponse>(ctx, 'ContentPublicService',
          'CheckAppVersion', request, CheckAppVersionResponse());
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
