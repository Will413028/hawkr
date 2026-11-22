// This is a generated file - do not edit.
//
// Generated from hawkr/crm/v1alpha1/crm.proto.

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
import 'package:protobuf/well_known_types/google/protobuf/field_mask.pb.dart'
    as $2;
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart'
    as $1;

import '../../common/v1alpha1/image.pb.dart' as $0;
import 'crm.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'crm.pbenum.dart';

/// Tag 是店家的標籤。
class Tag extends $pb.GeneratedMessage {
  factory Tag({
    $core.String? id,
    $core.String? name,
  }) {
    final result = Tag._();
    if (id != null) result.id = id;
    if (name != null) result.name = name;
    return result;
  }

  Tag._();

  factory Tag.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Tag()..mergeFromBuffer(data, registry);
  factory Tag.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Tag()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Tag',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.crm.v1alpha1'),
      createEmptyInstance: Tag.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'name')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Tag clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Tag copyWith(void Function(Tag) updates) =>
      super.copyWith((message) => updates(message as Tag)) as Tag;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Tag() / Tag.new instead')
  static Tag create() => Tag._();
  static $pb.GeneratedMessage $_createMessage() => Tag._();
  @$core.override
  Tag createEmptyInstance() => Tag._();
  @$core.pragma('dart2js:noInline')
  static Tag getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Tag>(Tag.$_createMessage);
  static Tag? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  /// 名稱，去掉前後空白後 1 到 50 個字元；UpdateTag 唯一可以修改的欄位。
  @$pb.TagNumber(2)
  $core.String get name => $_getSZ(1);
  @$pb.TagNumber(2)
  set name($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasName() => $_has(1);
  @$pb.TagNumber(2)
  void clearName() => $_clearField(2);
}

/// CustomerDisplay 是客戶的顯示資料，在讀取時由 buyer 補；不含任何聯絡方式。
class CustomerDisplay extends $pb.GeneratedMessage {
  factory CustomerDisplay({
    $core.String? nickname,
    $0.Image? avatar,
    $core.bool? deleted,
  }) {
    final result = CustomerDisplay._();
    if (nickname != null) result.nickname = nickname;
    if (avatar != null) result.avatar = avatar;
    if (deleted != null) result.deleted = deleted;
    return result;
  }

  CustomerDisplay._();

