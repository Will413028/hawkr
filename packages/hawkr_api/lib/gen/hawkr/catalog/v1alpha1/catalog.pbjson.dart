// This is a generated file - do not edit.
//
// Generated from hawkr/catalog/v1alpha1/catalog.proto.

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

@$core.Deprecated('Use productInputDescriptor instead')
const ProductInput$json = {
  '1': 'ProductInput',
  '2': [
    {'1': 'title', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'title'},
    {'1': 'category_id', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'categoryId'},
    {'1': 'hashtag_inputs', '3': 3, '4': 3, '5': 9, '10': 'hashtagInputs'},
    {
      '1': 'gallery',
      '3': 4,
      '4': 3,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.ImageInput',
      '8': {},
      '10': 'gallery'
    },
    {
      '1': 'description',
      '3': 5,
      '4': 3,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.DescriptionBlock',
      '8': {},
      '10': 'description'
    },
    {
      '1': 'axes',
      '3': 6,
      '4': 3,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.OptionAxisInput',
      '8': {},
      '10': 'axes'
    },
    {
      '1': 'skus',
      '3': 7,
      '4': 3,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.SkuInput',
      '8': {},
      '10': 'skus'
    },
  ],
};

/// Descriptor for `ProductInput`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List productInputDescriptor = $convert.base64Decode(
    'CgxQcm9kdWN0SW5wdXQSHwoFdGl0bGUYASABKAlCCeBBApK1GAIQZFIFdGl0bGUSJAoLY2F0ZW'
    'dvcnlfaWQYAiABKAlCA+BBAlIKY2F0ZWdvcnlJZBIlCg5oYXNodGFnX2lucHV0cxgDIAMoCVIN'
    'aGFzaHRhZ0lucHV0cxJECgdnYWxsZXJ5GAQgAygLMiIuaGF3a3IuY2F0YWxvZy52MWFscGhhMS'
    '5JbWFnZUlucHV0QgaStRgCCAlSB2dhbGxlcnkSVwoLZGVzY3JpcHRpb24YBSADKAsyKC5oYXdr'
    'ci5jYXRhbG9nLnYxYWxwaGExLkRlc2NyaXB0aW9uQmxvY2tCC5K1GAcIZBi4FyAUUgtkZXNjcm'
    'lwdGlvbhJDCgRheGVzGAYgAygLMicuaGF3a3IuY2F0YWxvZy52MWFscGhhMS5PcHRpb25BeGlz'
    'SW5wdXRCBpK1GAIIAlIEYXhlcxI/CgRza3VzGAcgAygLMiAuaGF3a3IuY2F0YWxvZy52MWFscG'
    'hhMS5Ta3VJbnB1dEIJ4EECkrUYAghkUgRza3Vz');

@$core.Deprecated('Use imageInputDescriptor instead')
const ImageInput$json = {
  '1': 'ImageInput',
  '2': [
    {'1': 'staged_token', '3': 1, '4': 1, '5': 9, '9': 0, '10': 'stagedToken'},
    {'1': 'asset_id', '3': 2, '4': 1, '5': 9, '9': 0, '10': 'assetId'},
    {'1': 'alt', '3': 3, '4': 1, '5': 9, '8': {}, '10': 'alt'},
  ],
  '8': [
    {'1': 'source'},
  ],
};

/// Descriptor for `ImageInput`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List imageInputDescriptor = $convert.base64Decode(
    'CgpJbWFnZUlucHV0EiMKDHN0YWdlZF90b2tlbhgBIAEoCUgAUgtzdGFnZWRUb2tlbhIbCghhc3'
    'NldF9pZBgCIAEoCUgAUgdhc3NldElkEhkKA2FsdBgDIAEoCUIHkrUYAxDIAVIDYWx0QggKBnNv'
    'dXJjZQ==');

@$core.Deprecated('Use descriptionBlockDescriptor instead')
const DescriptionBlock$json = {
  '1': 'DescriptionBlock',
  '2': [
    {'1': 'paragraph', '3': 1, '4': 1, '5': 9, '9': 0, '10': 'paragraph'},
    {
      '1': 'image',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.ImageInput',
      '9': 0,
      '10': 'image'
    },
    {
      '1': 'bullet_list',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.BulletList',
      '9': 0,
      '10': 'bulletList'
    },
    {
      '1': 'table',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.Table',
      '9': 0,
      '10': 'table'
    },
  ],
  '8': [
    {'1': 'block'},
  ],
};

