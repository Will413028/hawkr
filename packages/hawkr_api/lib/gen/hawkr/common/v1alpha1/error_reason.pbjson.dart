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
    {'1': 'SELLER_SUSPENDED', '2': 2},
    {'1': 'PROSPECT_MATCH_FOUND', '2': 3},
    {'1': 'PROSPECT_TAX_ID_HELD', '2': 4},
    {'1': 'AGREEMENT_OUTDATED', '2': 5},
    {'1': 'SELLER_REQUIREMENT_MISSING', '2': 6},
    {'1': 'STORE_OPENING_SUSPENDED', '2': 7},
    {'1': 'STORE_OPENING_TERMINATED', '2': 8},
    {'1': 'ACCOUNT_HAS_STORE', '2': 9},
    {'1': 'ACCOUNT_SUSPENDED', '2': 10},
    {'1': 'IDENTITY_ALREADY_LINKED', '2': 11},
    {'1': 'REAUTHENTICATION_REQUIRED', '2': 12},
    {'1': 'CONTACT_EMAIL_REQUIRED', '2': 13},
    {'1': 'CONTENT_TAKEDOWN_ACTIVE', '2': 14},
    {'1': 'REVIEW_ALREADY_EXISTS', '2': 15},
    {'1': 'ORDER_NOT_REVIEWABLE', '2': 16},
    {'1': 'REVIEW_WINDOW_EXPIRED', '2': 17},
    {'1': 'REVIEW_NOT_REPLYABLE', '2': 18},
    {'1': 'SELF_MESSAGE_NOT_ALLOWED', '2': 19},
    {'1': 'STORE_CLOSED', '2': 20},
    {'1': 'CHAT_BLOCKED', '2': 21},
    {'1': 'RECEIVER_DELETED', '2': 22},
  ],
};

/// Descriptor for `ErrorReason`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List errorReasonDescriptor = $convert.base64Decode(
    'CgtFcnJvclJlYXNvbhIcChhFUlJPUl9SRUFTT05fVU5TUEVDSUZJRUQQABIUChBWRVJTSU9OX0'
    'NPTkZMSUNUEAESFAoQU0VMTEVSX1NVU1BFTkRFRBACEhgKFFBST1NQRUNUX01BVENIX0ZPVU5E'
    'EAMSGAoUUFJPU1BFQ1RfVEFYX0lEX0hFTEQQBBIWChJBR1JFRU1FTlRfT1VUREFURUQQBRIeCh'
    'pTRUxMRVJfUkVRVUlSRU1FTlRfTUlTU0lORxAGEhsKF1NUT1JFX09QRU5JTkdfU1VTUEVOREVE'
    'EAcSHAoYU1RPUkVfT1BFTklOR19URVJNSU5BVEVEEAgSFQoRQUNDT1VOVF9IQVNfU1RPUkUQCR'
    'IVChFBQ0NPVU5UX1NVU1BFTkRFRBAKEhsKF0lERU5USVRZX0FMUkVBRFlfTElOS0VEEAsSHQoZ'
    'UkVBVVRIRU5USUNBVElPTl9SRVFVSVJFRBAMEhoKFkNPTlRBQ1RfRU1BSUxfUkVRVUlSRUQQDR'
    'IbChdDT05URU5UX1RBS0VET1dOX0FDVElWRRAOEhkKFVJFVklFV19BTFJFQURZX0VYSVNUUxAP'
    'EhgKFE9SREVSX05PVF9SRVZJRVdBQkxFEBASGQoVUkVWSUVXX1dJTkRPV19FWFBJUkVEEBESGA'
    'oUUkVWSUVXX05PVF9SRVBMWUFCTEUQEhIcChhTRUxGX01FU1NBR0VfTk9UX0FMTE9XRUQQExIQ'
    'CgxTVE9SRV9DTE9TRUQQFBIQCgxDSEFUX0JMT0NLRUQQFRIUChBSRUNFSVZFUl9ERUxFVEVEEB'
    'Y=');
