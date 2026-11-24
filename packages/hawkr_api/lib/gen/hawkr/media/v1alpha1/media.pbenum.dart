// This is a generated file - do not edit.
//
// Generated from hawkr/media/v1alpha1/media.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

/// UploadState 是一筆暫存上傳的狀態。client 遇到 UNSPECIFIED 或不認得的值走「未知」的呈現。
class UploadState extends $pb.ProtobufEnum {
  static const UploadState UPLOAD_STATE_UNSPECIFIED =
      UploadState._(0, _omitEnumNames ? '' : 'UPLOAD_STATE_UNSPECIFIED');

  /// 已發網址，還沒收到 CompleteUpload。
  static const UploadState UPLOAD_STATE_PENDING =
      UploadState._(1, _omitEnumNames ? '' : 'UPLOAD_STATE_PENDING');

  /// 已收到 CompleteUpload，等待處理。
  static const UploadState UPLOAD_STATE_UPLOADED =
      UploadState._(2, _omitEnumNames ? '' : 'UPLOAD_STATE_UPLOADED');

  /// 處理完成，staged token 可以交給擁有者轉正。
  static const UploadState UPLOAD_STATE_READY =
      UploadState._(3, _omitEnumNames ? '' : 'UPLOAD_STATE_READY');

  /// 處理被拒絕，原因在 GetUploadResponse.reject_reason；要重新 BeginUpload。
  static const UploadState UPLOAD_STATE_REJECTED =
      UploadState._(4, _omitEnumNames ? '' : 'UPLOAD_STATE_REJECTED');

  /// 已被擁有者轉正。
  static const UploadState UPLOAD_STATE_CONFIRMED =
      UploadState._(5, _omitEnumNames ? '' : 'UPLOAD_STATE_CONFIRMED');

  /// 正在回收（過期或被放棄）。
  static const UploadState UPLOAD_STATE_DELETING =
      UploadState._(6, _omitEnumNames ? '' : 'UPLOAD_STATE_DELETING');

  /// 已回收。
  static const UploadState UPLOAD_STATE_DELETED =
      UploadState._(7, _omitEnumNames ? '' : 'UPLOAD_STATE_DELETED');

  static const $core.List<UploadState> values = <UploadState>[
    UPLOAD_STATE_UNSPECIFIED,
    UPLOAD_STATE_PENDING,
    UPLOAD_STATE_UPLOADED,
    UPLOAD_STATE_READY,
    UPLOAD_STATE_REJECTED,
    UPLOAD_STATE_CONFIRMED,
    UPLOAD_STATE_DELETING,
    UPLOAD_STATE_DELETED,
  ];

  static final $core.List<UploadState?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 7);
  static UploadState? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const UploadState._(super.value, super.name);
}

/// RejectReason 是處理拒絕一筆暫存上傳的原因。
class RejectReason extends $pb.ProtobufEnum {
  static const RejectReason REJECT_REASON_UNSPECIFIED =
      RejectReason._(0, _omitEnumNames ? '' : 'REJECT_REASON_UNSPECIFIED');

  /// 辨認不出是支援的圖片格式。
  static const RejectReason REJECT_REASON_UNRECOGNIZED_FORMAT = RejectReason._(
      1, _omitEnumNames ? '' : 'REJECT_REASON_UNRECOGNIZED_FORMAT');

  /// 實際大小與宣告的不符。
  static const RejectReason REJECT_REASON_SIZE_MISMATCH =
      RejectReason._(2, _omitEnumNames ? '' : 'REJECT_REASON_SIZE_MISMATCH');

  /// 實際內容與宣告的 MD5 不符。
  static const RejectReason REJECT_REASON_CHECKSUM_MISMATCH = RejectReason._(
      3, _omitEnumNames ? '' : 'REJECT_REASON_CHECKSUM_MISMATCH');

  /// 處理失敗（例如圖片損毀或超過像素上限），重試不會成功。
  static const RejectReason REJECT_REASON_PROCESSING_FAILED = RejectReason._(
      4, _omitEnumNames ? '' : 'REJECT_REASON_PROCESSING_FAILED');

  /// 暫存在處理完成前過期。
  static const RejectReason REJECT_REASON_EXPIRED =
      RejectReason._(5, _omitEnumNames ? '' : 'REJECT_REASON_EXPIRED');

  static const $core.List<RejectReason> values = <RejectReason>[
    REJECT_REASON_UNSPECIFIED,
    REJECT_REASON_UNRECOGNIZED_FORMAT,
    REJECT_REASON_SIZE_MISMATCH,
    REJECT_REASON_CHECKSUM_MISMATCH,
    REJECT_REASON_PROCESSING_FAILED,
    REJECT_REASON_EXPIRED,
  ];

  static final $core.List<RejectReason?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 5);
  static RejectReason? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const RejectReason._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
