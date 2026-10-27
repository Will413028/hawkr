// This is a generated file - do not edit.
//
// Generated from hawkr/agent/v1alpha1/agent.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

/// SessionVerificationMethod 是 session 驗證供應商持有邀請信箱的方式。
class SessionVerificationMethod extends $pb.ProtobufEnum {
  /// 沒有值，或這個版本的 client 不認得的方式。
  static const SessionVerificationMethod
      SESSION_VERIFICATION_METHOD_UNSPECIFIED = SessionVerificationMethod._(
          0, _omitEnumNames ? '' : 'SESSION_VERIFICATION_METHOD_UNSPECIFIED');

  /// Google 登入。
  static const SessionVerificationMethod SESSION_VERIFICATION_METHOD_GOOGLE =
      SessionVerificationMethod._(
          1, _omitEnumNames ? '' : 'SESSION_VERIFICATION_METHOD_GOOGLE');

  static const $core.List<SessionVerificationMethod> values =
      <SessionVerificationMethod>[
    SESSION_VERIFICATION_METHOD_UNSPECIFIED,
    SESSION_VERIFICATION_METHOD_GOOGLE,
  ];

  static final $core.List<SessionVerificationMethod?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 1);
  static SessionVerificationMethod? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const SessionVerificationMethod._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
