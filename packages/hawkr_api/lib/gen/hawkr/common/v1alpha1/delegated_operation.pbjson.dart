// This is a generated file - do not edit.
//
// Generated from hawkr/common/v1alpha1/delegated_operation.proto.

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

@$core.Deprecated('Use delegatedOperationDescriptor instead')
const DelegatedOperation$json = {
  '1': 'DelegatedOperation',
  '2': [
    {'1': 'request_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'requestId'},
    {
      '1': 'operation_reason',
      '3': 2,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'operationReason'
    },
    {
      '1': 'notify_seller',
      '3': 3,
      '4': 1,
      '5': 8,
      '9': 0,
      '10': 'notifySeller',
      '17': true
    },
  ],
  '8': [
    {'1': '_notify_seller'},
  ],
};

/// Descriptor for `DelegatedOperation`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List delegatedOperationDescriptor = $convert.base64Decode(
    'ChJEZWxlZ2F0ZWRPcGVyYXRpb24SKgoKcmVxdWVzdF9pZBgBIAEoCUIL4EEC4ozP1wgCCAFSCX'
    'JlcXVlc3RJZBIuChBvcGVyYXRpb25fcmVhc29uGAIgASgJQgPgQQJSD29wZXJhdGlvblJlYXNv'
    'bhIoCg1ub3RpZnlfc2VsbGVyGAMgASgISABSDG5vdGlmeVNlbGxlcogBAUIQCg5fbm90aWZ5X3'
    'NlbGxlcg==');
