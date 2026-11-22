// This is a generated file - do not edit.
//
// Generated from hawkr/crm/v1alpha1/crm.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports
// ignore_for_file: unused_import

import 'dart:convert' as $convert;
import 'dart:core' as $core;
import 'dart:typed_data' as $typed_data;

import 'package:protobuf/well_known_types/google/protobuf/field_mask.pbjson.dart'
    as $2;
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pbjson.dart'
    as $1;

import '../../common/v1alpha1/image.pbjson.dart' as $0;

@$core.Deprecated('Use customerTypeDescriptor instead')
const CustomerType$json = {
  '1': 'CustomerType',
  '2': [
    {'1': 'CUSTOMER_TYPE_UNSPECIFIED', '2': 0},
    {'1': 'CUSTOMER_TYPE_NORMAL', '2': 1},
    {'1': 'CUSTOMER_TYPE_VIP', '2': 2},
  ],
};

/// Descriptor for `CustomerType`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List customerTypeDescriptor = $convert.base64Decode(
    'CgxDdXN0b21lclR5cGUSHQoZQ1VTVE9NRVJfVFlQRV9VTlNQRUNJRklFRBAAEhgKFENVU1RPTU'
    'VSX1RZUEVfTk9STUFMEAESFQoRQ1VTVE9NRVJfVFlQRV9WSVAQAg==');

@$core.Deprecated('Use customerSortDescriptor instead')
const CustomerSort$json = {
  '1': 'CustomerSort',
  '2': [
    {'1': 'CUSTOMER_SORT_UNSPECIFIED', '2': 0},
    {'1': 'CUSTOMER_SORT_FIRST_ORDER_TIME', '2': 1},
    {'1': 'CUSTOMER_SORT_TOTAL_SPENT', '2': 2},
    {'1': 'CUSTOMER_SORT_ORDER_COUNT', '2': 3},
  ],
};

/// Descriptor for `CustomerSort`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List customerSortDescriptor = $convert.base64Decode(
    'CgxDdXN0b21lclNvcnQSHQoZQ1VTVE9NRVJfU09SVF9VTlNQRUNJRklFRBAAEiIKHkNVU1RPTU'
    'VSX1NPUlRfRklSU1RfT1JERVJfVElNRRABEh0KGUNVU1RPTUVSX1NPUlRfVE9UQUxfU1BFTlQQ'
    'AhIdChlDVVNUT01FUl9TT1JUX09SREVSX0NPVU5UEAM=');

@$core.Deprecated('Use tagDescriptor instead')
const Tag$json = {
  '1': 'Tag',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'id'},
    {'1': 'name', '3': 2, '4': 1, '5': 9, '10': 'name'},
  ],
};

/// Descriptor for `Tag`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List tagDescriptor = $convert.base64Decode(
    'CgNUYWcSEwoCaWQYASABKAlCA+BBCFICaWQSEgoEbmFtZRgCIAEoCVIEbmFtZQ==');

@$core.Deprecated('Use customerDisplayDescriptor instead')
const CustomerDisplay$json = {
  '1': 'CustomerDisplay',
  '2': [
    {
      '1': 'nickname',
      '3': 1,
      '4': 1,
      '5': 9,
      '8': {},
      '9': 0,
      '10': 'nickname',
      '17': true
    },
    {
      '1': 'avatar',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.Image',
      '8': {},
      '10': 'avatar'
    },
    {'1': 'deleted', '3': 3, '4': 1, '5': 8, '8': {}, '10': 'deleted'},
  ],
  '8': [
    {'1': '_nickname'},
  ],
};

/// Descriptor for `CustomerDisplay`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List customerDisplayDescriptor = $convert.base64Decode(
    'Cg9DdXN0b21lckRpc3BsYXkSJAoIbmlja25hbWUYASABKAlCA+BBA0gAUghuaWNrbmFtZYgBAR'
    'I5CgZhdmF0YXIYAiABKAsyHC5oYXdrci5jb21tb24udjFhbHBoYTEuSW1hZ2VCA+BBA1IGYXZh'
    'dGFyEh0KB2RlbGV0ZWQYAyABKAhCA+BBA1IHZGVsZXRlZEILCglfbmlja25hbWU=');

