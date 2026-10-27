// This is a generated file - do not edit.
//
// Generated from hawkr/common/v1alpha1/postal_address.proto.

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

@$core.Deprecated('Use postalAddressDescriptor instead')
const PostalAddress$json = {
  '1': 'PostalAddress',
  '2': [
    {'1': 'town_code', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'townCode'},
    {'1': 'address_detail', '3': 2, '4': 1, '5': 9, '10': 'addressDetail'},
  ],
};

/// Descriptor for `PostalAddress`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List postalAddressDescriptor = $convert.base64Decode(
    'Cg1Qb3N0YWxBZGRyZXNzEiAKCXRvd25fY29kZRgBIAEoCUID4EECUgh0b3duQ29kZRIlCg5hZG'
    'RyZXNzX2RldGFpbBgCIAEoCVINYWRkcmVzc0RldGFpbA==');
