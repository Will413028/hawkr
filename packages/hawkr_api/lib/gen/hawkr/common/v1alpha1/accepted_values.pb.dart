// This is a generated file - do not edit.
//
// Generated from hawkr/common/v1alpha1/accepted_values.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

import 'policy_category.pbenum.dart' as $0;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

class Accepted_values {
  static final acceptedPolicyCategories =
      $pb.Extension<$0.PolicyCategory>.repeated(
          _omitMessageNames ? '' : 'google.protobuf.FieldOptions',
          _omitFieldNames ? '' : 'acceptedPolicyCategories',
          50001,
          $pb.PbFieldType.KE,
          check: $pb.getCheckFunction($pb.PbFieldType.KE),
          valueOf: $0.PolicyCategory.valueOf,
          enumValues: $0.PolicyCategory.values);
  static void registerAllExtensions($pb.ExtensionRegistry registry) {
    registry.add(acceptedPolicyCategories);
  }
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