@$core.Deprecated('Use customerDescriptor instead')
const Customer$json = {
  '1': 'Customer',
  '2': [
    {'1': 'buyer_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'buyerId'},
    {
      '1': 'display',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.hawkr.crm.v1alpha1.CustomerDisplay',
      '8': {},
      '10': 'display'
    },
    {
      '1': 'type',
      '3': 3,
      '4': 1,
      '5': 14,
      '6': '.hawkr.crm.v1alpha1.CustomerType',
      '10': 'type'
    },
    {'1': 'total_spent', '3': 4, '4': 1, '5': 3, '8': {}, '10': 'totalSpent'},
    {'1': 'order_count', '3': 5, '4': 1, '5': 5, '8': {}, '10': 'orderCount'},
    {
      '1': 'first_order_time',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'firstOrderTime'
    },
    {
      '1': 'tags',
      '3': 7,
      '4': 3,
      '5': 11,
      '6': '.hawkr.crm.v1alpha1.Tag',
      '8': {},
      '10': 'tags'
    },
  ],
};

/// Descriptor for `Customer`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List customerDescriptor = $convert.base64Decode(
    'CghDdXN0b21lchIeCghidXllcl9pZBgBIAEoCUID4EEIUgdidXllcklkEkIKB2Rpc3BsYXkYAi'
    'ABKAsyIy5oYXdrci5jcm0udjFhbHBoYTEuQ3VzdG9tZXJEaXNwbGF5QgPgQQNSB2Rpc3BsYXkS'
    'NAoEdHlwZRgDIAEoDjIgLmhhd2tyLmNybS52MWFscGhhMS5DdXN0b21lclR5cGVSBHR5cGUSJA'
    'oLdG90YWxfc3BlbnQYBCABKANCA+BBA1IKdG90YWxTcGVudBIkCgtvcmRlcl9jb3VudBgFIAEo'
    'BUID4EEDUgpvcmRlckNvdW50EkkKEGZpcnN0X29yZGVyX3RpbWUYBiABKAsyGi5nb29nbGUucH'
    'JvdG9idWYuVGltZXN0YW1wQgPgQQNSDmZpcnN0T3JkZXJUaW1lEjAKBHRhZ3MYByADKAsyFy5o'
    'YXdrci5jcm0udjFhbHBoYTEuVGFnQgPgQQNSBHRhZ3M=');

@$core.Deprecated('Use listCustomersRequestDescriptor instead')
const ListCustomersRequest$json = {
  '1': 'ListCustomersRequest',
  '2': [
    {'1': 'store_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'storeId'},
    {
      '1': 'type',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.hawkr.crm.v1alpha1.CustomerType',
      '9': 0,
      '10': 'type',
      '17': true
    },
    {'1': 'tag_id', '3': 3, '4': 1, '5': 9, '9': 1, '10': 'tagId', '17': true},
    {
      '1': 'sort',
      '3': 4,
      '4': 1,
      '5': 14,
      '6': '.hawkr.crm.v1alpha1.CustomerSort',
      '10': 'sort'
    },
    {'1': 'descending', '3': 5, '4': 1, '5': 8, '10': 'descending'},
    {'1': 'page_size', '3': 6, '4': 1, '5': 5, '10': 'pageSize'},
    {'1': 'page_token', '3': 7, '4': 1, '5': 9, '10': 'pageToken'},
  ],
  '8': [
    {'1': '_type'},
    {'1': '_tag_id'},
  ],
};

/// Descriptor for `ListCustomersRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listCustomersRequestDescriptor = $convert.base64Decode(
    'ChRMaXN0Q3VzdG9tZXJzUmVxdWVzdBIeCghzdG9yZV9pZBgBIAEoCUID4EECUgdzdG9yZUlkEj'
    'kKBHR5cGUYAiABKA4yIC5oYXdrci5jcm0udjFhbHBoYTEuQ3VzdG9tZXJUeXBlSABSBHR5cGWI'
    'AQESGgoGdGFnX2lkGAMgASgJSAFSBXRhZ0lkiAEBEjQKBHNvcnQYBCABKA4yIC5oYXdrci5jcm'
    '0udjFhbHBoYTEuQ3VzdG9tZXJTb3J0UgRzb3J0Eh4KCmRlc2NlbmRpbmcYBSABKAhSCmRlc2Nl'
    'bmRpbmcSGwoJcGFnZV9zaXplGAYgASgFUghwYWdlU2l6ZRIdCgpwYWdlX3Rva2VuGAcgASgJUg'
    'lwYWdlVG9rZW5CBwoFX3R5cGVCCQoHX3RhZ19pZA==');