/// Descriptor for `DescriptionBlock`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List descriptionBlockDescriptor = $convert.base64Decode(
    'ChBEZXNjcmlwdGlvbkJsb2NrEh4KCXBhcmFncmFwaBgBIAEoCUgAUglwYXJhZ3JhcGgSOgoFaW'
    '1hZ2UYAiABKAsyIi5oYXdrci5jYXRhbG9nLnYxYWxwaGExLkltYWdlSW5wdXRIAFIFaW1hZ2US'
    'RQoLYnVsbGV0X2xpc3QYAyABKAsyIi5oYXdrci5jYXRhbG9nLnYxYWxwaGExLkJ1bGxldExpc3'
    'RIAFIKYnVsbGV0TGlzdBI1CgV0YWJsZRgEIAEoCzIdLmhhd2tyLmNhdGFsb2cudjFhbHBoYTEu'
    'VGFibGVIAFIFdGFibGVCBwoFYmxvY2s=');

@$core.Deprecated('Use bulletListDescriptor instead')
const BulletList$json = {
  '1': 'BulletList',
  '2': [
    {'1': 'items', '3': 1, '4': 3, '5': 9, '8': {}, '10': 'items'},
  ],
};

/// Descriptor for `BulletList`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List bulletListDescriptor = $convert.base64Decode(
    'CgpCdWxsZXRMaXN0Eh8KBWl0ZW1zGAEgAygJQgmStRgFCDIQyAFSBWl0ZW1z');

@$core.Deprecated('Use tableDescriptor instead')
const Table$json = {
  '1': 'Table',
  '2': [
    {
      '1': 'rows',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.TableRow',
      '8': {},
      '10': 'rows'
    },
  ],
};

/// Descriptor for `Table`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List tableDescriptor = $convert.base64Decode(
    'CgVUYWJsZRI8CgRyb3dzGAEgAygLMiAuaGF3a3IuY2F0YWxvZy52MWFscGhhMS5UYWJsZVJvd0'
    'IGkrUYAggyUgRyb3dz');

@$core.Deprecated('Use tableRowDescriptor instead')
const TableRow$json = {
  '1': 'TableRow',
  '2': [
    {'1': 'cells', '3': 1, '4': 3, '5': 9, '8': {}, '10': 'cells'},
  ],
};

/// Descriptor for `TableRow`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List tableRowDescriptor = $convert.base64Decode(
    'CghUYWJsZVJvdxIfCgVjZWxscxgBIAMoCUIJkrUYBQgKEMgBUgVjZWxscw==');

@$core.Deprecated('Use optionAxisInputDescriptor instead')
const OptionAxisInput$json = {
  '1': 'OptionAxisInput',
  '2': [
    {'1': 'name', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'name'},
    {'1': 'values', '3': 2, '4': 3, '5': 9, '8': {}, '10': 'values'},
  ],
};

/// Descriptor for `OptionAxisInput`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List optionAxisInputDescriptor = $convert.base64Decode(
    'Cg9PcHRpb25BeGlzSW5wdXQSHQoEbmFtZRgBIAEoCUIJ4EECkrUYAhAUUgRuYW1lEiMKBnZhbH'
    'VlcxgCIAMoCUIL4EECkrUYBAgyEChSBnZhbHVlcw==');

@$core.Deprecated('Use skuInputDescriptor instead')
const SkuInput$json = {
  '1': 'SkuInput',
  '2': [
    {'1': 'option1', '3': 1, '4': 1, '5': 9, '10': 'option1'},
    {'1': 'option2', '3': 2, '4': 1, '5': 9, '10': 'option2'},
    {'1': 'price', '3': 3, '4': 1, '5': 3, '8': {}, '10': 'price'},
    {
      '1': 'compare_at_price',
      '3': 4,
      '4': 1,
      '5': 3,
      '9': 0,
      '10': 'compareAtPrice',
      '17': true
    },
    {'1': 'initial_stock', '3': 5, '4': 1, '5': 5, '10': 'initialStock'},
  ],
  '8': [
    {'1': '_compare_at_price'},
  ],
};

/// Descriptor for `SkuInput`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List skuInputDescriptor = $convert.base64Decode(
    'CghTa3VJbnB1dBIYCgdvcHRpb24xGAEgASgJUgdvcHRpb24xEhgKB29wdGlvbjIYAiABKAlSB2'
    '9wdGlvbjISGQoFcHJpY2UYAyABKANCA+BBAlIFcHJpY2USLQoQY29tcGFyZV9hdF9wcmljZRgE'
    'IAEoA0gAUg5jb21wYXJlQXRQcmljZYgBARIjCg1pbml0aWFsX3N0b2NrGAUgASgFUgxpbml0aW'
    'FsU3RvY2tCEwoRX2NvbXBhcmVfYXRfcHJpY2U=');
