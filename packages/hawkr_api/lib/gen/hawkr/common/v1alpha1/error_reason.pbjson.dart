// This is a generated file - do not edit.
//
// Generated from hawkr/common/v1alpha1/error_reason.proto.

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

@$core.Deprecated('Use errorReasonDescriptor instead')
const ErrorReason$json = {
  '1': 'ErrorReason',
  '2': [
    {'1': 'ERROR_REASON_UNSPECIFIED', '2': 0},
    {'1': 'VERSION_CONFLICT', '2': 1},
    {'1': 'SELLER_SUSPENDED', '2': 3},
    {'1': 'PROSPECT_MATCH_FOUND', '2': 4},
    {'1': 'PROSPECT_TAX_ID_HELD', '2': 10},
    {'1': 'AGREEMENT_OUTDATED', '2': 6},
    {'1': 'SELLER_REQUIREMENT_MISSING', '2': 7},
    {'1': 'STORE_OPENING_SUSPENDED', '2': 8},
    {'1': 'STORE_OPENING_TERMINATED', '2': 9},
    {'1': 'ACCOUNT_HAS_STORE', '2': 11},
    {'1': 'ACCOUNT_SUSPENDED', '2': 12},
    {'1': 'IDENTITY_ALREADY_LINKED', '2': 13},
    {'1': 'REAUTHENTICATION_REQUIRED', '2': 14},
  ],
  '4': [
    {'1': 2, '2': 2},
    {'1': 5, '2': 5},
  ],
  '5': ['PHONE_CODE_EXPIRED', 'ACCOUNT_HAS_OPEN_STORE'],
};

/// Descriptor for `ErrorReason`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List errorReasonDescriptor = $convert.base64Decode(
    'CgtFcnJvclJlYXNvbhIcChhFUlJPUl9SRUFTT05fVU5TUEVDSUZJRUQQABIUChBWRVJTSU9OX0'
    'NPTkZMSUNUEAESFAoQU0VMTEVSX1NVU1BFTkRFRBADEhgKFFBST1NQRUNUX01BVENIX0ZPVU5E'
    'EAQSGAoUUFJPU1BFQ1RfVEFYX0lEX0hFTEQQChIWChJBR1JFRU1FTlRfT1VUREFURUQQBhIeCh'
    'pTRUxMRVJfUkVRVUlSRU1FTlRfTUlTU0lORxAHEhsKF1NUT1JFX09QRU5JTkdfU1VTUEVOREVE'
    'EAgSHAoYU1RPUkVfT1BFTklOR19URVJNSU5BVEVEEAkSFQoRQUNDT1VOVF9IQVNfU1RPUkUQCx'
    'IVChFBQ0NPVU5UX1NVU1BFTkRFRBAMEhsKF0lERU5USVRZX0FMUkVBRFlfTElOS0VEEA0SHQoZ'
    'UkVBVVRIRU5USUNBVElPTl9SRVFVSVJFRBAOIgQIAhACIgQIBRAFKhJQSE9ORV9DT0RFX0VYUE'
    'lSRUQqFkFDQ09VTlRfSEFTX09QRU5fU1RPUkU=');