@$core.Deprecated('Use listCustomersResponseDescriptor instead')
const ListCustomersResponse$json = {
  '1': 'ListCustomersResponse',
  '2': [
    {
      '1': 'customers',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.crm.v1alpha1.Customer',
      '10': 'customers'
    },
    {'1': 'next_page_token', '3': 2, '4': 1, '5': 9, '10': 'nextPageToken'},
  ],
};

/// Descriptor for `ListCustomersResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listCustomersResponseDescriptor = $convert.base64Decode(
    'ChVMaXN0Q3VzdG9tZXJzUmVzcG9uc2USOgoJY3VzdG9tZXJzGAEgAygLMhwuaGF3a3IuY3JtLn'
    'YxYWxwaGExLkN1c3RvbWVyUgljdXN0b21lcnMSJgoPbmV4dF9wYWdlX3Rva2VuGAIgASgJUg1u'
    'ZXh0UGFnZVRva2Vu');

@$core.Deprecated('Use getCustomerRequestDescriptor instead')
const GetCustomerRequest$json = {
  '1': 'GetCustomerRequest',
  '2': [
    {'1': 'store_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'storeId'},
    {'1': 'buyer_id', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'buyerId'},
  ],
};

/// Descriptor for `GetCustomerRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getCustomerRequestDescriptor = $convert.base64Decode(
    'ChJHZXRDdXN0b21lclJlcXVlc3QSHgoIc3RvcmVfaWQYASABKAlCA+BBAlIHc3RvcmVJZBIeCg'
    'hidXllcl9pZBgCIAEoCUID4EECUgdidXllcklk');

@$core.Deprecated('Use getCustomerResponseDescriptor instead')
const GetCustomerResponse$json = {
  '1': 'GetCustomerResponse',
  '2': [
    {
      '1': 'customer',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.crm.v1alpha1.Customer',
      '10': 'customer'
    },
  ],
};

/// Descriptor for `GetCustomerResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getCustomerResponseDescriptor = $convert.base64Decode(
    'ChNHZXRDdXN0b21lclJlc3BvbnNlEjgKCGN1c3RvbWVyGAEgASgLMhwuaGF3a3IuY3JtLnYxYW'
    'xwaGExLkN1c3RvbWVyUghjdXN0b21lcg==');

@$core.Deprecated('Use updateCustomerRequestDescriptor instead')
const UpdateCustomerRequest$json = {
  '1': 'UpdateCustomerRequest',
  '2': [
    {'1': 'store_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'storeId'},
    {
      '1': 'customer',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.hawkr.crm.v1alpha1.Customer',
      '8': {},
      '10': 'customer'
    },
    {
      '1': 'update_mask',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.FieldMask',
      '8': {},
      '10': 'updateMask'
    },
  ],
};

/// Descriptor for `UpdateCustomerRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateCustomerRequestDescriptor = $convert.base64Decode(
    'ChVVcGRhdGVDdXN0b21lclJlcXVlc3QSHgoIc3RvcmVfaWQYASABKAlCA+BBAlIHc3RvcmVJZB'
    'I9CghjdXN0b21lchgCIAEoCzIcLmhhd2tyLmNybS52MWFscGhhMS5DdXN0b21lckID4EECUghj'
    'dXN0b21lchJACgt1cGRhdGVfbWFzaxgDIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5GaWVsZE1hc2'
    'tCA+BBAlIKdXBkYXRlTWFzaw==');

@$core.Deprecated('Use updateCustomerResponseDescriptor instead')
const UpdateCustomerResponse$json = {
  '1': 'UpdateCustomerResponse',
  '2': [
    {
      '1': 'customer',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.crm.v1alpha1.Customer',
      '10': 'customer'
    },
  ],
};