  factory CustomerDisplay.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CustomerDisplay()..mergeFromBuffer(data, registry);
  factory CustomerDisplay.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CustomerDisplay()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CustomerDisplay',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.crm.v1alpha1'),
      createEmptyInstance: CustomerDisplay.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'nickname')
    ..aOM<$0.Image>(2, _omitFieldNames ? '' : 'avatar',
        subBuilder: $0.Image.$_createMessage)
    ..aOB(3, _omitFieldNames ? '' : 'deleted')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CustomerDisplay clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CustomerDisplay copyWith(void Function(CustomerDisplay) updates) =>
      super.copyWith((message) => updates(message as CustomerDisplay))
          as CustomerDisplay;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use CustomerDisplay() / CustomerDisplay.new instead')
  static CustomerDisplay create() => CustomerDisplay._();
  static $pb.GeneratedMessage $_createMessage() => CustomerDisplay._();
  @$core.override
  CustomerDisplay createEmptyInstance() => CustomerDisplay._();
  @$core.pragma('dart2js:noInline')
  static CustomerDisplay getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<CustomerDisplay>(
          CustomerDisplay.$_createMessage);
  static CustomerDisplay? _defaultInstance;

  /// 買家的暱稱；沒有設定或買家已刪除時不設。
  @$pb.TagNumber(1)
  $core.String get nickname => $_getSZ(0);
  @$pb.TagNumber(1)
  set nickname($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasNickname() => $_has(0);
  @$pb.TagNumber(1)
  void clearNickname() => $_clearField(1);

  /// 買家的頭像；沒有設定或買家已刪除時不設。
  @$pb.TagNumber(2)
  $0.Image get avatar => $_getN(1);
  @$pb.TagNumber(2)
  set avatar($0.Image value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasAvatar() => $_has(1);
  @$pb.TagNumber(2)
  void clearAvatar() => $_clearField(2);
  @$pb.TagNumber(2)
  $0.Image ensureAvatar() => $_ensure(1);

  /// 買家帳號已刪除。
  @$pb.TagNumber(3)
  $core.bool get deleted => $_getBF(2);
  @$pb.TagNumber(3)
  set deleted($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasDeleted() => $_has(2);
  @$pb.TagNumber(3)
  void clearDeleted() => $_clearField(3);
}

/// Customer 是店家的一位客戶。只有這些欄位：沒有電話、Email、地址。
class Customer extends $pb.GeneratedMessage {
  factory Customer({
    $core.String? buyerId,
    CustomerDisplay? display,
    CustomerType? type,
    $fixnum.Int64? totalSpent,
    $core.int? orderCount,
    $1.Timestamp? firstOrderTime,
    $core.Iterable<Tag>? tags,
  }) {
    final result = Customer._();
    if (buyerId != null) result.buyerId = buyerId;
    if (display != null) result.display = display;
    if (type != null) result.type = type;
    if (totalSpent != null) result.totalSpent = totalSpent;
    if (orderCount != null) result.orderCount = orderCount;
    if (firstOrderTime != null) result.firstOrderTime = firstOrderTime;
    if (tags != null) result.tags.addAll(tags);
    return result;
  }

  Customer._();

  factory Customer.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Customer()..mergeFromBuffer(data, registry);
  factory Customer.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Customer()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Customer',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.crm.v1alpha1'),
      createEmptyInstance: Customer.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'buyerId')
    ..aOM<CustomerDisplay>(2, _omitFieldNames ? '' : 'display',
        subBuilder: CustomerDisplay.$_createMessage)
    ..aE<CustomerType>(3, _omitFieldNames ? '' : 'type',
        enumValues: CustomerType.values)
    ..aInt64(4, _omitFieldNames ? '' : 'totalSpent')
    ..aI(5, _omitFieldNames ? '' : 'orderCount')
    ..aOM<$1.Timestamp>(6, _omitFieldNames ? '' : 'firstOrderTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..pPM<Tag>(7, _omitFieldNames ? '' : 'tags',
        subBuilder: Tag.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Customer clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Customer copyWith(void Function(Customer) updates) =>
      super.copyWith((message) => updates(message as Customer)) as Customer;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Customer() / Customer.new instead')
  static Customer create() => Customer._();
  static $pb.GeneratedMessage $_createMessage() => Customer._();
  @$core.override
  Customer createEmptyInstance() => Customer._();
  @$core.pragma('dart2js:noInline')
  static Customer getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Customer>(Customer.$_createMessage);
  static Customer? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get buyerId => $_getSZ(0);
  @$pb.TagNumber(1)
  set buyerId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBuyerId() => $_has(0);
  @$pb.TagNumber(1)
  void clearBuyerId() => $_clearField(1);

  @$pb.TagNumber(2)
  CustomerDisplay get display => $_getN(1);
  @$pb.TagNumber(2)
  set display(CustomerDisplay value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasDisplay() => $_has(1);
  @$pb.TagNumber(2)
  void clearDisplay() => $_clearField(2);
  @$pb.TagNumber(2)
  CustomerDisplay ensureDisplay() => $_ensure(1);

  /// 分類；UpdateCustomer 唯一可以修改的欄位。
  @$pb.TagNumber(3)
  CustomerType get type => $_getN(2);
  @$pb.TagNumber(3)
  set type(CustomerType value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasType() => $_has(2);
  @$pb.TagNumber(3)
  void clearType() => $_clearField(3);

  /// 完成訂單的累計實付淨額，新台幣元（退款已扣，不為負）。
  @$pb.TagNumber(4)
  $fixnum.Int64 get totalSpent => $_getI64(3);
  @$pb.TagNumber(4)
  set totalSpent($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasTotalSpent() => $_has(3);
  @$pb.TagNumber(4)
  void clearTotalSpent() => $_clearField(4);

  /// 完成的訂單數；退款不扣。
  @$pb.TagNumber(5)
  $core.int get orderCount => $_getIZ(4);
  @$pb.TagNumber(5)
  set orderCount($core.int value) => $_setSignedInt32(4, value);
  @$pb.TagNumber(5)
  $core.bool hasOrderCount() => $_has(4);
  @$pb.TagNumber(5)
  void clearOrderCount() => $_clearField(5);

  /// 成為客戶的時間（首筆完成訂單）。
  @$pb.TagNumber(6)
  $1.Timestamp get firstOrderTime => $_getN(5);
  @$pb.TagNumber(6)
  set firstOrderTime($1.Timestamp value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasFirstOrderTime() => $_has(5);
  @$pb.TagNumber(6)
  void clearFirstOrderTime() => $_clearField(6);
  @$pb.TagNumber(6)
  $1.Timestamp ensureFirstOrderTime() => $_ensure(5);

  /// 貼在客戶身上的標籤，名稱升序。
  @$pb.TagNumber(7)
  $pb.PbList<Tag> get tags => $_getList(6);
}

class ListCustomersRequest extends $pb.GeneratedMessage {
  factory ListCustomersRequest({
    $core.String? storeId,
    CustomerType? type,
    $core.String? tagId,
    CustomerSort? sort,
    $core.bool? descending,
    $core.int? pageSize,
    $core.String? pageToken,
  }) {
    final result = ListCustomersRequest._();
    if (storeId != null) result.storeId = storeId;
    if (type != null) result.type = type;
    if (tagId != null) result.tagId = tagId;
    if (sort != null) result.sort = sort;
    if (descending != null) result.descending = descending;
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    return result;
  }

  ListCustomersRequest._();

  factory ListCustomersRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListCustomersRequest()..mergeFromBuffer(data, registry);
  factory ListCustomersRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListCustomersRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListCustomersRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.crm.v1alpha1'),
      createEmptyInstance: ListCustomersRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'storeId')
    ..aE<CustomerType>(2, _omitFieldNames ? '' : 'type',
        enumValues: CustomerType.values)
    ..aOS(3, _omitFieldNames ? '' : 'tagId')
    ..aE<CustomerSort>(4, _omitFieldNames ? '' : 'sort',
        enumValues: CustomerSort.values)
    ..aOB(5, _omitFieldNames ? '' : 'descending')
    ..aI(6, _omitFieldNames ? '' : 'pageSize')
    ..aOS(7, _omitFieldNames ? '' : 'pageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListCustomersRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListCustomersRequest copyWith(void Function(ListCustomersRequest) updates) =>
      super.copyWith((message) => updates(message as ListCustomersRequest))
          as ListCustomersRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListCustomersRequest() / ListCustomersRequest.new instead')
  static ListCustomersRequest create() => ListCustomersRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListCustomersRequest._();
  @$core.override
  ListCustomersRequest createEmptyInstance() => ListCustomersRequest._();
  @$core.pragma('dart2js:noInline')
  static ListCustomersRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListCustomersRequest>(
          ListCustomersRequest.$_createMessage);
  static ListCustomersRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storeId => $_getSZ(0);
  @$pb.TagNumber(1)
  set storeId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStoreId() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoreId() => $_clearField(1);

  @$pb.TagNumber(2)
  CustomerType get type => $_getN(1);
  @$pb.TagNumber(2)
  set type(CustomerType value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasType() => $_has(1);
  @$pb.TagNumber(2)
  void clearType() => $_clearField(2);

  /// 只列貼了這個標籤的客戶；標籤的 ID。
  @$pb.TagNumber(3)
  $core.String get tagId => $_getSZ(2);
  @$pb.TagNumber(3)
  set tagId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasTagId() => $_has(2);
  @$pb.TagNumber(3)
  void clearTagId() => $_clearField(3);

  @$pb.TagNumber(4)
  CustomerSort get sort => $_getN(3);
  @$pb.TagNumber(4)
  set sort(CustomerSort value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasSort() => $_has(3);
  @$pb.TagNumber(4)
  void clearSort() => $_clearField(4);

  /// 為真時大的在前。
  @$pb.TagNumber(5)
  $core.bool get descending => $_getBF(4);
  @$pb.TagNumber(5)
  set descending($core.bool value) => $_setBool(4, value);
  @$pb.TagNumber(5)
  $core.bool hasDescending() => $_has(4);
  @$pb.TagNumber(5)
  void clearDescending() => $_clearField(5);

  /// 預設 20，上限 100。
  @$pb.TagNumber(6)
  $core.int get pageSize => $_getIZ(5);
  @$pb.TagNumber(6)
  set pageSize($core.int value) => $_setSignedInt32(5, value);
  @$pb.TagNumber(6)
  $core.bool hasPageSize() => $_has(5);
  @$pb.TagNumber(6)
  void clearPageSize() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get pageToken => $_getSZ(6);
  @$pb.TagNumber(7)
  set pageToken($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasPageToken() => $_has(6);
  @$pb.TagNumber(7)
  void clearPageToken() => $_clearField(7);
}

class ListCustomersResponse extends $pb.GeneratedMessage {
  factory ListCustomersResponse({
    $core.Iterable<Customer>? customers,
    $core.String? nextPageToken,
  }) {
    final result = ListCustomersResponse._();
    if (customers != null) result.customers.addAll(customers);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    return result;
  }

  ListCustomersResponse._();

  factory ListCustomersResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListCustomersResponse()..mergeFromBuffer(data, registry);
  factory ListCustomersResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListCustomersResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListCustomersResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.crm.v1alpha1'),
      createEmptyInstance: ListCustomersResponse.$_createMessage)
    ..pPM<Customer>(1, _omitFieldNames ? '' : 'customers',
        subBuilder: Customer.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListCustomersResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListCustomersResponse copyWith(
          void Function(ListCustomersResponse) updates) =>
      super.copyWith((message) => updates(message as ListCustomersResponse))
          as ListCustomersResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListCustomersResponse() / ListCustomersResponse.new instead')
  static ListCustomersResponse create() => ListCustomersResponse._();
  static $pb.GeneratedMessage $_createMessage() => ListCustomersResponse._();
  @$core.override
  ListCustomersResponse createEmptyInstance() => ListCustomersResponse._();
  @$core.pragma('dart2js:noInline')
  static ListCustomersResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListCustomersResponse>(
          ListCustomersResponse.$_createMessage);
  static ListCustomersResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Customer> get customers => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);
}

class GetCustomerRequest extends $pb.GeneratedMessage {
  factory GetCustomerRequest({
    $core.String? storeId,
    $core.String? buyerId,
  }) {
    final result = GetCustomerRequest._();
    if (storeId != null) result.storeId = storeId;
    if (buyerId != null) result.buyerId = buyerId;
    return result;
  }

  GetCustomerRequest._();

  factory GetCustomerRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetCustomerRequest()..mergeFromBuffer(data, registry);
  factory GetCustomerRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetCustomerRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetCustomerRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.crm.v1alpha1'),
      createEmptyInstance: GetCustomerRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'storeId')
    ..aOS(2, _omitFieldNames ? '' : 'buyerId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetCustomerRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetCustomerRequest copyWith(void Function(GetCustomerRequest) updates) =>
      super.copyWith((message) => updates(message as GetCustomerRequest))
          as GetCustomerRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GetCustomerRequest() / GetCustomerRequest.new instead')
  static GetCustomerRequest create() => GetCustomerRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetCustomerRequest._();
  @$core.override
  GetCustomerRequest createEmptyInstance() => GetCustomerRequest._();
  @$core.pragma('dart2js:noInline')
  static GetCustomerRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetCustomerRequest>(
          GetCustomerRequest.$_createMessage);
  static GetCustomerRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storeId => $_getSZ(0);
  @$pb.TagNumber(1)
  set storeId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStoreId() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoreId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get buyerId => $_getSZ(1);
  @$pb.TagNumber(2)
  set buyerId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasBuyerId() => $_has(1);
  @$pb.TagNumber(2)
  void clearBuyerId() => $_clearField(2);
}

class GetCustomerResponse extends $pb.GeneratedMessage {
  factory GetCustomerResponse({
    Customer? customer,
  }) {
    final result = GetCustomerResponse._();
    if (customer != null) result.customer = customer;
    return result;
  }

  GetCustomerResponse._();

  factory GetCustomerResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetCustomerResponse()..mergeFromBuffer(data, registry);
  factory GetCustomerResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetCustomerResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetCustomerResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.crm.v1alpha1'),
      createEmptyInstance: GetCustomerResponse.$_createMessage)
    ..aOM<Customer>(1, _omitFieldNames ? '' : 'customer',
        subBuilder: Customer.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetCustomerResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetCustomerResponse copyWith(void Function(GetCustomerResponse) updates) =>
      super.copyWith((message) => updates(message as GetCustomerResponse))
          as GetCustomerResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use GetCustomerResponse() / GetCustomerResponse.new instead')
  static GetCustomerResponse create() => GetCustomerResponse._();
  static $pb.GeneratedMessage $_createMessage() => GetCustomerResponse._();
  @$core.override
  GetCustomerResponse createEmptyInstance() => GetCustomerResponse._();
  @$core.pragma('dart2js:noInline')
  static GetCustomerResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetCustomerResponse>(
          GetCustomerResponse.$_createMessage);
  static GetCustomerResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Customer get customer => $_getN(0);
  @$pb.TagNumber(1)
  set customer(Customer value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasCustomer() => $_has(0);
  @$pb.TagNumber(1)
  void clearCustomer() => $_clearField(1);
  @$pb.TagNumber(1)
  Customer ensureCustomer() => $_ensure(0);
}

class UpdateCustomerRequest extends $pb.GeneratedMessage {
  factory UpdateCustomerRequest({
    $core.String? storeId,
    Customer? customer,
    $2.FieldMask? updateMask,
  }) {
    final result = UpdateCustomerRequest._();
    if (storeId != null) result.storeId = storeId;
    if (customer != null) result.customer = customer;
    if (updateMask != null) result.updateMask = updateMask;
    return result;
  }

  UpdateCustomerRequest._();

  factory UpdateCustomerRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateCustomerRequest()..mergeFromBuffer(data, registry);
  factory UpdateCustomerRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateCustomerRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateCustomerRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.crm.v1alpha1'),
      createEmptyInstance: UpdateCustomerRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'storeId')
    ..aOM<Customer>(2, _omitFieldNames ? '' : 'customer',
        subBuilder: Customer.$_createMessage)
    ..aOM<$2.FieldMask>(3, _omitFieldNames ? '' : 'updateMask',
        subBuilder: $2.FieldMask.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateCustomerRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateCustomerRequest copyWith(
          void Function(UpdateCustomerRequest) updates) =>
      super.copyWith((message) => updates(message as UpdateCustomerRequest))
          as UpdateCustomerRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use UpdateCustomerRequest() / UpdateCustomerRequest.new instead')
  static UpdateCustomerRequest create() => UpdateCustomerRequest._();
  static $pb.GeneratedMessage $_createMessage() => UpdateCustomerRequest._();
  @$core.override
  UpdateCustomerRequest createEmptyInstance() => UpdateCustomerRequest._();
  @$core.pragma('dart2js:noInline')
  static UpdateCustomerRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateCustomerRequest>(
          UpdateCustomerRequest.$_createMessage);
  static UpdateCustomerRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storeId => $_getSZ(0);
  @$pb.TagNumber(1)
  set storeId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStoreId() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoreId() => $_clearField(1);

  /// 只有 buyer_id 與 type 有作用。
  @$pb.TagNumber(2)
  Customer get customer => $_getN(1);
  @$pb.TagNumber(2)
  set customer(Customer value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasCustomer() => $_has(1);
  @$pb.TagNumber(2)
  void clearCustomer() => $_clearField(2);
  @$pb.TagNumber(2)
  Customer ensureCustomer() => $_ensure(1);

  /// 只能列 type。
  @$pb.TagNumber(3)
  $2.FieldMask get updateMask => $_getN(2);
  @$pb.TagNumber(3)
  set updateMask($2.FieldMask value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasUpdateMask() => $_has(2);
  @$pb.TagNumber(3)
  void clearUpdateMask() => $_clearField(3);
  @$pb.TagNumber(3)
  $2.FieldMask ensureUpdateMask() => $_ensure(2);
}

class UpdateCustomerResponse extends $pb.GeneratedMessage {
  factory UpdateCustomerResponse({
    Customer? customer,
  }) {
    final result = UpdateCustomerResponse._();
    if (customer != null) result.customer = customer;
    return result;
  }

  UpdateCustomerResponse._();

  factory UpdateCustomerResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateCustomerResponse()..mergeFromBuffer(data, registry);
  factory UpdateCustomerResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateCustomerResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateCustomerResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.crm.v1alpha1'),
      createEmptyInstance: UpdateCustomerResponse.$_createMessage)
    ..aOM<Customer>(1, _omitFieldNames ? '' : 'customer',
        subBuilder: Customer.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateCustomerResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateCustomerResponse copyWith(
          void Function(UpdateCustomerResponse) updates) =>
      super.copyWith((message) => updates(message as UpdateCustomerResponse))
          as UpdateCustomerResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use UpdateCustomerResponse() / UpdateCustomerResponse.new instead')
  static UpdateCustomerResponse create() => UpdateCustomerResponse._();
  static $pb.GeneratedMessage $_createMessage() => UpdateCustomerResponse._();
  @$core.override
  UpdateCustomerResponse createEmptyInstance() => UpdateCustomerResponse._();
  @$core.pragma('dart2js:noInline')
  static UpdateCustomerResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateCustomerResponse>(
          UpdateCustomerResponse.$_createMessage);
  static UpdateCustomerResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Customer get customer => $_getN(0);
  @$pb.TagNumber(1)
  set customer(Customer value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasCustomer() => $_has(0);
  @$pb.TagNumber(1)
  void clearCustomer() => $_clearField(1);
  @$pb.TagNumber(1)
  Customer ensureCustomer() => $_ensure(0);
}

class CreateTagRequest extends $pb.GeneratedMessage {
  factory CreateTagRequest({
    $core.String? storeId,
    $core.String? name,
  }) {
    final result = CreateTagRequest._();
    if (storeId != null) result.storeId = storeId;
    if (name != null) result.name = name;
    return result;
  }

  CreateTagRequest._();

  factory CreateTagRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateTagRequest()..mergeFromBuffer(data, registry);
  factory CreateTagRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateTagRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateTagRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.crm.v1alpha1'),
      createEmptyInstance: CreateTagRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'storeId')
    ..aOS(2, _omitFieldNames ? '' : 'name')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateTagRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateTagRequest copyWith(void Function(CreateTagRequest) updates) =>
      super.copyWith((message) => updates(message as CreateTagRequest))
          as CreateTagRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use CreateTagRequest() / CreateTagRequest.new instead')
  static CreateTagRequest create() => CreateTagRequest._();
  static $pb.GeneratedMessage $_createMessage() => CreateTagRequest._();
  @$core.override
  CreateTagRequest createEmptyInstance() => CreateTagRequest._();
  @$core.pragma('dart2js:noInline')
  static CreateTagRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<CreateTagRequest>(
          CreateTagRequest.$_createMessage);
  static CreateTagRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storeId => $_getSZ(0);
  @$pb.TagNumber(1)
  set storeId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStoreId() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoreId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get name => $_getSZ(1);
  @$pb.TagNumber(2)
  set name($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasName() => $_has(1);
  @$pb.TagNumber(2)
  void clearName() => $_clearField(2);
}

class CreateTagResponse extends $pb.GeneratedMessage {
  factory CreateTagResponse({
    Tag? tag,
  }) {
    final result = CreateTagResponse._();
    if (tag != null) result.tag = tag;
    return result;
  }

  CreateTagResponse._();

  factory CreateTagResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateTagResponse()..mergeFromBuffer(data, registry);
  factory CreateTagResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateTagResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateTagResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.crm.v1alpha1'),
      createEmptyInstance: CreateTagResponse.$_createMessage)
    ..aOM<Tag>(1, _omitFieldNames ? '' : 'tag', subBuilder: Tag.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateTagResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateTagResponse copyWith(void Function(CreateTagResponse) updates) =>
      super.copyWith((message) => updates(message as CreateTagResponse))
          as CreateTagResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use CreateTagResponse() / CreateTagResponse.new instead')
  static CreateTagResponse create() => CreateTagResponse._();
  static $pb.GeneratedMessage $_createMessage() => CreateTagResponse._();
  @$core.override
  CreateTagResponse createEmptyInstance() => CreateTagResponse._();
  @$core.pragma('dart2js:noInline')
  static CreateTagResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<CreateTagResponse>(
          CreateTagResponse.$_createMessage);
  static CreateTagResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Tag get tag => $_getN(0);
  @$pb.TagNumber(1)
  set tag(Tag value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasTag() => $_has(0);
  @$pb.TagNumber(1)
  void clearTag() => $_clearField(1);
  @$pb.TagNumber(1)
  Tag ensureTag() => $_ensure(0);
}

class ListTagsRequest extends $pb.GeneratedMessage {
  factory ListTagsRequest({
    $core.String? storeId,
  }) {
    final result = ListTagsRequest._();
    if (storeId != null) result.storeId = storeId;
    return result;
  }

  ListTagsRequest._();

  factory ListTagsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListTagsRequest()..mergeFromBuffer(data, registry);
  factory ListTagsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListTagsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListTagsRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.crm.v1alpha1'),
      createEmptyInstance: ListTagsRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'storeId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListTagsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListTagsRequest copyWith(void Function(ListTagsRequest) updates) =>
      super.copyWith((message) => updates(message as ListTagsRequest))
          as ListTagsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ListTagsRequest() / ListTagsRequest.new instead')
  static ListTagsRequest create() => ListTagsRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListTagsRequest._();
  @$core.override
  ListTagsRequest createEmptyInstance() => ListTagsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListTagsRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ListTagsRequest>(
          ListTagsRequest.$_createMessage);
  static ListTagsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storeId => $_getSZ(0);
  @$pb.TagNumber(1)
  set storeId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStoreId() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoreId() => $_clearField(1);
}

class ListTagsResponse extends $pb.GeneratedMessage {
  factory ListTagsResponse({
    $core.Iterable<Tag>? tags,
  }) {
    final result = ListTagsResponse._();
    if (tags != null) result.tags.addAll(tags);
    return result;
  }

  ListTagsResponse._();

  factory ListTagsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListTagsResponse()..mergeFromBuffer(data, registry);
  factory ListTagsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListTagsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListTagsResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.crm.v1alpha1'),
      createEmptyInstance: ListTagsResponse.$_createMessage)
    ..pPM<Tag>(1, _omitFieldNames ? '' : 'tags',
        subBuilder: Tag.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListTagsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListTagsResponse copyWith(void Function(ListTagsResponse) updates) =>
      super.copyWith((message) => updates(message as ListTagsResponse))
          as ListTagsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ListTagsResponse() / ListTagsResponse.new instead')
  static ListTagsResponse create() => ListTagsResponse._();
  static $pb.GeneratedMessage $_createMessage() => ListTagsResponse._();
  @$core.override
  ListTagsResponse createEmptyInstance() => ListTagsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListTagsResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ListTagsResponse>(
          ListTagsResponse.$_createMessage);
  static ListTagsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Tag> get tags => $_getList(0);
}

class UpdateTagRequest extends $pb.GeneratedMessage {
  factory UpdateTagRequest({
    $core.String? storeId,
    Tag? tag,
    $2.FieldMask? updateMask,
  }) {
    final result = UpdateTagRequest._();
    if (storeId != null) result.storeId = storeId;
    if (tag != null) result.tag = tag;
    if (updateMask != null) result.updateMask = updateMask;
    return result;
  }

  UpdateTagRequest._();

  factory UpdateTagRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateTagRequest()..mergeFromBuffer(data, registry);
  factory UpdateTagRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateTagRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateTagRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.crm.v1alpha1'),
      createEmptyInstance: UpdateTagRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'storeId')
    ..aOM<Tag>(2, _omitFieldNames ? '' : 'tag', subBuilder: Tag.$_createMessage)
    ..aOM<$2.FieldMask>(3, _omitFieldNames ? '' : 'updateMask',
        subBuilder: $2.FieldMask.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateTagRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateTagRequest copyWith(void Function(UpdateTagRequest) updates) =>
      super.copyWith((message) => updates(message as UpdateTagRequest))
          as UpdateTagRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use UpdateTagRequest() / UpdateTagRequest.new instead')
  static UpdateTagRequest create() => UpdateTagRequest._();
  static $pb.GeneratedMessage $_createMessage() => UpdateTagRequest._();
  @$core.override
  UpdateTagRequest createEmptyInstance() => UpdateTagRequest._();
  @$core.pragma('dart2js:noInline')
  static UpdateTagRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<UpdateTagRequest>(
          UpdateTagRequest.$_createMessage);
  static UpdateTagRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storeId => $_getSZ(0);
  @$pb.TagNumber(1)
  set storeId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStoreId() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoreId() => $_clearField(1);

  /// 只有 id 與 name 有作用。
  @$pb.TagNumber(2)
  Tag get tag => $_getN(1);
  @$pb.TagNumber(2)
  set tag(Tag value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasTag() => $_has(1);
  @$pb.TagNumber(2)
  void clearTag() => $_clearField(2);
  @$pb.TagNumber(2)
  Tag ensureTag() => $_ensure(1);

  /// 只能列 name。
  @$pb.TagNumber(3)
  $2.FieldMask get updateMask => $_getN(2);
  @$pb.TagNumber(3)
  set updateMask($2.FieldMask value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasUpdateMask() => $_has(2);
  @$pb.TagNumber(3)
  void clearUpdateMask() => $_clearField(3);
  @$pb.TagNumber(3)
  $2.FieldMask ensureUpdateMask() => $_ensure(2);
}

class UpdateTagResponse extends $pb.GeneratedMessage {
  factory UpdateTagResponse({
    Tag? tag,
  }) {
    final result = UpdateTagResponse._();
    if (tag != null) result.tag = tag;
    return result;
  }

  UpdateTagResponse._();

  factory UpdateTagResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateTagResponse()..mergeFromBuffer(data, registry);
  factory UpdateTagResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateTagResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateTagResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.crm.v1alpha1'),
      createEmptyInstance: UpdateTagResponse.$_createMessage)
    ..aOM<Tag>(1, _omitFieldNames ? '' : 'tag', subBuilder: Tag.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateTagResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateTagResponse copyWith(void Function(UpdateTagResponse) updates) =>
      super.copyWith((message) => updates(message as UpdateTagResponse))
          as UpdateTagResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use UpdateTagResponse() / UpdateTagResponse.new instead')
  static UpdateTagResponse create() => UpdateTagResponse._();
  static $pb.GeneratedMessage $_createMessage() => UpdateTagResponse._();
  @$core.override
  UpdateTagResponse createEmptyInstance() => UpdateTagResponse._();
  @$core.pragma('dart2js:noInline')
  static UpdateTagResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<UpdateTagResponse>(
          UpdateTagResponse.$_createMessage);
  static UpdateTagResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Tag get tag => $_getN(0);
  @$pb.TagNumber(1)
  set tag(Tag value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasTag() => $_has(0);
  @$pb.TagNumber(1)
  void clearTag() => $_clearField(1);
  @$pb.TagNumber(1)
  Tag ensureTag() => $_ensure(0);
}

class DeleteTagRequest extends $pb.GeneratedMessage {
  factory DeleteTagRequest({
    $core.String? storeId,
    $core.String? tagId,
  }) {
    final result = DeleteTagRequest._();
    if (storeId != null) result.storeId = storeId;
    if (tagId != null) result.tagId = tagId;
    return result;
  }

  DeleteTagRequest._();

  factory DeleteTagRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteTagRequest()..mergeFromBuffer(data, registry);
  factory DeleteTagRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteTagRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteTagRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.crm.v1alpha1'),
      createEmptyInstance: DeleteTagRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'storeId')
    ..aOS(2, _omitFieldNames ? '' : 'tagId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteTagRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteTagRequest copyWith(void Function(DeleteTagRequest) updates) =>
      super.copyWith((message) => updates(message as DeleteTagRequest))
          as DeleteTagRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use DeleteTagRequest() / DeleteTagRequest.new instead')
  static DeleteTagRequest create() => DeleteTagRequest._();
  static $pb.GeneratedMessage $_createMessage() => DeleteTagRequest._();
  @$core.override
  DeleteTagRequest createEmptyInstance() => DeleteTagRequest._();
  @$core.pragma('dart2js:noInline')
  static DeleteTagRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<DeleteTagRequest>(
          DeleteTagRequest.$_createMessage);
  static DeleteTagRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storeId => $_getSZ(0);
  @$pb.TagNumber(1)
  set storeId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStoreId() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoreId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get tagId => $_getSZ(1);
  @$pb.TagNumber(2)
  set tagId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasTagId() => $_has(1);
  @$pb.TagNumber(2)
  void clearTagId() => $_clearField(2);
}

class DeleteTagResponse extends $pb.GeneratedMessage {
  factory DeleteTagResponse() => DeleteTagResponse._();

  DeleteTagResponse._();

  factory DeleteTagResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteTagResponse()..mergeFromBuffer(data, registry);
  factory DeleteTagResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteTagResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteTagResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.crm.v1alpha1'),
      createEmptyInstance: DeleteTagResponse.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteTagResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteTagResponse copyWith(void Function(DeleteTagResponse) updates) =>
      super.copyWith((message) => updates(message as DeleteTagResponse))
          as DeleteTagResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use DeleteTagResponse() / DeleteTagResponse.new instead')
  static DeleteTagResponse create() => DeleteTagResponse._();
  static $pb.GeneratedMessage $_createMessage() => DeleteTagResponse._();
  @$core.override
  DeleteTagResponse createEmptyInstance() => DeleteTagResponse._();
  @$core.pragma('dart2js:noInline')
  static DeleteTagResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<DeleteTagResponse>(
          DeleteTagResponse.$_createMessage);
  static DeleteTagResponse? _defaultInstance;
}

class AddCustomerTagsRequest extends $pb.GeneratedMessage {
  factory AddCustomerTagsRequest({
    $core.String? storeId,
    $core.String? buyerId,
    $core.Iterable<$core.String>? tagNames,
  }) {
    final result = AddCustomerTagsRequest._();
    if (storeId != null) result.storeId = storeId;
    if (buyerId != null) result.buyerId = buyerId;
    if (tagNames != null) result.tagNames.addAll(tagNames);
    return result;
  }

  AddCustomerTagsRequest._();

  factory AddCustomerTagsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AddCustomerTagsRequest()..mergeFromBuffer(data, registry);
  factory AddCustomerTagsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AddCustomerTagsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AddCustomerTagsRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.crm.v1alpha1'),
      createEmptyInstance: AddCustomerTagsRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'storeId')
    ..aOS(2, _omitFieldNames ? '' : 'buyerId')
    ..pPS(3, _omitFieldNames ? '' : 'tagNames')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AddCustomerTagsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AddCustomerTagsRequest copyWith(
          void Function(AddCustomerTagsRequest) updates) =>
      super.copyWith((message) => updates(message as AddCustomerTagsRequest))
          as AddCustomerTagsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use AddCustomerTagsRequest() / AddCustomerTagsRequest.new instead')
  static AddCustomerTagsRequest create() => AddCustomerTagsRequest._();
  static $pb.GeneratedMessage $_createMessage() => AddCustomerTagsRequest._();
  @$core.override
  AddCustomerTagsRequest createEmptyInstance() => AddCustomerTagsRequest._();
  @$core.pragma('dart2js:noInline')
  static AddCustomerTagsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AddCustomerTagsRequest>(
          AddCustomerTagsRequest.$_createMessage);
  static AddCustomerTagsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storeId => $_getSZ(0);
  @$pb.TagNumber(1)
  set storeId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStoreId() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoreId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get buyerId => $_getSZ(1);
  @$pb.TagNumber(2)
  set buyerId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasBuyerId() => $_has(1);
  @$pb.TagNumber(2)
  void clearBuyerId() => $_clearField(2);

  /// 標籤名稱，1 到 20 個。
  @$pb.TagNumber(3)
  $pb.PbList<$core.String> get tagNames => $_getList(2);
}

class AddCustomerTagsResponse extends $pb.GeneratedMessage {
  factory AddCustomerTagsResponse({
    Customer? customer,
  }) {
    final result = AddCustomerTagsResponse._();
    if (customer != null) result.customer = customer;
    return result;
  }

  AddCustomerTagsResponse._();

  factory AddCustomerTagsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AddCustomerTagsResponse()..mergeFromBuffer(data, registry);
  factory AddCustomerTagsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AddCustomerTagsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AddCustomerTagsResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.crm.v1alpha1'),
      createEmptyInstance: AddCustomerTagsResponse.$_createMessage)
    ..aOM<Customer>(1, _omitFieldNames ? '' : 'customer',
        subBuilder: Customer.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AddCustomerTagsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AddCustomerTagsResponse copyWith(
          void Function(AddCustomerTagsResponse) updates) =>
      super.copyWith((message) => updates(message as AddCustomerTagsResponse))
          as AddCustomerTagsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use AddCustomerTagsResponse() / AddCustomerTagsResponse.new instead')
  static AddCustomerTagsResponse create() => AddCustomerTagsResponse._();
  static $pb.GeneratedMessage $_createMessage() => AddCustomerTagsResponse._();
  @$core.override
  AddCustomerTagsResponse createEmptyInstance() => AddCustomerTagsResponse._();
  @$core.pragma('dart2js:noInline')
  static AddCustomerTagsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AddCustomerTagsResponse>(
          AddCustomerTagsResponse.$_createMessage);
  static AddCustomerTagsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Customer get customer => $_getN(0);
  @$pb.TagNumber(1)
  set customer(Customer value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasCustomer() => $_has(0);
  @$pb.TagNumber(1)
  void clearCustomer() => $_clearField(1);
  @$pb.TagNumber(1)
  Customer ensureCustomer() => $_ensure(0);
}

class RemoveCustomerTagRequest extends $pb.GeneratedMessage {
  factory RemoveCustomerTagRequest({
    $core.String? storeId,
    $core.String? buyerId,
    $core.String? tagId,
  }) {
    final result = RemoveCustomerTagRequest._();
    if (storeId != null) result.storeId = storeId;
    if (buyerId != null) result.buyerId = buyerId;
    if (tagId != null) result.tagId = tagId;
    return result;
  }

  RemoveCustomerTagRequest._();

  factory RemoveCustomerTagRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RemoveCustomerTagRequest()..mergeFromBuffer(data, registry);
  factory RemoveCustomerTagRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RemoveCustomerTagRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RemoveCustomerTagRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.crm.v1alpha1'),
      createEmptyInstance: RemoveCustomerTagRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'storeId')
    ..aOS(2, _omitFieldNames ? '' : 'buyerId')
    ..aOS(3, _omitFieldNames ? '' : 'tagId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveCustomerTagRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveCustomerTagRequest copyWith(
          void Function(RemoveCustomerTagRequest) updates) =>
      super.copyWith((message) => updates(message as RemoveCustomerTagRequest))
          as RemoveCustomerTagRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use RemoveCustomerTagRequest() / RemoveCustomerTagRequest.new instead')
  static RemoveCustomerTagRequest create() => RemoveCustomerTagRequest._();
  static $pb.GeneratedMessage $_createMessage() => RemoveCustomerTagRequest._();
  @$core.override
  RemoveCustomerTagRequest createEmptyInstance() =>
      RemoveCustomerTagRequest._();
  @$core.pragma('dart2js:noInline')
  static RemoveCustomerTagRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RemoveCustomerTagRequest>(
          RemoveCustomerTagRequest.$_createMessage);
  static RemoveCustomerTagRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storeId => $_getSZ(0);
  @$pb.TagNumber(1)
  set storeId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStoreId() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoreId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get buyerId => $_getSZ(1);
  @$pb.TagNumber(2)
  set buyerId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasBuyerId() => $_has(1);
  @$pb.TagNumber(2)
  void clearBuyerId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get tagId => $_getSZ(2);
  @$pb.TagNumber(3)
  set tagId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasTagId() => $_has(2);
  @$pb.TagNumber(3)
  void clearTagId() => $_clearField(3);
}

class RemoveCustomerTagResponse extends $pb.GeneratedMessage {
  factory RemoveCustomerTagResponse({
    Customer? customer,
  }) {
    final result = RemoveCustomerTagResponse._();
    if (customer != null) result.customer = customer;
    return result;
  }

  RemoveCustomerTagResponse._();

  factory RemoveCustomerTagResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RemoveCustomerTagResponse()..mergeFromBuffer(data, registry);
  factory RemoveCustomerTagResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RemoveCustomerTagResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RemoveCustomerTagResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.crm.v1alpha1'),
      createEmptyInstance: RemoveCustomerTagResponse.$_createMessage)
    ..aOM<Customer>(1, _omitFieldNames ? '' : 'customer',
        subBuilder: Customer.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveCustomerTagResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveCustomerTagResponse copyWith(
          void Function(RemoveCustomerTagResponse) updates) =>
      super.copyWith((message) => updates(message as RemoveCustomerTagResponse))
          as RemoveCustomerTagResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use RemoveCustomerTagResponse() / RemoveCustomerTagResponse.new instead')
  static RemoveCustomerTagResponse create() => RemoveCustomerTagResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      RemoveCustomerTagResponse._();
  @$core.override
  RemoveCustomerTagResponse createEmptyInstance() =>
      RemoveCustomerTagResponse._();
  @$core.pragma('dart2js:noInline')
  static RemoveCustomerTagResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RemoveCustomerTagResponse>(
          RemoveCustomerTagResponse.$_createMessage);
  static RemoveCustomerTagResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Customer get customer => $_getN(0);
  @$pb.TagNumber(1)
  set customer(Customer value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasCustomer() => $_has(0);
  @$pb.TagNumber(1)
  void clearCustomer() => $_clearField(1);
  @$pb.TagNumber(1)
  Customer ensureCustomer() => $_ensure(0);
}

/// CrmAppService 是店家的客戶與標籤。
class CrmAppServiceApi {
  final $pb.RpcClient _client;

  CrmAppServiceApi(this._client);

  /// ListCustomers 列出店家的客戶。type 只列這個分類，tag_id 只列貼了這個標籤的客戶（單一標籤），都不設是全部；明確送出 type 卻是 CUSTOMER_TYPE_UNSPECIFIED 回 invalid_argument。
  /// sort 預設（CUSTOMER_SORT_UNSPECIFIED）是 CUSTOMER_SORT_FIRST_ORDER_TIME，descending 為真時大的在前（預設小的在前）；同值以買家 ID 排序，分頁穩定。
  /// 不回總數，結束只看 next_page_token；page_token 只能用在同一家店與同一組篩選與排序。tag_id 不是這家店的標籤時結果是空的一頁。
  $async.Future<ListCustomersResponse> listCustomers(
          $pb.ClientContext? ctx, ListCustomersRequest request) =>
      _client.invoke<ListCustomersResponse>(ctx, 'CrmAppService',
          'ListCustomers', request, ListCustomersResponse());

  /// GetCustomer 讀一位客戶與他的標籤。客戶只在首筆完成訂單時建立；這家店沒有這位客戶（包含買家已刪除）回 not_found。
  $async.Future<GetCustomerResponse> getCustomer(
          $pb.ClientContext? ctx, GetCustomerRequest request) =>
      _client.invoke<GetCustomerResponse>(
          ctx, 'CrmAppService', 'GetCustomer', request, GetCustomerResponse());

  /// UpdateCustomer 修改客戶的分類（update_mask 只能列 type；值 NORMAL 或 VIP，UNSPECIFIED 回 invalid_argument）。update_mask 沒有列 type 回 invalid_argument。
  /// 這家店沒有這位客戶回 not_found。
  $async.Future<UpdateCustomerResponse> updateCustomer(
          $pb.ClientContext? ctx, UpdateCustomerRequest request) =>
      _client.invoke<UpdateCustomerResponse>(ctx, 'CrmAppService',
          'UpdateCustomer', request, UpdateCustomerResponse());

  /// CreateTag 建立標籤，名稱去掉前後空白後 1 到 50 個字元。同店已有同名標籤回 already_exists 與 CRM_TAG_NAME_TAKEN；每家店最多 200 個標籤，超過回 failed_precondition 與 CRM_TAG_LIMIT。
  $async.Future<CreateTagResponse> createTag(
          $pb.ClientContext? ctx, CreateTagRequest request) =>
      _client.invoke<CreateTagResponse>(
          ctx, 'CrmAppService', 'CreateTag', request, CreateTagResponse());

  /// ListTags 列出店家全部的標籤，名稱升序，不分頁（每家店最多 200 個）。
  $async.Future<ListTagsResponse> listTags(
          $pb.ClientContext? ctx, ListTagsRequest request) =>
      _client.invoke<ListTagsResponse>(
          ctx, 'CrmAppService', 'ListTags', request, ListTagsResponse());

  /// UpdateTag 改標籤名稱（update_mask 只能列 name，沒有列 name 回 invalid_argument）。撞名回 already_exists 與 CRM_TAG_NAME_TAKEN；標籤不存在回 not_found。
  $async.Future<UpdateTagResponse> updateTag(
          $pb.ClientContext? ctx, UpdateTagRequest request) =>
      _client.invoke<UpdateTagResponse>(
          ctx, 'CrmAppService', 'UpdateTag', request, UpdateTagResponse());

  /// DeleteTag 刪標籤，它在客戶身上的關聯一併刪除。標籤不存在回 not_found。
  $async.Future<DeleteTagResponse> deleteTag(
          $pb.ClientContext? ctx, DeleteTagRequest request) =>
      _client.invoke<DeleteTagResponse>(
          ctx, 'CrmAppService', 'DeleteTag', request, DeleteTagResponse());

  /// AddCustomerTags 替客戶貼上一批標籤，名稱不存在的先建立，已貼的不重複；一次 1 到 20 個名稱。整批原子：任何一步失敗都沒有寫入。每位客戶最多 20 個標籤、每家店最多 200 個標籤，
  /// 超過回 failed_precondition 與 CRM_TAG_LIMIT。這家店沒有這位客戶回 not_found。回傳貼完之後的客戶。
  $async.Future<AddCustomerTagsResponse> addCustomerTags(
          $pb.ClientContext? ctx, AddCustomerTagsRequest request) =>
      _client.invoke<AddCustomerTagsResponse>(ctx, 'CrmAppService',
          'AddCustomerTags', request, AddCustomerTagsResponse());

  /// RemoveCustomerTag 移除客戶身上的一個標籤，客戶本來就沒有這個標籤時也成功。這家店沒有這位客戶回 not_found。回傳移除之後的客戶。
  $async.Future<RemoveCustomerTagResponse> removeCustomerTag(
          $pb.ClientContext? ctx, RemoveCustomerTagRequest request) =>
      _client.invoke<RemoveCustomerTagResponse>(ctx, 'CrmAppService',
          'RemoveCustomerTag', request, RemoveCustomerTagResponse());
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
