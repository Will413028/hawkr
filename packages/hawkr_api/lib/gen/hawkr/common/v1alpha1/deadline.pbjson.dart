// This is a generated file - do not edit.
//
// Generated from hawkr/common/v1alpha1/deadline.proto.

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

@$core.Deprecated('Use deadlineDescriptor instead')
const Deadline$json = {
  '1': 'Deadline',
  '2': [
    {
      '1': 'start_time',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'startTime'
    },
    {
      '1': 'due_time',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'dueTime'
    },
  ],
};

/// Descriptor for `Deadline`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deadlineDescriptor = $convert.base64Decode(
    'CghEZWFkbGluZRI+CgpzdGFydF90aW1lGAEgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdG'
    'FtcEID4EECUglzdGFydFRpbWUSOgoIZHVlX3RpbWUYAiABKAsyGi5nb29nbGUucHJvdG9idWYu'
    'VGltZXN0YW1wQgPgQQJSB2R1ZVRpbWU=');