/// Descriptor for `UpdateCustomerResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateCustomerResponseDescriptor =
    $convert.base64Decode(
        'ChZVcGRhdGVDdXN0b21lclJlc3BvbnNlEjgKCGN1c3RvbWVyGAEgASgLMhwuaGF3a3IuY3JtLn'
        'YxYWxwaGExLkN1c3RvbWVyUghjdXN0b21lcg==');

@$core.Deprecated('Use createTagRequestDescriptor instead')
const CreateTagRequest$json = {
  '1': 'CreateTagRequest',
  '2': [
    {'1': 'store_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'storeId'},
    {'1': 'name', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'name'},
  ],
};

/// Descriptor for `CreateTagRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createTagRequestDescriptor = $convert.base64Decode(
    'ChBDcmVhdGVUYWdSZXF1ZXN0Eh4KCHN0b3JlX2lkGAEgASgJQgPgQQJSB3N0b3JlSWQSFwoEbm'
    'FtZRgCIAEoCUID4EECUgRuYW1l');

@$core.Deprecated('Use createTagResponseDescriptor instead')
const CreateTagResponse$json = {
  '1': 'CreateTagResponse',
  '2': [
    {
      '1': 'tag',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.crm.v1alpha1.Tag',
      '10': 'tag'
    },
  ],
};

/// Descriptor for `CreateTagResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createTagResponseDescriptor = $convert.base64Decode(
    'ChFDcmVhdGVUYWdSZXNwb25zZRIpCgN0YWcYASABKAsyFy5oYXdrci5jcm0udjFhbHBoYTEuVG'
    'FnUgN0YWc=');

@$core.Deprecated('Use listTagsRequestDescriptor instead')
const ListTagsRequest$json = {
  '1': 'ListTagsRequest',
  '2': [
    {'1': 'store_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'storeId'},
  ],
};

/// Descriptor for `ListTagsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listTagsRequestDescriptor = $convert.base64Decode(
    'Cg9MaXN0VGFnc1JlcXVlc3QSHgoIc3RvcmVfaWQYASABKAlCA+BBAlIHc3RvcmVJZA==');

@$core.Deprecated('Use listTagsResponseDescriptor instead')
const ListTagsResponse$json = {
  '1': 'ListTagsResponse',
  '2': [
    {
      '1': 'tags',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.crm.v1alpha1.Tag',
      '10': 'tags'
    },
  ],
};

/// Descriptor for `ListTagsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listTagsResponseDescriptor = $convert.base64Decode(
    'ChBMaXN0VGFnc1Jlc3BvbnNlEisKBHRhZ3MYASADKAsyFy5oYXdrci5jcm0udjFhbHBoYTEuVG'
    'FnUgR0YWdz');

@$core.Deprecated('Use updateTagRequestDescriptor instead')
const UpdateTagRequest$json = {
  '1': 'UpdateTagRequest',
  '2': [
    {'1': 'store_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'storeId'},
    {
      '1': 'tag',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.hawkr.crm.v1alpha1.Tag',
      '8': {},
      '10': 'tag'
    },
    {
      '1': 'update_mask',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.FieldMask',
      '8': {},
      '10': 'updateMask'
    },
  ],
};

/// Descriptor for `UpdateTagRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateTagRequestDescriptor = $convert.base64Decode(
    'ChBVcGRhdGVUYWdSZXF1ZXN0Eh4KCHN0b3JlX2lkGAEgASgJQgPgQQJSB3N0b3JlSWQSLgoDdG'
    'FnGAIgASgLMhcuaGF3a3IuY3JtLnYxYWxwaGExLlRhZ0ID4EECUgN0YWcSQAoLdXBkYXRlX21h'
    'c2sYAyABKAsyGi5nb29nbGUucHJvdG9idWYuRmllbGRNYXNrQgPgQQJSCnVwZGF0ZU1hc2s=');

@$core.Deprecated('Use updateTagResponseDescriptor instead')
const UpdateTagResponse$json = {
  '1': 'UpdateTagResponse',
  '2': [
    {
      '1': 'tag',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.crm.v1alpha1.Tag',
      '10': 'tag'
    },
  ],
};

/// Descriptor for `UpdateTagResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateTagResponseDescriptor = $convert.base64Decode(
    'ChFVcGRhdGVUYWdSZXNwb25zZRIpCgN0YWcYASABKAsyFy5oYXdrci5jcm0udjFhbHBoYTEuVG'
    'FnUgN0YWc=');

