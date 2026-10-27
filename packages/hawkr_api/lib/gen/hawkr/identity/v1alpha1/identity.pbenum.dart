// This is a generated file - do not edit.
//
// Generated from hawkr/identity/v1alpha1/identity.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

/// Role 是帳號另外申請的角色。買家不是角色：每個帳號都是買家。
class Role extends $pb.ProtobufEnum {
  static const Role ROLE_UNSPECIFIED =
      Role._(0, _omitEnumNames ? '' : 'ROLE_UNSPECIFIED');
  static const Role ROLE_SELLER =
      Role._(2, _omitEnumNames ? '' : 'ROLE_SELLER');

  static const $core.List<Role> values = <Role>[
    ROLE_UNSPECIFIED,
    ROLE_SELLER,
  ];

  static final $core.Map<$core.int, Role> _byValue =
      $pb.ProtobufEnum.initByValue(values);
  static Role? valueOf($core.int value) => _byValue[value];

  const Role._(super.value, super.name);
}

/// DeletionState 是申請之後帳號的狀態。
class DeleteAccountResponse_DeletionState extends $pb.ProtobufEnum {
  static const DeleteAccountResponse_DeletionState DELETION_STATE_UNSPECIFIED =
      DeleteAccountResponse_DeletionState._(
          0, _omitEnumNames ? '' : 'DELETION_STATE_UNSPECIFIED');

  /// 帳號已刪除，client 登出。
  static const DeleteAccountResponse_DeletionState DELETION_STATE_DELETED =
      DeleteAccountResponse_DeletionState._(
          1, _omitEnumNames ? '' : 'DELETION_STATE_DELETED');

  /// 還有未完成的訂單，最後一筆完成後自動刪除。
  static const DeleteAccountResponse_DeletionState
      DELETION_STATE_PENDING_ORDERS = DeleteAccountResponse_DeletionState._(
          2, _omitEnumNames ? '' : 'DELETION_STATE_PENDING_ORDERS');

  static const $core.List<DeleteAccountResponse_DeletionState> values =
      <DeleteAccountResponse_DeletionState>[
    DELETION_STATE_UNSPECIFIED,
    DELETION_STATE_DELETED,
    DELETION_STATE_PENDING_ORDERS,
  ];

  static final $core.List<DeleteAccountResponse_DeletionState?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static DeleteAccountResponse_DeletionState? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const DeleteAccountResponse_DeletionState._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
