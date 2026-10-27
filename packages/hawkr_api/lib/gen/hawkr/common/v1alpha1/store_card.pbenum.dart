// This is a generated file - do not edit.
//
// Generated from hawkr/common/v1alpha1/store_card.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

/// StoreCardStatus 是店家卡片上的營運狀態。
class StoreCardStatus extends $pb.ProtobufEnum {
  static const StoreCardStatus STORE_CARD_STATUS_UNSPECIFIED =
      StoreCardStatus._(
          0, _omitEnumNames ? '' : 'STORE_CARD_STATUS_UNSPECIFIED');

  /// 營業中：此刻可以公開曝光（營運中、沒有停權、身分認證與 PayUni 審核通過、店名與 logo 與所在地齊全），有公開的店面頁，
  /// 也可以被追蹤。
  static const StoreCardStatus STORE_CARD_STATUS_OPEN =
      StoreCardStatus._(1, _omitEnumNames ? '' : 'STORE_CARD_STATUS_OPEN');

  /// 暫停：沒有關閉，但此刻不可公開曝光（停權、等待刪除、PayUni 被拒絕、身分認證改回草稿、缺店名或 logo 或所在地）。
  static const StoreCardStatus STORE_CARD_STATUS_PAUSED =
      StoreCardStatus._(2, _omitEnumNames ? '' : 'STORE_CARD_STATUS_PAUSED');

  /// 已關閉。
  static const StoreCardStatus STORE_CARD_STATUS_CLOSED =
      StoreCardStatus._(3, _omitEnumNames ? '' : 'STORE_CARD_STATUS_CLOSED');

  static const $core.List<StoreCardStatus> values = <StoreCardStatus>[
    STORE_CARD_STATUS_UNSPECIFIED,
    STORE_CARD_STATUS_OPEN,
    STORE_CARD_STATUS_PAUSED,
    STORE_CARD_STATUS_CLOSED,
  ];

  static final $core.List<StoreCardStatus?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static StoreCardStatus? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const StoreCardStatus._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