@$core.Deprecated('Use deleteTagRequestDescriptor instead')
const DeleteTagRequest$json = {
  '1': 'DeleteTagRequest',
  '2': [
    {'1': 'store_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'storeId'},
    {'1': 'tag_id', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'tagId'},
  ],
};

/// Descriptor for `DeleteTagRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteTagRequestDescriptor = $convert.base64Decode(
    'ChBEZWxldGVUYWdSZXF1ZXN0Eh4KCHN0b3JlX2lkGAEgASgJQgPgQQJSB3N0b3JlSWQSGgoGdG'
    'FnX2lkGAIgASgJQgPgQQJSBXRhZ0lk');

@$core.Deprecated('Use deleteTagResponseDescriptor instead')
const DeleteTagResponse$json = {
  '1': 'DeleteTagResponse',
};

/// Descriptor for `DeleteTagResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteTagResponseDescriptor =
    $convert.base64Decode('ChFEZWxldGVUYWdSZXNwb25zZQ==');

@$core.Deprecated('Use addCustomerTagsRequestDescriptor instead')
const AddCustomerTagsRequest$json = {
  '1': 'AddCustomerTagsRequest',
  '2': [
    {'1': 'store_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'storeId'},
    {'1': 'buyer_id', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'buyerId'},
    {'1': 'tag_names', '3': 3, '4': 3, '5': 9, '8': {}, '10': 'tagNames'},
  ],
};

/// Descriptor for `AddCustomerTagsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List addCustomerTagsRequestDescriptor = $convert.base64Decode(
    'ChZBZGRDdXN0b21lclRhZ3NSZXF1ZXN0Eh4KCHN0b3JlX2lkGAEgASgJQgPgQQJSB3N0b3JlSW'
    'QSHgoIYnV5ZXJfaWQYAiABKAlCA+BBAlIHYnV5ZXJJZBIgCgl0YWdfbmFtZXMYAyADKAlCA+BB'
    'AlIIdGFnTmFtZXM=');

@$core.Deprecated('Use addCustomerTagsResponseDescriptor instead')
const AddCustomerTagsResponse$json = {
  '1': 'AddCustomerTagsResponse',
  '2': [
    {
      '1': 'customer',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.crm.v1alpha1.Customer',
      '10': 'customer'
    },
  ],
};

/// Descriptor for `AddCustomerTagsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List addCustomerTagsResponseDescriptor =
    $convert.base64Decode(
        'ChdBZGRDdXN0b21lclRhZ3NSZXNwb25zZRI4CghjdXN0b21lchgBIAEoCzIcLmhhd2tyLmNybS'
        '52MWFscGhhMS5DdXN0b21lclIIY3VzdG9tZXI=');

@$core.Deprecated('Use removeCustomerTagRequestDescriptor instead')
const RemoveCustomerTagRequest$json = {
  '1': 'RemoveCustomerTagRequest',
  '2': [
    {'1': 'store_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'storeId'},
    {'1': 'buyer_id', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'buyerId'},
    {'1': 'tag_id', '3': 3, '4': 1, '5': 9, '8': {}, '10': 'tagId'},
  ],
};

/// Descriptor for `RemoveCustomerTagRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List removeCustomerTagRequestDescriptor = $convert.base64Decode(
    'ChhSZW1vdmVDdXN0b21lclRhZ1JlcXVlc3QSHgoIc3RvcmVfaWQYASABKAlCA+BBAlIHc3Rvcm'
    'VJZBIeCghidXllcl9pZBgCIAEoCUID4EECUgdidXllcklkEhoKBnRhZ19pZBgDIAEoCUID4EEC'
    'UgV0YWdJZA==');

@$core.Deprecated('Use removeCustomerTagResponseDescriptor instead')
const RemoveCustomerTagResponse$json = {
  '1': 'RemoveCustomerTagResponse',
  '2': [
    {
      '1': 'customer',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.crm.v1alpha1.Customer',
      '10': 'customer'
    },
  ],
};

/// Descriptor for `RemoveCustomerTagResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List removeCustomerTagResponseDescriptor =
    $convert.base64Decode(
        'ChlSZW1vdmVDdXN0b21lclRhZ1Jlc3BvbnNlEjgKCGN1c3RvbWVyGAEgASgLMhwuaGF3a3IuY3'
        'JtLnYxYWxwaGExLkN1c3RvbWVyUghjdXN0b21lcg==');

const $core.Map<$core.String, $core.dynamic> CrmAppServiceBase$json = {
  '1': 'CrmAppService',
  '2': [
    {
      '1': 'ListCustomers',
      '2': '.hawkr.crm.v1alpha1.ListCustomersRequest',
      '3': '.hawkr.crm.v1alpha1.ListCustomersResponse',
      '4': {'34': 1},
    },
    {
      '1': 'GetCustomer',
      '2': '.hawkr.crm.v1alpha1.GetCustomerRequest',
      '3': '.hawkr.crm.v1alpha1.GetCustomerResponse',
      '4': {'34': 1},
    },
    {
      '1': 'UpdateCustomer',
      '2': '.hawkr.crm.v1alpha1.UpdateCustomerRequest',
      '3': '.hawkr.crm.v1alpha1.UpdateCustomerResponse',
      '4': {'34': 2},
    },
    {
      '1': 'CreateTag',
      '2': '.hawkr.crm.v1alpha1.CreateTagRequest',
      '3': '.hawkr.crm.v1alpha1.CreateTagResponse'
    },
    {
      '1': 'ListTags',
      '2': '.hawkr.crm.v1alpha1.ListTagsRequest',
      '3': '.hawkr.crm.v1alpha1.ListTagsResponse',
      '4': {'34': 1},
    },
    {
      '1': 'UpdateTag',
      '2': '.hawkr.crm.v1alpha1.UpdateTagRequest',
      '3': '.hawkr.crm.v1alpha1.UpdateTagResponse',
      '4': {'34': 2},
    },
    {
      '1': 'DeleteTag',
      '2': '.hawkr.crm.v1alpha1.DeleteTagRequest',
      '3': '.hawkr.crm.v1alpha1.DeleteTagResponse',
      '4': {'34': 2},
    },
    {
      '1': 'AddCustomerTags',
      '2': '.hawkr.crm.v1alpha1.AddCustomerTagsRequest',
      '3': '.hawkr.crm.v1alpha1.AddCustomerTagsResponse'
    },
    {
      '1': 'RemoveCustomerTag',
      '2': '.hawkr.crm.v1alpha1.RemoveCustomerTagRequest',
      '3': '.hawkr.crm.v1alpha1.RemoveCustomerTagResponse',
      '4': {'34': 2},
    },
  ],
};

@$core.Deprecated('Use crmAppServiceDescriptor instead')
const $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
    CrmAppServiceBase$messageJson = {
  '.hawkr.crm.v1alpha1.ListCustomersRequest': ListCustomersRequest$json,
  '.hawkr.crm.v1alpha1.ListCustomersResponse': ListCustomersResponse$json,
  '.hawkr.crm.v1alpha1.Customer': Customer$json,
  '.hawkr.crm.v1alpha1.CustomerDisplay': CustomerDisplay$json,
  '.hawkr.common.v1alpha1.Image': $0.Image$json,
  '.google.protobuf.Timestamp': $1.Timestamp$json,
  '.hawkr.crm.v1alpha1.Tag': Tag$json,
  '.hawkr.crm.v1alpha1.GetCustomerRequest': GetCustomerRequest$json,
  '.hawkr.crm.v1alpha1.GetCustomerResponse': GetCustomerResponse$json,
  '.hawkr.crm.v1alpha1.UpdateCustomerRequest': UpdateCustomerRequest$json,
  '.google.protobuf.FieldMask': $2.FieldMask$json,
  '.hawkr.crm.v1alpha1.UpdateCustomerResponse': UpdateCustomerResponse$json,
  '.hawkr.crm.v1alpha1.CreateTagRequest': CreateTagRequest$json,
  '.hawkr.crm.v1alpha1.CreateTagResponse': CreateTagResponse$json,
  '.hawkr.crm.v1alpha1.ListTagsRequest': ListTagsRequest$json,
  '.hawkr.crm.v1alpha1.ListTagsResponse': ListTagsResponse$json,
  '.hawkr.crm.v1alpha1.UpdateTagRequest': UpdateTagRequest$json,
  '.hawkr.crm.v1alpha1.UpdateTagResponse': UpdateTagResponse$json,
  '.hawkr.crm.v1alpha1.DeleteTagRequest': DeleteTagRequest$json,
  '.hawkr.crm.v1alpha1.DeleteTagResponse': DeleteTagResponse$json,
  '.hawkr.crm.v1alpha1.AddCustomerTagsRequest': AddCustomerTagsRequest$json,
  '.hawkr.crm.v1alpha1.AddCustomerTagsResponse': AddCustomerTagsResponse$json,
  '.hawkr.crm.v1alpha1.RemoveCustomerTagRequest': RemoveCustomerTagRequest$json,
  '.hawkr.crm.v1alpha1.RemoveCustomerTagResponse':
      RemoveCustomerTagResponse$json,
};

/// Descriptor for `CrmAppService`. Decode as a `google.protobuf.ServiceDescriptorProto`.
final $typed_data.Uint8List crmAppServiceDescriptor = $convert.base64Decode(
    'Cg1Dcm1BcHBTZXJ2aWNlEmkKDUxpc3RDdXN0b21lcnMSKC5oYXdrci5jcm0udjFhbHBoYTEuTG'
    'lzdEN1c3RvbWVyc1JlcXVlc3QaKS5oYXdrci5jcm0udjFhbHBoYTEuTGlzdEN1c3RvbWVyc1Jl'
    'c3BvbnNlIgOQAgESYwoLR2V0Q3VzdG9tZXISJi5oYXdrci5jcm0udjFhbHBoYTEuR2V0Q3VzdG'
    '9tZXJSZXF1ZXN0GicuaGF3a3IuY3JtLnYxYWxwaGExLkdldEN1c3RvbWVyUmVzcG9uc2UiA5AC'
    'ARJsCg5VcGRhdGVDdXN0b21lchIpLmhhd2tyLmNybS52MWFscGhhMS5VcGRhdGVDdXN0b21lcl'
    'JlcXVlc3QaKi5oYXdrci5jcm0udjFhbHBoYTEuVXBkYXRlQ3VzdG9tZXJSZXNwb25zZSIDkAIC'
    'ElgKCUNyZWF0ZVRhZxIkLmhhd2tyLmNybS52MWFscGhhMS5DcmVhdGVUYWdSZXF1ZXN0GiUuaG'
    'F3a3IuY3JtLnYxYWxwaGExLkNyZWF0ZVRhZ1Jlc3BvbnNlEloKCExpc3RUYWdzEiMuaGF3a3Iu'
    'Y3JtLnYxYWxwaGExLkxpc3RUYWdzUmVxdWVzdBokLmhhd2tyLmNybS52MWFscGhhMS5MaXN0VG'
    'Fnc1Jlc3BvbnNlIgOQAgESXQoJVXBkYXRlVGFnEiQuaGF3a3IuY3JtLnYxYWxwaGExLlVwZGF0'
    'ZVRhZ1JlcXVlc3QaJS5oYXdrci5jcm0udjFhbHBoYTEuVXBkYXRlVGFnUmVzcG9uc2UiA5ACAh'
    'JdCglEZWxldGVUYWcSJC5oYXdrci5jcm0udjFhbHBoYTEuRGVsZXRlVGFnUmVxdWVzdBolLmhh'
    'd2tyLmNybS52MWFscGhhMS5EZWxldGVUYWdSZXNwb25zZSIDkAICEmoKD0FkZEN1c3RvbWVyVG'
    'FncxIqLmhhd2tyLmNybS52MWFscGhhMS5BZGRDdXN0b21lclRhZ3NSZXF1ZXN0GisuaGF3a3Iu'
    'Y3JtLnYxYWxwaGExLkFkZEN1c3RvbWVyVGFnc1Jlc3BvbnNlEnUKEVJlbW92ZUN1c3RvbWVyVG'
    'FnEiwuaGF3a3IuY3JtLnYxYWxwaGExLlJlbW92ZUN1c3RvbWVyVGFnUmVxdWVzdBotLmhhd2ty'
    'LmNybS52MWFscGhhMS5SZW1vdmVDdXN0b21lclRhZ1Jlc3BvbnNlIgOQAgI=');
