// This is a generated file - do not edit.
//
// Generated from hawkr/procurement/v1alpha1/procurement.proto.

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

import 'package:protobuf/well_known_types/google/protobuf/timestamp.pbjson.dart'
    as $1;

import '../../../google/type/date.pbjson.dart' as $0;
import '../../common/v1alpha1/image.pbjson.dart' as $3;
import '../../common/v1alpha1/store_card.pbjson.dart' as $2;

@$core.Deprecated('Use slotKeyDescriptor instead')
const SlotKey$json = {
  '1': 'SlotKey',
  '2': [
    {'1': 'SLOT_KEY_UNSPECIFIED', '2': 0},
    {'1': 'SLOT_KEY_ITEM', '2': 1},
    {'1': 'SLOT_KEY_ITEM_RESOLUTION', '2': 2},
    {'1': 'SLOT_KEY_QUANTITY', '2': 3},
    {'1': 'SLOT_KEY_UNIT', '2': 4},
    {'1': 'SLOT_KEY_PACKAGING', '2': 5},
    {'1': 'SLOT_KEY_BUDGET_PER_UNIT', '2': 6},
    {'1': 'SLOT_KEY_DELIVERY_CITY', '2': 7},
    {'1': 'SLOT_KEY_DELIVERY_DATE', '2': 8},
    {'1': 'SLOT_KEY_EXACT_MATCH_REQUIRED', '2': 9},
    {'1': 'SLOT_KEY_ITEM_DISAMBIGUATION', '2': 10},
  ],
};

/// Descriptor for `SlotKey`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List slotKeyDescriptor = $convert.base64Decode(
    'CgdTbG90S2V5EhgKFFNMT1RfS0VZX1VOU1BFQ0lGSUVEEAASEQoNU0xPVF9LRVlfSVRFTRABEh'
    'wKGFNMT1RfS0VZX0lURU1fUkVTT0xVVElPThACEhUKEVNMT1RfS0VZX1FVQU5USVRZEAMSEQoN'
    'U0xPVF9LRVlfVU5JVBAEEhYKElNMT1RfS0VZX1BBQ0tBR0lORxAFEhwKGFNMT1RfS0VZX0JVRE'
    'dFVF9QRVJfVU5JVBAGEhoKFlNMT1RfS0VZX0RFTElWRVJZX0NJVFkQBxIaChZTTE9UX0tFWV9E'
    'RUxJVkVSWV9EQVRFEAgSIQodU0xPVF9LRVlfRVhBQ1RfTUFUQ0hfUkVRVUlSRUQQCRIgChxTTE'
    '9UX0tFWV9JVEVNX0RJU0FNQklHVUFUSU9OEAo=');

@$core.Deprecated('Use rejectionReasonDescriptor instead')
const RejectionReason$json = {
  '1': 'RejectionReason',
  '2': [
    {'1': 'REJECTION_REASON_UNSPECIFIED', '2': 0},
    {'1': 'REJECTION_REASON_EMPTY', '2': 1},
    {'1': 'REJECTION_REASON_TRUNCATED', '2': 2},
    {'1': 'REJECTION_REASON_QUANTITY_RANGE', '2': 3},
    {'1': 'REJECTION_REASON_BUDGET_RANGE', '2': 4},
    {'1': 'REJECTION_REASON_UNIT_NOT_ALLOWED', '2': 5},
    {'1': 'REJECTION_REASON_CITY_UNRECOGNIZED', '2': 6},
    {'1': 'REJECTION_REASON_DATE_FORMAT', '2': 7},
    {'1': 'REJECTION_REASON_DATE_MISSING_YEAR', '2': 8},
    {'1': 'REJECTION_REASON_DATE_PAST', '2': 9},
    {'1': 'REJECTION_REASON_DATE_TOO_FAR', '2': 10},
    {'1': 'REJECTION_REASON_RESOLUTION_NO_ITEM', '2': 11},
    {'1': 'REJECTION_REASON_RESOLUTION_NO_LABEL', '2': 12},
    {'1': 'REJECTION_REASON_RESOLUTION_UNKNOWN_KIND', '2': 13},
  ],
};

/// Descriptor for `RejectionReason`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List rejectionReasonDescriptor = $convert.base64Decode(
    'Cg9SZWplY3Rpb25SZWFzb24SIAocUkVKRUNUSU9OX1JFQVNPTl9VTlNQRUNJRklFRBAAEhoKFl'
    'JFSkVDVElPTl9SRUFTT05fRU1QVFkQARIeChpSRUpFQ1RJT05fUkVBU09OX1RSVU5DQVRFRBAC'
    'EiMKH1JFSkVDVElPTl9SRUFTT05fUVVBTlRJVFlfUkFOR0UQAxIhCh1SRUpFQ1RJT05fUkVBU0'
    '9OX0JVREdFVF9SQU5HRRAEEiUKIVJFSkVDVElPTl9SRUFTT05fVU5JVF9OT1RfQUxMT1dFRBAF'
    'EiYKIlJFSkVDVElPTl9SRUFTT05fQ0lUWV9VTlJFQ09HTklaRUQQBhIgChxSRUpFQ1RJT05fUk'
    'VBU09OX0RBVEVfRk9STUFUEAcSJgoiUkVKRUNUSU9OX1JFQVNPTl9EQVRFX01JU1NJTkdfWUVB'
    'UhAIEh4KGlJFSkVDVElPTl9SRUFTT05fREFURV9QQVNUEAkSIQodUkVKRUNUSU9OX1JFQVNPTl'
    '9EQVRFX1RPT19GQVIQChInCiNSRUpFQ1RJT05fUkVBU09OX1JFU09MVVRJT05fTk9fSVRFTRAL'
    'EigKJFJFSkVDVElPTl9SRUFTT05fUkVTT0xVVElPTl9OT19MQUJFTBAMEiwKKFJFSkVDVElPTl'
    '9SRUFTT05fUkVTT0xVVElPTl9VTktOT1dOX0tJTkQQDQ==');

@$core.Deprecated('Use requestStatusDescriptor instead')
const RequestStatus$json = {
  '1': 'RequestStatus',
  '2': [
    {'1': 'REQUEST_STATUS_UNSPECIFIED', '2': 0},
    {'1': 'REQUEST_STATUS_CLARIFYING', '2': 1},
    {'1': 'REQUEST_STATUS_REQUIREMENT_READY', '2': 2},
    {'1': 'REQUEST_STATUS_MATCHING', '2': 3},
  ],
};

/// Descriptor for `RequestStatus`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List requestStatusDescriptor = $convert.base64Decode(
    'Cg1SZXF1ZXN0U3RhdHVzEh4KGlJFUVVFU1RfU1RBVFVTX1VOU1BFQ0lGSUVEEAASHQoZUkVRVU'
    'VTVF9TVEFUVVNfQ0xBUklGWUlORxABEiQKIFJFUVVFU1RfU1RBVFVTX1JFUVVJUkVNRU5UX1JF'
    'QURZEAISGwoXUkVRVUVTVF9TVEFUVVNfTUFUQ0hJTkcQAw==');

@$core.Deprecated('Use runTypeDescriptor instead')
const RunType$json = {
  '1': 'RunType',
  '2': [
    {'1': 'RUN_TYPE_UNSPECIFIED', '2': 0},
    {'1': 'RUN_TYPE_TURN', '2': 1},
    {'1': 'RUN_TYPE_CANDIDATE_SEARCH', '2': 2},
    {'1': 'RUN_TYPE_CLARIFICATION_RESUME', '2': 3},
  ],
};

/// Descriptor for `RunType`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List runTypeDescriptor = $convert.base64Decode(
    'CgdSdW5UeXBlEhgKFFJVTl9UWVBFX1VOU1BFQ0lGSUVEEAASEQoNUlVOX1RZUEVfVFVSThABEh'
    '0KGVJVTl9UWVBFX0NBTkRJREFURV9TRUFSQ0gQAhIhCh1SVU5fVFlQRV9DTEFSSUZJQ0FUSU9O'
    'X1JFU1VNRRAD');

@$core.Deprecated('Use runStatusDescriptor instead')
const RunStatus$json = {
  '1': 'RunStatus',
  '2': [
    {'1': 'RUN_STATUS_UNSPECIFIED', '2': 0},
    {'1': 'RUN_STATUS_QUEUED', '2': 1},
    {'1': 'RUN_STATUS_RUNNING', '2': 2},
    {'1': 'RUN_STATUS_WAITING_FOR_INPUT', '2': 3},
    {'1': 'RUN_STATUS_CANCEL_REQUESTED', '2': 4},
    {'1': 'RUN_STATUS_SUCCEEDED', '2': 5},
    {'1': 'RUN_STATUS_FAILED', '2': 6},
    {'1': 'RUN_STATUS_CANCELLED', '2': 7},
  ],
  '4': [
    {'1': 8, '2': 8},
    {'1': 9, '2': 9},
  ],
  '5': ['RUN_STATUS_RESUMED', 'RUN_STATUS_EXPIRED'],
};

/// Descriptor for `RunStatus`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List runStatusDescriptor = $convert.base64Decode(
    'CglSdW5TdGF0dXMSGgoWUlVOX1NUQVRVU19VTlNQRUNJRklFRBAAEhUKEVJVTl9TVEFUVVNfUV'
    'VFVUVEEAESFgoSUlVOX1NUQVRVU19SVU5OSU5HEAISIAocUlVOX1NUQVRVU19XQUlUSU5HX0ZP'
    'Ul9JTlBVVBADEh8KG1JVTl9TVEFUVVNfQ0FOQ0VMX1JFUVVFU1RFRBAEEhgKFFJVTl9TVEFUVV'
    'NfU1VDQ0VFREVEEAUSFQoRUlVOX1NUQVRVU19GQUlMRUQQBhIYChRSVU5fU1RBVFVTX0NBTkNF'
    'TExFRBAHIgQICBAIIgQICRAJKhJSVU5fU1RBVFVTX1JFU1VNRUQqElJVTl9TVEFUVVNfRVhQSV'
    'JFRA==');

@$core.Deprecated('Use runFailureReasonDescriptor instead')
const RunFailureReason$json = {
  '1': 'RunFailureReason',
  '2': [
    {'1': 'RUN_FAILURE_REASON_UNSPECIFIED', '2': 0},
    {'1': 'RUN_FAILURE_REASON_WORKER_LOST', '2': 1},
    {'1': 'RUN_FAILURE_REASON_SEARCH_FAILED', '2': 2},
    {'1': 'RUN_FAILURE_REASON_TURN_PIPELINE_ERROR', '2': 3},
    {'1': 'RUN_FAILURE_REASON_MODEL_UNAVAILABLE', '2': 5},
    {'1': 'RUN_FAILURE_REASON_ITEM_CATEGORY_UNAVAILABLE', '2': 6},
    {'1': 'RUN_FAILURE_REASON_ANSWER_REJECTED', '2': 7},
  ],
  '4': [
    {'1': 4, '2': 4},
  ],
  '5': ['RUN_FAILURE_REASON_EXPIRED'],
};

/// Descriptor for `RunFailureReason`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List runFailureReasonDescriptor = $convert.base64Decode(
    'ChBSdW5GYWlsdXJlUmVhc29uEiIKHlJVTl9GQUlMVVJFX1JFQVNPTl9VTlNQRUNJRklFRBAAEi'
    'IKHlJVTl9GQUlMVVJFX1JFQVNPTl9XT1JLRVJfTE9TVBABEiQKIFJVTl9GQUlMVVJFX1JFQVNP'
    'Tl9TRUFSQ0hfRkFJTEVEEAISKgomUlVOX0ZBSUxVUkVfUkVBU09OX1RVUk5fUElQRUxJTkVfRV'
    'JST1IQAxIoCiRSVU5fRkFJTFVSRV9SRUFTT05fTU9ERUxfVU5BVkFJTEFCTEUQBRIwCixSVU5f'
    'RkFJTFVSRV9SRUFTT05fSVRFTV9DQVRFR09SWV9VTkFWQUlMQUJMRRAGEiYKIlJVTl9GQUlMVV'
    'JFX1JFQVNPTl9BTlNXRVJfUkVKRUNURUQQByIECAQQBCoaUlVOX0ZBSUxVUkVfUkVBU09OX0VY'
    'UElSRUQ=');

@$core.Deprecated('Use stepStatusDescriptor instead')
const StepStatus$json = {
  '1': 'StepStatus',
  '2': [
    {'1': 'STEP_STATUS_UNSPECIFIED', '2': 0},
    {'1': 'STEP_STATUS_PENDING', '2': 1},
    {'1': 'STEP_STATUS_RUNNING', '2': 2},
    {'1': 'STEP_STATUS_WAITING_FOR_INPUT', '2': 3},
    {'1': 'STEP_STATUS_SUCCEEDED', '2': 4},
    {'1': 'STEP_STATUS_FAILED', '2': 5},
    {'1': 'STEP_STATUS_SKIPPED', '2': 6},
    {'1': 'STEP_STATUS_CANCELLED', '2': 7},
  ],
};

/// Descriptor for `StepStatus`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List stepStatusDescriptor = $convert.base64Decode(
    'CgpTdGVwU3RhdHVzEhsKF1NURVBfU1RBVFVTX1VOU1BFQ0lGSUVEEAASFwoTU1RFUF9TVEFUVV'
    'NfUEVORElORxABEhcKE1NURVBfU1RBVFVTX1JVTk5JTkcQAhIhCh1TVEVQX1NUQVRVU19XQUlU'
    'SU5HX0ZPUl9JTlBVVBADEhkKFVNURVBfU1RBVFVTX1NVQ0NFRURFRBAEEhYKElNURVBfU1RBVF'
    'VTX0ZBSUxFRBAFEhcKE1NURVBfU1RBVFVTX1NLSVBQRUQQBhIZChVTVEVQX1NUQVRVU19DQU5D'
    'RUxMRUQQBw==');

@$core.Deprecated('Use interruptKindDescriptor instead')
const InterruptKind$json = {
  '1': 'InterruptKind',
  '2': [
    {'1': 'INTERRUPT_KIND_UNSPECIFIED', '2': 0},
    {'1': 'INTERRUPT_KIND_CLARIFICATION', '2': 1},
  ],
};

/// Descriptor for `InterruptKind`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List interruptKindDescriptor = $convert.base64Decode(
    'Cg1JbnRlcnJ1cHRLaW5kEh4KGklOVEVSUlVQVF9LSU5EX1VOU1BFQ0lGSUVEEAASIAocSU5URV'
    'JSVVBUX0tJTkRfQ0xBUklGSUNBVElPThAB');

@$core.Deprecated('Use itemResolutionKindDescriptor instead')
const ItemResolutionKind$json = {
  '1': 'ItemResolutionKind',
  '2': [
    {'1': 'ITEM_RESOLUTION_KIND_UNSPECIFIED', '2': 0},
    {'1': 'ITEM_RESOLUTION_KIND_CATEGORY_CONFIRMED', '2': 1},
    {'1': 'ITEM_RESOLUTION_KIND_BUYER_DESCRIBED', '2': 2},
  ],
};

/// Descriptor for `ItemResolutionKind`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List itemResolutionKindDescriptor = $convert.base64Decode(
    'ChJJdGVtUmVzb2x1dGlvbktpbmQSJAogSVRFTV9SRVNPTFVUSU9OX0tJTkRfVU5TUEVDSUZJRU'
    'QQABIrCidJVEVNX1JFU09MVVRJT05fS0lORF9DQVRFR09SWV9DT05GSVJNRUQQARIoCiRJVEVN'
    'X1JFU09MVVRJT05fS0lORF9CVVlFUl9ERVNDUklCRUQQAg==');

@$core.Deprecated('Use conversationRoleDescriptor instead')
const ConversationRole$json = {
  '1': 'ConversationRole',
  '2': [
    {'1': 'CONVERSATION_ROLE_UNSPECIFIED', '2': 0},
    {'1': 'CONVERSATION_ROLE_USER', '2': 1},
    {'1': 'CONVERSATION_ROLE_ASSISTANT', '2': 2},
  ],
};

/// Descriptor for `ConversationRole`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List conversationRoleDescriptor = $convert.base64Decode(
    'ChBDb252ZXJzYXRpb25Sb2xlEiEKHUNPTlZFUlNBVElPTl9ST0xFX1VOU1BFQ0lGSUVEEAASGg'
    'oWQ09OVkVSU0FUSU9OX1JPTEVfVVNFUhABEh8KG0NPTlZFUlNBVElPTl9ST0xFX0FTU0lTVEFO'
    'VBAC');

@$core.Deprecated('Use blockKindDescriptor instead')
const BlockKind$json = {
  '1': 'BlockKind',
  '2': [
    {'1': 'BLOCK_KIND_UNSPECIFIED', '2': 0},
    {'1': 'BLOCK_KIND_UNDERSTOOD', '2': 1},
    {'1': 'BLOCK_KIND_CONTEXT', '2': 2},
    {'1': 'BLOCK_KIND_RATIONALE', '2': 3},
    {'1': 'BLOCK_KIND_WARNING', '2': 4},
    {'1': 'BLOCK_KIND_NEXT_STEP', '2': 5},
    {'1': 'BLOCK_KIND_COMPLETE', '2': 6},
  ],
};

/// Descriptor for `BlockKind`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List blockKindDescriptor = $convert.base64Decode(
    'CglCbG9ja0tpbmQSGgoWQkxPQ0tfS0lORF9VTlNQRUNJRklFRBAAEhkKFUJMT0NLX0tJTkRfVU'
    '5ERVJTVE9PRBABEhYKEkJMT0NLX0tJTkRfQ09OVEVYVBACEhgKFEJMT0NLX0tJTkRfUkFUSU9O'
    'QUxFEAMSFgoSQkxPQ0tfS0lORF9XQVJOSU5HEAQSGAoUQkxPQ0tfS0lORF9ORVhUX1NURVAQBR'
    'IXChNCTE9DS19LSU5EX0NPTVBMRVRFEAY=');

@$core.Deprecated('Use audienceModeDescriptor instead')
const AudienceMode$json = {
  '1': 'AudienceMode',
  '2': [
    {'1': 'AUDIENCE_MODE_UNSPECIFIED', '2': 0},
    {'1': 'AUDIENCE_MODE_WISH_POOL', '2': 1},
    {'1': 'AUDIENCE_MODE_DIRECT_RECIPIENTS', '2': 2},
  ],
};

/// Descriptor for `AudienceMode`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List audienceModeDescriptor = $convert.base64Decode(
    'CgxBdWRpZW5jZU1vZGUSHQoZQVVESUVOQ0VfTU9ERV9VTlNQRUNJRklFRBAAEhsKF0FVRElFTk'
    'NFX01PREVfV0lTSF9QT09MEAESIwofQVVESUVOQ0VfTU9ERV9ESVJFQ1RfUkVDSVBJRU5UUxAC');

@$core.Deprecated('Use publicationStateDescriptor instead')
const PublicationState$json = {
  '1': 'PublicationState',
  '2': [
    {'1': 'PUBLICATION_STATE_UNSPECIFIED', '2': 0},
    {'1': 'PUBLICATION_STATE_PUBLISHED', '2': 1},
  ],
};

/// Descriptor for `PublicationState`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List publicationStateDescriptor = $convert.base64Decode(
    'ChBQdWJsaWNhdGlvblN0YXRlEiEKHVBVQkxJQ0FUSU9OX1NUQVRFX1VOU1BFQ0lGSUVEEAASHw'
    'obUFVCTElDQVRJT05fU1RBVEVfUFVCTElTSEVEEAE=');

@$core.Deprecated('Use inviteDeliveryStateDescriptor instead')
const InviteDeliveryState$json = {
  '1': 'InviteDeliveryState',
  '2': [
    {'1': 'INVITE_DELIVERY_STATE_UNSPECIFIED', '2': 0},
    {'1': 'INVITE_DELIVERY_STATE_QUEUED', '2': 1},
    {'1': 'INVITE_DELIVERY_STATE_SENT', '2': 2},
    {'1': 'INVITE_DELIVERY_STATE_FAILED', '2': 3},
  ],
};

/// Descriptor for `InviteDeliveryState`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List inviteDeliveryStateDescriptor = $convert.base64Decode(
    'ChNJbnZpdGVEZWxpdmVyeVN0YXRlEiUKIUlOVklURV9ERUxJVkVSWV9TVEFURV9VTlNQRUNJRk'
    'lFRBAAEiAKHElOVklURV9ERUxJVkVSWV9TVEFURV9RVUVVRUQQARIeChpJTlZJVEVfREVMSVZF'
    'UllfU1RBVEVfU0VOVBACEiAKHElOVklURV9ERUxJVkVSWV9TVEFURV9GQUlMRUQQAw==');

@$core.Deprecated('Use sellerResponseTypeDescriptor instead')
const SellerResponseType$json = {
  '1': 'SellerResponseType',
  '2': [
    {'1': 'SELLER_RESPONSE_TYPE_UNSPECIFIED', '2': 0},
    {'1': 'SELLER_RESPONSE_TYPE_MANUAL_QUOTE', '2': 1},
    {'1': 'SELLER_RESPONSE_TYPE_ACCEPT_NON_PRICE_TERMS_WITH_QUOTE', '2': 2},
  ],
};

/// Descriptor for `SellerResponseType`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List sellerResponseTypeDescriptor = $convert.base64Decode(
    'ChJTZWxsZXJSZXNwb25zZVR5cGUSJAogU0VMTEVSX1JFU1BPTlNFX1RZUEVfVU5TUEVDSUZJRU'
    'QQABIlCiFTRUxMRVJfUkVTUE9OU0VfVFlQRV9NQU5VQUxfUVVPVEUQARI6CjZTRUxMRVJfUkVT'
    'UE9OU0VfVFlQRV9BQ0NFUFRfTk9OX1BSSUNFX1RFUk1TX1dJVEhfUVVPVEUQAg==');

@$core.Deprecated('Use taxInclusionBasisDescriptor instead')
const TaxInclusionBasis$json = {
  '1': 'TaxInclusionBasis',
  '2': [
    {'1': 'TAX_INCLUSION_BASIS_UNSPECIFIED', '2': 0},
    {'1': 'TAX_INCLUSION_BASIS_TAX_INCLUDED', '2': 1},
    {'1': 'TAX_INCLUSION_BASIS_TAX_EXCLUDED', '2': 2},
  ],
};

/// Descriptor for `TaxInclusionBasis`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List taxInclusionBasisDescriptor = $convert.base64Decode(
    'ChFUYXhJbmNsdXNpb25CYXNpcxIjCh9UQVhfSU5DTFVTSU9OX0JBU0lTX1VOU1BFQ0lGSUVEEA'
    'ASJAogVEFYX0lOQ0xVU0lPTl9CQVNJU19UQVhfSU5DTFVERUQQARIkCiBUQVhfSU5DTFVTSU9O'
    'X0JBU0lTX1RBWF9FWENMVURFRBAC');

@$core.Deprecated('Use sellerResponseStatusDescriptor instead')
const SellerResponseStatus$json = {
  '1': 'SellerResponseStatus',
  '2': [
    {'1': 'SELLER_RESPONSE_STATUS_UNSPECIFIED', '2': 0},
    {'1': 'SELLER_RESPONSE_STATUS_ACTIVE', '2': 1},
    {'1': 'SELLER_RESPONSE_STATUS_SUPERSEDED', '2': 2},
    {'1': 'SELLER_RESPONSE_STATUS_WITHDRAWN', '2': 3},
    {'1': 'SELLER_RESPONSE_STATUS_EXPIRED', '2': 4},
  ],
};

/// Descriptor for `SellerResponseStatus`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List sellerResponseStatusDescriptor = $convert.base64Decode(
    'ChRTZWxsZXJSZXNwb25zZVN0YXR1cxImCiJTRUxMRVJfUkVTUE9OU0VfU1RBVFVTX1VOU1BFQ0'
    'lGSUVEEAASIQodU0VMTEVSX1JFU1BPTlNFX1NUQVRVU19BQ1RJVkUQARIlCiFTRUxMRVJfUkVT'
    'UE9OU0VfU1RBVFVTX1NVUEVSU0VERUQQAhIkCiBTRUxMRVJfUkVTUE9OU0VfU1RBVFVTX1dJVE'
    'hEUkFXThADEiIKHlNFTExFUl9SRVNQT05TRV9TVEFUVVNfRVhQSVJFRBAE');

@$core.Deprecated('Use quoteAcceptabilityDescriptor instead')
const QuoteAcceptability$json = {
  '1': 'QuoteAcceptability',
  '2': [
    {'1': 'QUOTE_ACCEPTABILITY_UNSPECIFIED', '2': 0},
    {'1': 'QUOTE_ACCEPTABILITY_ACCEPTABLE', '2': 1},
    {'1': 'QUOTE_ACCEPTABILITY_EXPIRED', '2': 2},
    {'1': 'QUOTE_ACCEPTABILITY_WITHDRAWN', '2': 3},
    {'1': 'QUOTE_ACCEPTABILITY_SUPERSEDED', '2': 4},
    {'1': 'QUOTE_ACCEPTABILITY_INELIGIBLE', '2': 5},
  ],
};

/// Descriptor for `QuoteAcceptability`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List quoteAcceptabilityDescriptor = $convert.base64Decode(
    'ChJRdW90ZUFjY2VwdGFiaWxpdHkSIwofUVVPVEVfQUNDRVBUQUJJTElUWV9VTlNQRUNJRklFRB'
    'AAEiIKHlFVT1RFX0FDQ0VQVEFCSUxJVFlfQUNDRVBUQUJMRRABEh8KG1FVT1RFX0FDQ0VQVEFC'
    'SUxJVFlfRVhQSVJFRBACEiEKHVFVT1RFX0FDQ0VQVEFCSUxJVFlfV0lUSERSQVdOEAMSIgoeUV'
    'VPVEVfQUNDRVBUQUJJTElUWV9TVVBFUlNFREVEEAQSIgoeUVVPVEVfQUNDRVBUQUJJTElUWV9J'
    'TkVMSUdJQkxFEAU=');

@$core.Deprecated('Use orderProvisioningStateDescriptor instead')
const OrderProvisioningState$json = {
  '1': 'OrderProvisioningState',
  '2': [
    {'1': 'ORDER_PROVISIONING_STATE_UNSPECIFIED', '2': 0},
    {'1': 'ORDER_PROVISIONING_STATE_CREATING_ORDER', '2': 1},
    {'1': 'ORDER_PROVISIONING_STATE_ORDER_CREATED', '2': 2},
    {'1': 'ORDER_PROVISIONING_STATE_ORDER_FAILED', '2': 3},
  ],
};

/// Descriptor for `OrderProvisioningState`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List orderProvisioningStateDescriptor = $convert.base64Decode(
    'ChZPcmRlclByb3Zpc2lvbmluZ1N0YXRlEigKJE9SREVSX1BST1ZJU0lPTklOR19TVEFURV9VTl'
    'NQRUNJRklFRBAAEisKJ09SREVSX1BST1ZJU0lPTklOR19TVEFURV9DUkVBVElOR19PUkRFUhAB'
    'EioKJk9SREVSX1BST1ZJU0lPTklOR19TVEFURV9PUkRFUl9DUkVBVEVEEAISKQolT1JERVJfUF'
    'JPVklTSU9OSU5HX1NUQVRFX09SREVSX0ZBSUxFRBAD');

@$core.Deprecated('Use cancelDispositionDescriptor instead')
const CancelDisposition$json = {
  '1': 'CancelDisposition',
  '2': [
    {'1': 'CANCEL_DISPOSITION_UNSPECIFIED', '2': 0},
    {'1': 'CANCEL_DISPOSITION_REQUESTED', '2': 1},
    {'1': 'CANCEL_DISPOSITION_ALREADY_CANCELLED', '2': 2},
    {'1': 'CANCEL_DISPOSITION_ALREADY_TERMINAL', '2': 3},
    {'1': 'CANCEL_DISPOSITION_TOO_LATE', '2': 4},
  ],
};

/// Descriptor for `CancelDisposition`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List cancelDispositionDescriptor = $convert.base64Decode(
    'ChFDYW5jZWxEaXNwb3NpdGlvbhIiCh5DQU5DRUxfRElTUE9TSVRJT05fVU5TUEVDSUZJRUQQAB'
    'IgChxDQU5DRUxfRElTUE9TSVRJT05fUkVRVUVTVEVEEAESKAokQ0FOQ0VMX0RJU1BPU0lUSU9O'
    'X0FMUkVBRFlfQ0FOQ0VMTEVEEAISJwojQ0FOQ0VMX0RJU1BPU0lUSU9OX0FMUkVBRFlfVEVSTU'
    'lOQUwQAxIfChtDQU5DRUxfRElTUE9TSVRJT05fVE9PX0xBVEUQBA==');

@$core.Deprecated('Use questionOptionDescriptor instead')
const QuestionOption$json = {
  '1': 'QuestionOption',
  '2': [
    {'1': 'option_id', '3': 1, '4': 1, '5': 9, '10': 'optionId'},
    {'1': 'label', '3': 2, '4': 1, '5': 9, '10': 'label'},
  ],
};

/// Descriptor for `QuestionOption`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List questionOptionDescriptor = $convert.base64Decode(
    'Cg5RdWVzdGlvbk9wdGlvbhIbCglvcHRpb25faWQYASABKAlSCG9wdGlvbklkEhQKBWxhYmVsGA'
    'IgASgJUgVsYWJlbA==');

@$core.Deprecated('Use clarifyQuestionDescriptor instead')
const ClarifyQuestion$json = {
  '1': 'ClarifyQuestion',
  '2': [
    {
      '1': 'slot_key',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.SlotKey',
      '10': 'slotKey'
    },
    {'1': 'prompt', '3': 2, '4': 1, '5': 9, '10': 'prompt'},
    {
      '1': 'options',
      '3': 3,
      '4': 3,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.QuestionOption',
      '10': 'options'
    },
    {
      '1': 'unit_options',
      '3': 4,
      '4': 3,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.QuestionOption',
      '10': 'unitOptions'
    },
    {'1': 'allow_free_text', '3': 5, '4': 1, '5': 8, '10': 'allowFreeText'},
    {'1': 'can_skip', '3': 6, '4': 1, '5': 8, '10': 'canSkip'},
  ],
};

/// Descriptor for `ClarifyQuestion`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List clarifyQuestionDescriptor = $convert.base64Decode(
    'Cg9DbGFyaWZ5UXVlc3Rpb24SPgoIc2xvdF9rZXkYASABKA4yIy5oYXdrci5wcm9jdXJlbWVudC'
    '52MWFscGhhMS5TbG90S2V5UgdzbG90S2V5EhYKBnByb21wdBgCIAEoCVIGcHJvbXB0EkQKB29w'
    'dGlvbnMYAyADKAsyKi5oYXdrci5wcm9jdXJlbWVudC52MWFscGhhMS5RdWVzdGlvbk9wdGlvbl'
    'IHb3B0aW9ucxJNCgx1bml0X29wdGlvbnMYBCADKAsyKi5oYXdrci5wcm9jdXJlbWVudC52MWFs'
    'cGhhMS5RdWVzdGlvbk9wdGlvblILdW5pdE9wdGlvbnMSJgoPYWxsb3dfZnJlZV90ZXh0GAUgAS'
    'gIUg1hbGxvd0ZyZWVUZXh0EhkKCGNhbl9za2lwGAYgASgIUgdjYW5Ta2lw');

@$core.Deprecated('Use stepDescriptor instead')
const Step$json = {
  '1': 'Step',
  '2': [
    {'1': 'code', '3': 1, '4': 1, '5': 9, '10': 'code'},
    {
      '1': 'status',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.StepStatus',
      '10': 'status'
    },
    {'1': 'label', '3': 3, '4': 1, '5': 9, '10': 'label'},
    {'1': 'duration_ms', '3': 4, '4': 1, '5': 3, '10': 'durationMs'},
    {
      '1': 'safe_args',
      '3': 5,
      '4': 3,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.Step.SafeArgsEntry',
      '10': 'safeArgs'
    },
    {'1': 'text', '3': 6, '4': 1, '5': 9, '10': 'text'},
  ],
  '3': [Step_SafeArgsEntry$json],
};

@$core.Deprecated('Use stepDescriptor instead')
const Step_SafeArgsEntry$json = {
  '1': 'SafeArgsEntry',
  '2': [
    {'1': 'key', '3': 1, '4': 1, '5': 9, '10': 'key'},
    {'1': 'value', '3': 2, '4': 1, '5': 9, '10': 'value'},
  ],
  '7': {'7': true},
};

/// Descriptor for `Step`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List stepDescriptor = $convert.base64Decode(
    'CgRTdGVwEhIKBGNvZGUYASABKAlSBGNvZGUSPgoGc3RhdHVzGAIgASgOMiYuaGF3a3IucHJvY3'
    'VyZW1lbnQudjFhbHBoYTEuU3RlcFN0YXR1c1IGc3RhdHVzEhQKBWxhYmVsGAMgASgJUgVsYWJl'
    'bBIfCgtkdXJhdGlvbl9tcxgEIAEoA1IKZHVyYXRpb25NcxJLCglzYWZlX2FyZ3MYBSADKAsyLi'
    '5oYXdrci5wcm9jdXJlbWVudC52MWFscGhhMS5TdGVwLlNhZmVBcmdzRW50cnlSCHNhZmVBcmdz'
    'EhIKBHRleHQYBiABKAlSBHRleHQaOwoNU2FmZUFyZ3NFbnRyeRIQCgNrZXkYASABKAlSA2tleR'
    'IUCgV2YWx1ZRgCIAEoCVIFdmFsdWU6AjgB');

@$core.Deprecated('Use recordedFieldDescriptor instead')
const RecordedField$json = {
  '1': 'RecordedField',
  '2': [
    {
      '1': 'key',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.SlotKey',
      '10': 'key'
    },
    {'1': 'label', '3': 2, '4': 1, '5': 9, '10': 'label'},
    {'1': 'value', '3': 3, '4': 1, '5': 9, '10': 'value'},
  ],
};

/// Descriptor for `RecordedField`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List recordedFieldDescriptor = $convert.base64Decode(
    'Cg1SZWNvcmRlZEZpZWxkEjUKA2tleRgBIAEoDjIjLmhhd2tyLnByb2N1cmVtZW50LnYxYWxwaG'
    'ExLlNsb3RLZXlSA2tleRIUCgVsYWJlbBgCIAEoCVIFbGFiZWwSFAoFdmFsdWUYAyABKAlSBXZh'
    'bHVl');

@$core.Deprecated('Use rejectedFieldDescriptor instead')
const RejectedField$json = {
  '1': 'RejectedField',
  '2': [
    {
      '1': 'key',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.SlotKey',
      '10': 'key'
    },
    {
      '1': 'reason',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.RejectionReason',
      '10': 'reason'
    },
  ],
};

/// Descriptor for `RejectedField`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List rejectedFieldDescriptor = $convert.base64Decode(
    'Cg1SZWplY3RlZEZpZWxkEjUKA2tleRgBIAEoDjIjLmhhd2tyLnByb2N1cmVtZW50LnYxYWxwaG'
    'ExLlNsb3RLZXlSA2tleRJDCgZyZWFzb24YAiABKA4yKy5oYXdrci5wcm9jdXJlbWVudC52MWFs'
    'cGhhMS5SZWplY3Rpb25SZWFzb25SBnJlYXNvbg==');

@$core.Deprecated('Use itemResolutionDescriptor instead')
const ItemResolution$json = {
  '1': 'ItemResolution',
  '2': [
    {
      '1': 'kind',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.ItemResolutionKind',
      '10': 'kind'
    },
    {'1': 'label', '3': 2, '4': 1, '5': 9, '10': 'label'},
    {'1': 'display_value', '3': 3, '4': 1, '5': 9, '10': 'displayValue'},
  ],
};

/// Descriptor for `ItemResolution`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List itemResolutionDescriptor = $convert.base64Decode(
    'Cg5JdGVtUmVzb2x1dGlvbhJCCgRraW5kGAEgASgOMi4uaGF3a3IucHJvY3VyZW1lbnQudjFhbH'
    'BoYTEuSXRlbVJlc29sdXRpb25LaW5kUgRraW5kEhQKBWxhYmVsGAIgASgJUgVsYWJlbBIjCg1k'
    'aXNwbGF5X3ZhbHVlGAMgASgJUgxkaXNwbGF5VmFsdWU=');

@$core.Deprecated('Use slotsDescriptor instead')
const Slots$json = {
  '1': 'Slots',
  '2': [
    {'1': 'item', '3': 1, '4': 1, '5': 9, '9': 0, '10': 'item', '17': true},
    {
      '1': 'quantity',
      '3': 2,
      '4': 1,
      '5': 5,
      '9': 1,
      '10': 'quantity',
      '17': true
    },
    {'1': 'unit', '3': 3, '4': 1, '5': 9, '9': 2, '10': 'unit', '17': true},
    {
      '1': 'packaging',
      '3': 4,
      '4': 1,
      '5': 9,
      '9': 3,
      '10': 'packaging',
      '17': true
    },
    {
      '1': 'budget_per_unit',
      '3': 5,
      '4': 1,
      '5': 3,
      '9': 4,
      '10': 'budgetPerUnit',
      '17': true
    },
    {
      '1': 'delivery_city',
      '3': 6,
      '4': 1,
      '5': 9,
      '9': 5,
      '10': 'deliveryCity',
      '17': true
    },
    {
      '1': 'delivery_date',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.google.type.Date',
      '9': 6,
      '10': 'deliveryDate',
      '17': true
    },
    {
      '1': 'exact_match_required',
      '3': 8,
      '4': 1,
      '5': 8,
      '9': 7,
      '10': 'exactMatchRequired',
      '17': true
    },
    {
      '1': 'item_resolution',
      '3': 9,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.ItemResolution',
      '9': 8,
      '10': 'itemResolution',
      '17': true
    },
  ],
  '8': [
    {'1': '_item'},
    {'1': '_quantity'},
    {'1': '_unit'},
    {'1': '_packaging'},
    {'1': '_budget_per_unit'},
    {'1': '_delivery_city'},
    {'1': '_delivery_date'},
    {'1': '_exact_match_required'},
    {'1': '_item_resolution'},
  ],
};

/// Descriptor for `Slots`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List slotsDescriptor = $convert.base64Decode(
    'CgVTbG90cxIXCgRpdGVtGAEgASgJSABSBGl0ZW2IAQESHwoIcXVhbnRpdHkYAiABKAVIAVIIcX'
    'VhbnRpdHmIAQESFwoEdW5pdBgDIAEoCUgCUgR1bml0iAEBEiEKCXBhY2thZ2luZxgEIAEoCUgD'
    'UglwYWNrYWdpbmeIAQESKwoPYnVkZ2V0X3Blcl91bml0GAUgASgDSARSDWJ1ZGdldFBlclVuaX'
    'SIAQESKAoNZGVsaXZlcnlfY2l0eRgGIAEoCUgFUgxkZWxpdmVyeUNpdHmIAQESOwoNZGVsaXZl'
    'cnlfZGF0ZRgHIAEoCzIRLmdvb2dsZS50eXBlLkRhdGVIBlIMZGVsaXZlcnlEYXRliAEBEjUKFG'
    'V4YWN0X21hdGNoX3JlcXVpcmVkGAggASgISAdSEmV4YWN0TWF0Y2hSZXF1aXJlZIgBARJYCg9p'
    'dGVtX3Jlc29sdXRpb24YCSABKAsyKi5oYXdrci5wcm9jdXJlbWVudC52MWFscGhhMS5JdGVtUm'
    'Vzb2x1dGlvbkgIUg5pdGVtUmVzb2x1dGlvbogBAUIHCgVfaXRlbUILCglfcXVhbnRpdHlCBwoF'
    'X3VuaXRCDAoKX3BhY2thZ2luZ0ISChBfYnVkZ2V0X3Blcl91bml0QhAKDl9kZWxpdmVyeV9jaX'
    'R5QhAKDl9kZWxpdmVyeV9kYXRlQhcKFV9leGFjdF9tYXRjaF9yZXF1aXJlZEISChBfaXRlbV9y'
    'ZXNvbHV0aW9u');

@$core.Deprecated('Use blockDescriptor instead')
const Block$json = {
  '1': 'Block',
  '2': [
    {
      '1': 'kind',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.BlockKind',
      '10': 'kind'
    },
    {'1': 'text', '3': 2, '4': 1, '5': 9, '10': 'text'},
  ],
};

/// Descriptor for `Block`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List blockDescriptor = $convert.base64Decode(
    'CgVCbG9jaxI5CgRraW5kGAEgASgOMiUuaGF3a3IucHJvY3VyZW1lbnQudjFhbHBoYTEuQmxvY2'
    'tLaW5kUgRraW5kEhIKBHRleHQYAiABKAlSBHRleHQ=');

@$core.Deprecated('Use evidenceRefDescriptor instead')
const EvidenceRef$json = {
  '1': 'EvidenceRef',
  '2': [
    {'1': 'url', '3': 1, '4': 1, '5': 9, '10': 'url'},
    {'1': 'label', '3': 2, '4': 1, '5': 9, '10': 'label'},
  ],
};

/// Descriptor for `EvidenceRef`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List evidenceRefDescriptor = $convert.base64Decode(
    'CgtFdmlkZW5jZVJlZhIQCgN1cmwYASABKAlSA3VybBIUCgVsYWJlbBgCIAEoCVIFbGFiZWw=');

@$core.Deprecated('Use interruptDescriptor instead')
const Interrupt$json = {
  '1': 'Interrupt',
  '2': [
    {'1': 'interrupt_id', '3': 1, '4': 1, '5': 9, '10': 'interruptId'},
    {
      '1': 'kind',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.InterruptKind',
      '10': 'kind'
    },
  ],
};

/// Descriptor for `Interrupt`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List interruptDescriptor = $convert.base64Decode(
    'CglJbnRlcnJ1cHQSIQoMaW50ZXJydXB0X2lkGAEgASgJUgtpbnRlcnJ1cHRJZBI9CgRraW5kGA'
    'IgASgOMikuaGF3a3IucHJvY3VyZW1lbnQudjFhbHBoYTEuSW50ZXJydXB0S2luZFIEa2luZA==');

@$core.Deprecated('Use quotaSnapshotDescriptor instead')
const QuotaSnapshot$json = {
  '1': 'QuotaSnapshot',
  '2': [
    {'1': 'plan_code', '3': 1, '4': 1, '5': 9, '10': 'planCode'},
    {'1': 'remaining', '3': 2, '4': 1, '5': 5, '10': 'remaining'},
    {'1': 'limit', '3': 3, '4': 1, '5': 5, '10': 'limit'},
    {
      '1': 'reset_time',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'resetTime'
    },
  ],
};

/// Descriptor for `QuotaSnapshot`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List quotaSnapshotDescriptor = $convert.base64Decode(
    'Cg1RdW90YVNuYXBzaG90EhsKCXBsYW5fY29kZRgBIAEoCVIIcGxhbkNvZGUSHAoJcmVtYWluaW'
    '5nGAIgASgFUglyZW1haW5pbmcSFAoFbGltaXQYAyABKAVSBWxpbWl0EjkKCnJlc2V0X3RpbWUY'
    'BCABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wUglyZXNldFRpbWU=');

@$core.Deprecated('Use turnResultDescriptor instead')
const TurnResult$json = {
  '1': 'TurnResult',
  '2': [
    {'1': 'request_id', '3': 1, '4': 1, '5': 9, '10': 'requestId'},
    {'1': 'conversation_id', '3': 2, '4': 1, '5': 9, '10': 'conversationId'},
    {'1': 'revision_id', '3': 3, '4': 1, '5': 9, '10': 'revisionId'},
    {
      '1': 'slots',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.Slots',
      '10': 'slots'
    },
    {
      '1': 'rejected',
      '3': 5,
      '4': 3,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.RejectedField',
      '10': 'rejected'
    },
    {
      '1': 'missing_required',
      '3': 6,
      '4': 3,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.SlotKey',
      '10': 'missingRequired'
    },
    {
      '1': 'recorded_this_turn',
      '3': 7,
      '4': 3,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.RecordedField',
      '10': 'recordedThisTurn'
    },
    {'1': 'reply', '3': 10, '4': 1, '5': 9, '10': 'reply'},
    {
      '1': 'steps',
      '3': 11,
      '4': 3,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.Step',
      '10': 'steps'
    },
    {
      '1': 'question',
      '3': 12,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.ClarifyQuestion',
      '10': 'question'
    },
    {
      '1': 'blocks',
      '3': 13,
      '4': 3,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.Block',
      '10': 'blocks'
    },
    {'1': 'text_fallback', '3': 14, '4': 1, '5': 9, '10': 'textFallback'},
    {
      '1': 'evidence_refs',
      '3': 15,
      '4': 3,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.EvidenceRef',
      '10': 'evidenceRefs'
    },
    {'1': 'message_id', '3': 16, '4': 1, '5': 9, '10': 'messageId'},
    {
      '1': 'create_time',
      '3': 17,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'createTime'
    },
    {'1': 'run_id', '3': 18, '4': 1, '5': 9, '10': 'runId'},
    {
      '1': 'interrupt',
      '3': 19,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.Interrupt',
      '9': 0,
      '10': 'interrupt',
      '17': true
    },
    {
      '1': 'quota',
      '3': 20,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.QuotaSnapshot',
      '10': 'quota'
    },
  ],
  '8': [
    {'1': '_interrupt'},
  ],
  '9': [
    {'1': 8, '2': 9},
    {'1': 9, '2': 10},
  ],
  '10': ['degraded', 'degraded_reason'],
};

/// Descriptor for `TurnResult`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List turnResultDescriptor = $convert.base64Decode(
    'CgpUdXJuUmVzdWx0Eh0KCnJlcXVlc3RfaWQYASABKAlSCXJlcXVlc3RJZBInCg9jb252ZXJzYX'
    'Rpb25faWQYAiABKAlSDmNvbnZlcnNhdGlvbklkEh8KC3JldmlzaW9uX2lkGAMgASgJUgpyZXZp'
    'c2lvbklkEjcKBXNsb3RzGAQgASgLMiEuaGF3a3IucHJvY3VyZW1lbnQudjFhbHBoYTEuU2xvdH'
    'NSBXNsb3RzEkUKCHJlamVjdGVkGAUgAygLMikuaGF3a3IucHJvY3VyZW1lbnQudjFhbHBoYTEu'
    'UmVqZWN0ZWRGaWVsZFIIcmVqZWN0ZWQSTgoQbWlzc2luZ19yZXF1aXJlZBgGIAMoDjIjLmhhd2'
    'tyLnByb2N1cmVtZW50LnYxYWxwaGExLlNsb3RLZXlSD21pc3NpbmdSZXF1aXJlZBJXChJyZWNv'
    'cmRlZF90aGlzX3R1cm4YByADKAsyKS5oYXdrci5wcm9jdXJlbWVudC52MWFscGhhMS5SZWNvcm'
    'RlZEZpZWxkUhByZWNvcmRlZFRoaXNUdXJuEhQKBXJlcGx5GAogASgJUgVyZXBseRI2CgVzdGVw'
    'cxgLIAMoCzIgLmhhd2tyLnByb2N1cmVtZW50LnYxYWxwaGExLlN0ZXBSBXN0ZXBzEkcKCHF1ZX'
    'N0aW9uGAwgASgLMisuaGF3a3IucHJvY3VyZW1lbnQudjFhbHBoYTEuQ2xhcmlmeVF1ZXN0aW9u'
    'UghxdWVzdGlvbhI5CgZibG9ja3MYDSADKAsyIS5oYXdrci5wcm9jdXJlbWVudC52MWFscGhhMS'
    '5CbG9ja1IGYmxvY2tzEiMKDXRleHRfZmFsbGJhY2sYDiABKAlSDHRleHRGYWxsYmFjaxJMCg1l'
    'dmlkZW5jZV9yZWZzGA8gAygLMicuaGF3a3IucHJvY3VyZW1lbnQudjFhbHBoYTEuRXZpZGVuY2'
    'VSZWZSDGV2aWRlbmNlUmVmcxIdCgptZXNzYWdlX2lkGBAgASgJUgltZXNzYWdlSWQSQAoLY3Jl'
    'YXRlX3RpbWUYESABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wQgPgQQNSCmNyZWF0ZV'
    'RpbWUSFQoGcnVuX2lkGBIgASgJUgVydW5JZBJICglpbnRlcnJ1cHQYEyABKAsyJS5oYXdrci5w'
    'cm9jdXJlbWVudC52MWFscGhhMS5JbnRlcnJ1cHRIAFIJaW50ZXJydXB0iAEBEj8KBXF1b3RhGB'
    'QgASgLMikuaGF3a3IucHJvY3VyZW1lbnQudjFhbHBoYTEuUXVvdGFTbmFwc2hvdFIFcXVvdGFC'
    'DAoKX2ludGVycnVwdEoECAgQCUoECAkQClIIZGVncmFkZWRSD2RlZ3JhZGVkX3JlYXNvbg==');

@$core.Deprecated('Use runSummaryDescriptor instead')
const RunSummary$json = {
  '1': 'RunSummary',
  '2': [
    {'1': 'run_id', '3': 1, '4': 1, '5': 9, '10': 'runId'},
    {
      '1': 'run_type',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.RunType',
      '10': 'runType'
    },
    {
      '1': 'status',
      '3': 3,
      '4': 1,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.RunStatus',
      '10': 'status'
    },
    {'1': 'last_cursor', '3': 4, '4': 1, '5': 9, '10': 'lastCursor'},
    {'1': 'terminal', '3': 5, '4': 1, '5': 8, '10': 'terminal'},
  ],
};

/// Descriptor for `RunSummary`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List runSummaryDescriptor = $convert.base64Decode(
    'CgpSdW5TdW1tYXJ5EhUKBnJ1bl9pZBgBIAEoCVIFcnVuSWQSPgoIcnVuX3R5cGUYAiABKA4yIy'
    '5oYXdrci5wcm9jdXJlbWVudC52MWFscGhhMS5SdW5UeXBlUgdydW5UeXBlEj0KBnN0YXR1cxgD'
    'IAEoDjIlLmhhd2tyLnByb2N1cmVtZW50LnYxYWxwaGExLlJ1blN0YXR1c1IGc3RhdHVzEh8KC2'
    'xhc3RfY3Vyc29yGAQgASgJUgpsYXN0Q3Vyc29yEhoKCHRlcm1pbmFsGAUgASgIUgh0ZXJtaW5h'
    'bA==');

@$core.Deprecated('Use openConfirmationDescriptor instead')
const OpenConfirmation$json = {
  '1': 'OpenConfirmation',
  '2': [
    {'1': 'interrupt_id', '3': 1, '4': 1, '5': 9, '10': 'interruptId'},
    {
      '1': 'kind',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.InterruptKind',
      '10': 'kind'
    },
    {
      '1': 'question',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.ClarifyQuestion',
      '10': 'question'
    },
    {'1': 'processing', '3': 5, '4': 1, '5': 8, '10': 'processing'},
  ],
  '9': [
    {'1': 3, '2': 4},
  ],
  '10': ['expire_time'],
};

/// Descriptor for `OpenConfirmation`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List openConfirmationDescriptor = $convert.base64Decode(
    'ChBPcGVuQ29uZmlybWF0aW9uEiEKDGludGVycnVwdF9pZBgBIAEoCVILaW50ZXJydXB0SWQSPQ'
    'oEa2luZBgCIAEoDjIpLmhhd2tyLnByb2N1cmVtZW50LnYxYWxwaGExLkludGVycnVwdEtpbmRS'
    'BGtpbmQSRwoIcXVlc3Rpb24YBCABKAsyKy5oYXdrci5wcm9jdXJlbWVudC52MWFscGhhMS5DbG'
    'FyaWZ5UXVlc3Rpb25SCHF1ZXN0aW9uEh4KCnByb2Nlc3NpbmcYBSABKAhSCnByb2Nlc3NpbmdK'
    'BAgDEARSC2V4cGlyZV90aW1l');

@$core.Deprecated('Use publishedRfqDescriptor instead')
const PublishedRfq$json = {
  '1': 'PublishedRfq',
  '2': [
    {'1': 'publication_id', '3': 1, '4': 1, '5': 9, '10': 'publicationId'},
    {'1': 'version', '3': 2, '4': 1, '5': 9, '10': 'version'},
    {
      '1': 'publish_time',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'publishTime'
    },
    {'1': 'recipient_count', '3': 4, '4': 1, '5': 5, '10': 'recipientCount'},
    {'1': 'queued_count', '3': 5, '4': 1, '5': 5, '10': 'queuedCount'},
    {
      '1': 'seller_facing_hash',
      '3': 6,
      '4': 1,
      '5': 9,
      '10': 'sellerFacingHash'
    },
  ],
};

/// Descriptor for `PublishedRfq`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List publishedRfqDescriptor = $convert.base64Decode(
    'CgxQdWJsaXNoZWRSZnESJQoOcHVibGljYXRpb25faWQYASABKAlSDXB1YmxpY2F0aW9uSWQSGA'
    'oHdmVyc2lvbhgCIAEoCVIHdmVyc2lvbhI9CgxwdWJsaXNoX3RpbWUYAyABKAsyGi5nb29nbGUu'
    'cHJvdG9idWYuVGltZXN0YW1wUgtwdWJsaXNoVGltZRInCg9yZWNpcGllbnRfY291bnQYBCABKA'
    'VSDnJlY2lwaWVudENvdW50EiEKDHF1ZXVlZF9jb3VudBgFIAEoBVILcXVldWVkQ291bnQSLAoS'
    'c2VsbGVyX2ZhY2luZ19oYXNoGAYgASgJUhBzZWxsZXJGYWNpbmdIYXNo');

@$core.Deprecated('Use conversationMessageDescriptor instead')
const ConversationMessage$json = {
  '1': 'ConversationMessage',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'id'},
    {
      '1': 'role',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.ConversationRole',
      '10': 'role'
    },
    {'1': 'text', '3': 3, '4': 1, '5': 9, '10': 'text'},
    {
      '1': 'steps',
      '3': 4,
      '4': 3,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.Step',
      '10': 'steps'
    },
    {
      '1': 'recorded_this_turn',
      '3': 5,
      '4': 3,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.RecordedField',
      '10': 'recordedThisTurn'
    },
    {
      '1': 'create_time',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'createTime'
    },
  ],
};

/// Descriptor for `ConversationMessage`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List conversationMessageDescriptor = $convert.base64Decode(
    'ChNDb252ZXJzYXRpb25NZXNzYWdlEhMKAmlkGAEgASgJQgPgQQhSAmlkEkAKBHJvbGUYAiABKA'
    '4yLC5oYXdrci5wcm9jdXJlbWVudC52MWFscGhhMS5Db252ZXJzYXRpb25Sb2xlUgRyb2xlEhIK'
    'BHRleHQYAyABKAlSBHRleHQSNgoFc3RlcHMYBCADKAsyIC5oYXdrci5wcm9jdXJlbWVudC52MW'
    'FscGhhMS5TdGVwUgVzdGVwcxJXChJyZWNvcmRlZF90aGlzX3R1cm4YBSADKAsyKS5oYXdrci5w'
    'cm9jdXJlbWVudC52MWFscGhhMS5SZWNvcmRlZEZpZWxkUhByZWNvcmRlZFRoaXNUdXJuEkAKC2'
    'NyZWF0ZV90aW1lGAYgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcEID4EEDUgpjcmVh'
    'dGVUaW1l');

@$core.Deprecated('Use conversationSummaryDescriptor instead')
const ConversationSummary$json = {
  '1': 'ConversationSummary',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'id'},
    {'1': 'title', '3': 2, '4': 1, '5': 9, '10': 'title'},
    {
      '1': 'last_message_preview',
      '3': 3,
      '4': 1,
      '5': 9,
      '10': 'lastMessagePreview'
    },
    {
      '1': 'update_time',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'updateTime'
    },
  ],
};

/// Descriptor for `ConversationSummary`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List conversationSummaryDescriptor = $convert.base64Decode(
    'ChNDb252ZXJzYXRpb25TdW1tYXJ5EhMKAmlkGAEgASgJQgPgQQhSAmlkEhQKBXRpdGxlGAIgAS'
    'gJUgV0aXRsZRIwChRsYXN0X21lc3NhZ2VfcHJldmlldxgDIAEoCVISbGFzdE1lc3NhZ2VQcmV2'
    'aWV3EkAKC3VwZGF0ZV90aW1lGAQgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcEID4E'
    'EDUgp1cGRhdGVUaW1l');

@$core.Deprecated('Use submitTurnRequestDescriptor instead')
const SubmitTurnRequest$json = {
  '1': 'SubmitTurnRequest',
  '2': [
    {'1': 'request_id', '3': 1, '4': 1, '5': 9, '10': 'requestId'},
    {'1': 'message', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'message'},
    {
      '1': 'client_request_id',
      '3': 3,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'clientRequestId'
    },
  ],
};

/// Descriptor for `SubmitTurnRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List submitTurnRequestDescriptor = $convert.base64Decode(
    'ChFTdWJtaXRUdXJuUmVxdWVzdBIdCgpyZXF1ZXN0X2lkGAEgASgJUglyZXF1ZXN0SWQSHQoHbW'
    'Vzc2FnZRgCIAEoCUID4EECUgdtZXNzYWdlEjcKEWNsaWVudF9yZXF1ZXN0X2lkGAMgASgJQgvg'
    'QQLijM/XCAIIAVIPY2xpZW50UmVxdWVzdElk');

@$core.Deprecated('Use submitTurnResponseDescriptor instead')
const SubmitTurnResponse$json = {
  '1': 'SubmitTurnResponse',
  '2': [
    {'1': 'request_id', '3': 1, '4': 1, '5': 9, '10': 'requestId'},
    {'1': 'conversation_id', '3': 2, '4': 1, '5': 9, '10': 'conversationId'},
    {'1': 'run_id', '3': 3, '4': 1, '5': 9, '10': 'runId'},
  ],
  '9': [
    {'1': 4, '2': 5},
  ],
  '10': ['run_version'],
};

/// Descriptor for `SubmitTurnResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List submitTurnResponseDescriptor = $convert.base64Decode(
    'ChJTdWJtaXRUdXJuUmVzcG9uc2USHQoKcmVxdWVzdF9pZBgBIAEoCVIJcmVxdWVzdElkEicKD2'
    'NvbnZlcnNhdGlvbl9pZBgCIAEoCVIOY29udmVyc2F0aW9uSWQSFQoGcnVuX2lkGAMgASgJUgVy'
    'dW5JZEoECAQQBVILcnVuX3ZlcnNpb24=');

@$core.Deprecated('Use getSnapshotRequestDescriptor instead')
const GetSnapshotRequest$json = {
  '1': 'GetSnapshotRequest',
  '2': [
    {'1': 'request_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'requestId'},
  ],
};

/// Descriptor for `GetSnapshotRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getSnapshotRequestDescriptor = $convert.base64Decode(
    'ChJHZXRTbmFwc2hvdFJlcXVlc3QSIgoKcmVxdWVzdF9pZBgBIAEoCUID4EECUglyZXF1ZXN0SW'
    'Q=');

@$core.Deprecated('Use getSnapshotResponseDescriptor instead')
const GetSnapshotResponse$json = {
  '1': 'GetSnapshotResponse',
  '2': [
    {'1': 'request_id', '3': 1, '4': 1, '5': 9, '10': 'requestId'},
    {'1': 'revision_id', '3': 2, '4': 1, '5': 9, '10': 'revisionId'},
    {
      '1': 'status',
      '3': 3,
      '4': 1,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.RequestStatus',
      '10': 'status'
    },
    {
      '1': 'slots',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.Slots',
      '10': 'slots'
    },
    {
      '1': 'missing_required',
      '3': 5,
      '4': 3,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.SlotKey',
      '10': 'missingRequired'
    },
    {
      '1': 'runs',
      '3': 7,
      '4': 3,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.RunSummary',
      '10': 'runs'
    },
    {'1': 'active_run_id', '3': 8, '4': 1, '5': 9, '10': 'activeRunId'},
    {
      '1': 'open_confirmation',
      '3': 9,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.OpenConfirmation',
      '9': 0,
      '10': 'openConfirmation',
      '17': true
    },
    {
      '1': 'published_rfq',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.PublishedRfq',
      '9': 1,
      '10': 'publishedRfq',
      '17': true
    },
  ],
  '8': [
    {'1': '_open_confirmation'},
    {'1': '_published_rfq'},
  ],
  '9': [
    {'1': 6, '2': 7},
  ],
  '10': ['item_disambiguation'],
};

/// Descriptor for `GetSnapshotResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getSnapshotResponseDescriptor = $convert.base64Decode(
    'ChNHZXRTbmFwc2hvdFJlc3BvbnNlEh0KCnJlcXVlc3RfaWQYASABKAlSCXJlcXVlc3RJZBIfCg'
    'tyZXZpc2lvbl9pZBgCIAEoCVIKcmV2aXNpb25JZBJBCgZzdGF0dXMYAyABKA4yKS5oYXdrci5w'
    'cm9jdXJlbWVudC52MWFscGhhMS5SZXF1ZXN0U3RhdHVzUgZzdGF0dXMSNwoFc2xvdHMYBCABKA'
    'syIS5oYXdrci5wcm9jdXJlbWVudC52MWFscGhhMS5TbG90c1IFc2xvdHMSTgoQbWlzc2luZ19y'
    'ZXF1aXJlZBgFIAMoDjIjLmhhd2tyLnByb2N1cmVtZW50LnYxYWxwaGExLlNsb3RLZXlSD21pc3'
    'NpbmdSZXF1aXJlZBI6CgRydW5zGAcgAygLMiYuaGF3a3IucHJvY3VyZW1lbnQudjFhbHBoYTEu'
    'UnVuU3VtbWFyeVIEcnVucxIiCg1hY3RpdmVfcnVuX2lkGAggASgJUgthY3RpdmVSdW5JZBJeCh'
    'FvcGVuX2NvbmZpcm1hdGlvbhgJIAEoCzIsLmhhd2tyLnByb2N1cmVtZW50LnYxYWxwaGExLk9w'
    'ZW5Db25maXJtYXRpb25IAFIQb3BlbkNvbmZpcm1hdGlvbogBARJSCg1wdWJsaXNoZWRfcmZxGA'
    'ogASgLMiguaGF3a3IucHJvY3VyZW1lbnQudjFhbHBoYTEuUHVibGlzaGVkUmZxSAFSDHB1Ymxp'
    'c2hlZFJmcYgBAUIUChJfb3Blbl9jb25maXJtYXRpb25CEAoOX3B1Ymxpc2hlZF9yZnFKBAgGEA'
    'dSE2l0ZW1fZGlzYW1iaWd1YXRpb24=');

@$core.Deprecated('Use matchedFactDescriptor instead')
const MatchedFact$json = {
  '1': 'MatchedFact',
  '2': [
    {
      '1': 'slot_key',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.SlotKey',
      '10': 'slotKey'
    },
    {'1': 'label', '3': 2, '4': 1, '5': 9, '10': 'label'},
  ],
};

/// Descriptor for `MatchedFact`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List matchedFactDescriptor = $convert.base64Decode(
    'CgtNYXRjaGVkRmFjdBI+CghzbG90X2tleRgBIAEoDjIjLmhhd2tyLnByb2N1cmVtZW50LnYxYW'
    'xwaGExLlNsb3RLZXlSB3Nsb3RLZXkSFAoFbGFiZWwYAiABKAlSBWxhYmVs');

@$core.Deprecated('Use unverifiableFactDescriptor instead')
const UnverifiableFact$json = {
  '1': 'UnverifiableFact',
  '2': [
    {
      '1': 'slot_key',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.SlotKey',
      '10': 'slotKey'
    },
    {'1': 'reason', '3': 2, '4': 1, '5': 9, '10': 'reason'},
  ],
};

/// Descriptor for `UnverifiableFact`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List unverifiableFactDescriptor = $convert.base64Decode(
    'ChBVbnZlcmlmaWFibGVGYWN0Ej4KCHNsb3Rfa2V5GAEgASgOMiMuaGF3a3IucHJvY3VyZW1lbn'
    'QudjFhbHBoYTEuU2xvdEtleVIHc2xvdEtleRIWCgZyZWFzb24YAiABKAlSBnJlYXNvbg==');

@$core.Deprecated('Use candidateDescriptor instead')
const Candidate$json = {
  '1': 'Candidate',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'id'},
    {
      '1': 'matched_facts',
      '3': 4,
      '4': 3,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.MatchedFact',
      '10': 'matchedFacts'
    },
    {
      '1': 'unverifiable_facts',
      '3': 5,
      '4': 3,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.UnverifiableFact',
      '10': 'unverifiableFacts'
    },
    {
      '1': 'facts_time',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'factsTime'
    },
    {'1': 'selected', '3': 7, '4': 1, '5': 8, '10': 'selected'},
    {
      '1': 'store',
      '3': 8,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.StoreCard',
      '10': 'store'
    },
  ],
  '9': [
    {'1': 3, '2': 4},
  ],
  '10': ['score'],
};

/// Descriptor for `Candidate`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List candidateDescriptor = $convert.base64Decode(
    'CglDYW5kaWRhdGUSEwoCaWQYASABKAlCA+BBCFICaWQSTAoNbWF0Y2hlZF9mYWN0cxgEIAMoCz'
    'InLmhhd2tyLnByb2N1cmVtZW50LnYxYWxwaGExLk1hdGNoZWRGYWN0UgxtYXRjaGVkRmFjdHMS'
    'WwoSdW52ZXJpZmlhYmxlX2ZhY3RzGAUgAygLMiwuaGF3a3IucHJvY3VyZW1lbnQudjFhbHBoYT'
    'EuVW52ZXJpZmlhYmxlRmFjdFIRdW52ZXJpZmlhYmxlRmFjdHMSOQoKZmFjdHNfdGltZRgGIAEo'
    'CzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBSCWZhY3RzVGltZRIaCghzZWxlY3RlZBgHIA'
    'EoCFIIc2VsZWN0ZWQSNgoFc3RvcmUYCCABKAsyIC5oYXdrci5jb21tb24udjFhbHBoYTEuU3Rv'
    'cmVDYXJkUgVzdG9yZUoECAMQBFIFc2NvcmU=');

@$core.Deprecated('Use startCandidateSearchRequestDescriptor instead')
const StartCandidateSearchRequest$json = {
  '1': 'StartCandidateSearchRequest',
  '2': [
    {'1': 'request_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'requestId'},
    {
      '1': 'client_request_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'clientRequestId'
    },
  ],
};

/// Descriptor for `StartCandidateSearchRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List startCandidateSearchRequestDescriptor =
    $convert.base64Decode(
        'ChtTdGFydENhbmRpZGF0ZVNlYXJjaFJlcXVlc3QSIgoKcmVxdWVzdF9pZBgBIAEoCUID4EECUg'
        'lyZXF1ZXN0SWQSNwoRY2xpZW50X3JlcXVlc3RfaWQYAiABKAlCC+BBAuKMz9cIAggBUg9jbGll'
        'bnRSZXF1ZXN0SWQ=');

@$core.Deprecated('Use startCandidateSearchResponseDescriptor instead')
const StartCandidateSearchResponse$json = {
  '1': 'StartCandidateSearchResponse',
  '2': [
    {'1': 'request_id', '3': 1, '4': 1, '5': 9, '10': 'requestId'},
    {'1': 'run_id', '3': 2, '4': 1, '5': 9, '10': 'runId'},
    {
      '1': 'status',
      '3': 3,
      '4': 1,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.RunStatus',
      '10': 'status'
    },
    {'1': 'candidate_set_id', '3': 4, '4': 1, '5': 9, '10': 'candidateSetId'},
  ],
  '9': [
    {'1': 5, '2': 6},
  ],
  '10': ['run_version'],
};

/// Descriptor for `StartCandidateSearchResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List startCandidateSearchResponseDescriptor = $convert.base64Decode(
    'ChxTdGFydENhbmRpZGF0ZVNlYXJjaFJlc3BvbnNlEh0KCnJlcXVlc3RfaWQYASABKAlSCXJlcX'
    'Vlc3RJZBIVCgZydW5faWQYAiABKAlSBXJ1bklkEj0KBnN0YXR1cxgDIAEoDjIlLmhhd2tyLnBy'
    'b2N1cmVtZW50LnYxYWxwaGExLlJ1blN0YXR1c1IGc3RhdHVzEigKEGNhbmRpZGF0ZV9zZXRfaW'
    'QYBCABKAlSDmNhbmRpZGF0ZVNldElkSgQIBRAGUgtydW5fdmVyc2lvbg==');

@$core.Deprecated('Use getCandidateSearchRequestDescriptor instead')
const GetCandidateSearchRequest$json = {
  '1': 'GetCandidateSearchRequest',
  '2': [
    {'1': 'request_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'requestId'},
    {'1': 'run_id', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'runId'},
  ],
};

/// Descriptor for `GetCandidateSearchRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getCandidateSearchRequestDescriptor =
    $convert.base64Decode(
        'ChlHZXRDYW5kaWRhdGVTZWFyY2hSZXF1ZXN0EiIKCnJlcXVlc3RfaWQYASABKAlCA+BBAlIJcm'
        'VxdWVzdElkEhoKBnJ1bl9pZBgCIAEoCUID4EECUgVydW5JZA==');

@$core.Deprecated('Use getCandidateSearchResponseDescriptor instead')
const GetCandidateSearchResponse$json = {
  '1': 'GetCandidateSearchResponse',
  '2': [
    {'1': 'request_id', '3': 1, '4': 1, '5': 9, '10': 'requestId'},
    {'1': 'run_id', '3': 2, '4': 1, '5': 9, '10': 'runId'},
    {'1': 'candidate_set_id', '3': 3, '4': 1, '5': 9, '10': 'candidateSetId'},
    {
      '1': 'status',
      '3': 4,
      '4': 1,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.RunStatus',
      '10': 'status'
    },
    {
      '1': 'failure_reason',
      '3': 5,
      '4': 1,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.RunFailureReason',
      '10': 'failureReason'
    },
    {
      '1': 'candidates',
      '3': 6,
      '4': 3,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.Candidate',
      '10': 'candidates'
    },
  ],
};

/// Descriptor for `GetCandidateSearchResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getCandidateSearchResponseDescriptor = $convert.base64Decode(
    'ChpHZXRDYW5kaWRhdGVTZWFyY2hSZXNwb25zZRIdCgpyZXF1ZXN0X2lkGAEgASgJUglyZXF1ZX'
    'N0SWQSFQoGcnVuX2lkGAIgASgJUgVydW5JZBIoChBjYW5kaWRhdGVfc2V0X2lkGAMgASgJUg5j'
    'YW5kaWRhdGVTZXRJZBI9CgZzdGF0dXMYBCABKA4yJS5oYXdrci5wcm9jdXJlbWVudC52MWFscG'
    'hhMS5SdW5TdGF0dXNSBnN0YXR1cxJTCg5mYWlsdXJlX3JlYXNvbhgFIAEoDjIsLmhhd2tyLnBy'
    'b2N1cmVtZW50LnYxYWxwaGExLlJ1bkZhaWx1cmVSZWFzb25SDWZhaWx1cmVSZWFzb24SRQoKY2'
    'FuZGlkYXRlcxgGIAMoCzIlLmhhd2tyLnByb2N1cmVtZW50LnYxYWxwaGExLkNhbmRpZGF0ZVIK'
    'Y2FuZGlkYXRlcw==');

@$core.Deprecated('Use toggleSelectionRequestDescriptor instead')
const ToggleSelectionRequest$json = {
  '1': 'ToggleSelectionRequest',
  '2': [
    {'1': 'request_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'requestId'},
    {
      '1': 'candidate_set_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'candidateSetId'
    },
    {'1': 'candidate_id', '3': 3, '4': 1, '5': 9, '8': {}, '10': 'candidateId'},
    {'1': 'selected', '3': 4, '4': 1, '5': 8, '10': 'selected'},
  ],
};

/// Descriptor for `ToggleSelectionRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List toggleSelectionRequestDescriptor = $convert.base64Decode(
    'ChZUb2dnbGVTZWxlY3Rpb25SZXF1ZXN0EiIKCnJlcXVlc3RfaWQYASABKAlCA+BBAlIJcmVxdW'
    'VzdElkEi0KEGNhbmRpZGF0ZV9zZXRfaWQYAiABKAlCA+BBAlIOY2FuZGlkYXRlU2V0SWQSJgoM'
    'Y2FuZGlkYXRlX2lkGAMgASgJQgPgQQJSC2NhbmRpZGF0ZUlkEhoKCHNlbGVjdGVkGAQgASgIUg'
    'hzZWxlY3RlZA==');

@$core.Deprecated('Use toggleSelectionResponseDescriptor instead')
const ToggleSelectionResponse$json = {
  '1': 'ToggleSelectionResponse',
  '2': [
    {'1': 'request_id', '3': 1, '4': 1, '5': 9, '10': 'requestId'},
    {'1': 'candidate_set_id', '3': 2, '4': 1, '5': 9, '10': 'candidateSetId'},
    {'1': 'candidate_id', '3': 3, '4': 1, '5': 9, '10': 'candidateId'},
    {'1': 'selected', '3': 4, '4': 1, '5': 8, '10': 'selected'},
  ],
};

/// Descriptor for `ToggleSelectionResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List toggleSelectionResponseDescriptor = $convert.base64Decode(
    'ChdUb2dnbGVTZWxlY3Rpb25SZXNwb25zZRIdCgpyZXF1ZXN0X2lkGAEgASgJUglyZXF1ZXN0SW'
    'QSKAoQY2FuZGlkYXRlX3NldF9pZBgCIAEoCVIOY2FuZGlkYXRlU2V0SWQSIQoMY2FuZGlkYXRl'
    'X2lkGAMgASgJUgtjYW5kaWRhdGVJZBIaCghzZWxlY3RlZBgEIAEoCFIIc2VsZWN0ZWQ=');

@$core.Deprecated('Use submitAnswerRequestDescriptor instead')
const SubmitAnswerRequest$json = {
  '1': 'SubmitAnswerRequest',
  '2': [
    {'1': 'request_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'requestId'},
    {
      '1': 'slot_key',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.SlotKey',
      '8': {},
      '10': 'slotKey'
    },
    {'1': 'option_id', '3': 3, '4': 1, '5': 9, '10': 'optionId'},
    {'1': 'free_text', '3': 4, '4': 1, '5': 9, '10': 'freeText'},
    {'1': 'unit_option_id', '3': 5, '4': 1, '5': 9, '10': 'unitOptionId'},
    {'1': 'skip', '3': 6, '4': 1, '5': 8, '10': 'skip'},
    {
      '1': 'expected_revision_id',
      '3': 7,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'expectedRevisionId'
    },
    {
      '1': 'client_request_id',
      '3': 8,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'clientRequestId'
    },
  ],
};

/// Descriptor for `SubmitAnswerRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List submitAnswerRequestDescriptor = $convert.base64Decode(
    'ChNTdWJtaXRBbnN3ZXJSZXF1ZXN0EiIKCnJlcXVlc3RfaWQYASABKAlCA+BBAlIJcmVxdWVzdE'
    'lkEkMKCHNsb3Rfa2V5GAIgASgOMiMuaGF3a3IucHJvY3VyZW1lbnQudjFhbHBoYTEuU2xvdEtl'
    'eUID4EECUgdzbG90S2V5EhsKCW9wdGlvbl9pZBgDIAEoCVIIb3B0aW9uSWQSGwoJZnJlZV90ZX'
    'h0GAQgASgJUghmcmVlVGV4dBIkCg51bml0X29wdGlvbl9pZBgFIAEoCVIMdW5pdE9wdGlvbklk'
    'EhIKBHNraXAYBiABKAhSBHNraXASNQoUZXhwZWN0ZWRfcmV2aXNpb25faWQYByABKAlCA+BBAl'
    'ISZXhwZWN0ZWRSZXZpc2lvbklkEjcKEWNsaWVudF9yZXF1ZXN0X2lkGAggASgJQgvgQQLijM/X'
    'CAIIAVIPY2xpZW50UmVxdWVzdElk');

@$core.Deprecated('Use submitAnswerResponseDescriptor instead')
const SubmitAnswerResponse$json = {
  '1': 'SubmitAnswerResponse',
  '2': [
    {
      '1': 'result',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.TurnResult',
      '10': 'result'
    },
  ],
};

/// Descriptor for `SubmitAnswerResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List submitAnswerResponseDescriptor = $convert.base64Decode(
    'ChRTdWJtaXRBbnN3ZXJSZXNwb25zZRI+CgZyZXN1bHQYASABKAsyJi5oYXdrci5wcm9jdXJlbW'
    'VudC52MWFscGhhMS5UdXJuUmVzdWx0UgZyZXN1bHQ=');

@$core.Deprecated('Use rewindRequestDescriptor instead')
const RewindRequest$json = {
  '1': 'RewindRequest',
  '2': [
    {'1': 'request_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'requestId'},
    {
      '1': 'slot_key',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.SlotKey',
      '8': {},
      '10': 'slotKey'
    },
    {
      '1': 'client_request_id',
      '3': 3,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'clientRequestId'
    },
    {
      '1': 'expected_revision_id',
      '3': 4,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'expectedRevisionId'
    },
  ],
};

/// Descriptor for `RewindRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List rewindRequestDescriptor = $convert.base64Decode(
    'Cg1SZXdpbmRSZXF1ZXN0EiIKCnJlcXVlc3RfaWQYASABKAlCA+BBAlIJcmVxdWVzdElkEkMKCH'
    'Nsb3Rfa2V5GAIgASgOMiMuaGF3a3IucHJvY3VyZW1lbnQudjFhbHBoYTEuU2xvdEtleUID4EEC'
    'UgdzbG90S2V5EjcKEWNsaWVudF9yZXF1ZXN0X2lkGAMgASgJQgvgQQLijM/XCAIIAVIPY2xpZW'
    '50UmVxdWVzdElkEjUKFGV4cGVjdGVkX3JldmlzaW9uX2lkGAQgASgJQgPgQQJSEmV4cGVjdGVk'
    'UmV2aXNpb25JZA==');

@$core.Deprecated('Use rewindResponseDescriptor instead')
const RewindResponse$json = {
  '1': 'RewindResponse',
  '2': [
    {
      '1': 'result',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.TurnResult',
      '10': 'result'
    },
  ],
};

/// Descriptor for `RewindResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List rewindResponseDescriptor = $convert.base64Decode(
    'Cg5SZXdpbmRSZXNwb25zZRI+CgZyZXN1bHQYASABKAsyJi5oYXdrci5wcm9jdXJlbWVudC52MW'
    'FscGhhMS5UdXJuUmVzdWx0UgZyZXN1bHQ=');

@$core.Deprecated('Use buyerRequestedRangeDescriptor instead')
const BuyerRequestedRange$json = {
  '1': 'BuyerRequestedRange',
  '2': [
    {
      '1': 'lower_bound_amount',
      '3': 1,
      '4': 1,
      '5': 3,
      '8': {},
      '10': 'lowerBoundAmount'
    },
    {
      '1': 'upper_bound_amount',
      '3': 2,
      '4': 1,
      '5': 3,
      '8': {},
      '10': 'upperBoundAmount'
    },
    {
      '1': 'basis_quantity',
      '3': 3,
      '4': 1,
      '5': 5,
      '8': {},
      '10': 'basisQuantity'
    },
    {'1': 'basis_unit', '3': 4, '4': 1, '5': 9, '8': {}, '10': 'basisUnit'},
  ],
};

/// Descriptor for `BuyerRequestedRange`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List buyerRequestedRangeDescriptor = $convert.base64Decode(
    'ChNCdXllclJlcXVlc3RlZFJhbmdlEjEKEmxvd2VyX2JvdW5kX2Ftb3VudBgBIAEoA0ID4EECUh'
    'Bsb3dlckJvdW5kQW1vdW50EjEKEnVwcGVyX2JvdW5kX2Ftb3VudBgCIAEoA0ID4EECUhB1cHBl'
    'ckJvdW5kQW1vdW50EioKDmJhc2lzX3F1YW50aXR5GAMgASgFQgPgQQNSDWJhc2lzUXVhbnRpdH'
    'kSIgoKYmFzaXNfdW5pdBgEIAEoCUID4EEDUgliYXNpc1VuaXQ=');

@$core.Deprecated('Use quoteTotalRangeDescriptor instead')
const QuoteTotalRange$json = {
  '1': 'QuoteTotalRange',
  '2': [
    {
      '1': 'lower_bound_amount',
      '3': 1,
      '4': 1,
      '5': 3,
      '10': 'lowerBoundAmount'
    },
    {
      '1': 'upper_bound_amount',
      '3': 2,
      '4': 1,
      '5': 3,
      '10': 'upperBoundAmount'
    },
    {'1': 'quantity', '3': 3, '4': 1, '5': 5, '10': 'quantity'},
    {'1': 'quantity_unit', '3': 4, '4': 1, '5': 9, '10': 'quantityUnit'},
  ],
};

/// Descriptor for `QuoteTotalRange`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List quoteTotalRangeDescriptor = $convert.base64Decode(
    'Cg9RdW90ZVRvdGFsUmFuZ2USLAoSbG93ZXJfYm91bmRfYW1vdW50GAEgASgDUhBsb3dlckJvdW'
    '5kQW1vdW50EiwKEnVwcGVyX2JvdW5kX2Ftb3VudBgCIAEoA1IQdXBwZXJCb3VuZEFtb3VudBIa'
    'CghxdWFudGl0eRgDIAEoBVIIcXVhbnRpdHkSIwoNcXVhbnRpdHlfdW5pdBgEIAEoCVIMcXVhbn'
    'RpdHlVbml0');

@$core.Deprecated('Use previewRfqPriceRequestDescriptor instead')
const PreviewRfqPriceRequest$json = {
  '1': 'PreviewRfqPriceRequest',
  '2': [
    {'1': 'request_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'requestId'},
    {
      '1': 'expected_revision_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'expectedRevisionId'
    },
    {
      '1': 'buyer_requested_range',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.BuyerRequestedRange',
      '8': {},
      '10': 'buyerRequestedRange'
    },
  ],
};

/// Descriptor for `PreviewRfqPriceRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List previewRfqPriceRequestDescriptor = $convert.base64Decode(
    'ChZQcmV2aWV3UmZxUHJpY2VSZXF1ZXN0EiIKCnJlcXVlc3RfaWQYASABKAlCA+BBAlIJcmVxdW'
    'VzdElkEjUKFGV4cGVjdGVkX3JldmlzaW9uX2lkGAIgASgJQgPgQQJSEmV4cGVjdGVkUmV2aXNp'
    'b25JZBJoChVidXllcl9yZXF1ZXN0ZWRfcmFuZ2UYAyABKAsyLy5oYXdrci5wcm9jdXJlbWVudC'
    '52MWFscGhhMS5CdXllclJlcXVlc3RlZFJhbmdlQgPgQQJSE2J1eWVyUmVxdWVzdGVkUmFuZ2U=');

@$core.Deprecated('Use previewRfqPriceResponseDescriptor instead')
const PreviewRfqPriceResponse$json = {
  '1': 'PreviewRfqPriceResponse',
  '2': [
    {
      '1': 'buyer_requested_range',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.BuyerRequestedRange',
      '10': 'buyerRequestedRange'
    },
    {
      '1': 'quote_total_range',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.QuoteTotalRange',
      '9': 0,
      '10': 'quoteTotalRange',
      '17': true
    },
  ],
  '8': [
    {'1': '_quote_total_range'},
  ],
};

/// Descriptor for `PreviewRfqPriceResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List previewRfqPriceResponseDescriptor = $convert.base64Decode(
    'ChdQcmV2aWV3UmZxUHJpY2VSZXNwb25zZRJjChVidXllcl9yZXF1ZXN0ZWRfcmFuZ2UYASABKA'
    'syLy5oYXdrci5wcm9jdXJlbWVudC52MWFscGhhMS5CdXllclJlcXVlc3RlZFJhbmdlUhNidXll'
    'clJlcXVlc3RlZFJhbmdlElwKEXF1b3RlX3RvdGFsX3JhbmdlGAIgASgLMisuaGF3a3IucHJvY3'
    'VyZW1lbnQudjFhbHBoYTEuUXVvdGVUb3RhbFJhbmdlSABSD3F1b3RlVG90YWxSYW5nZYgBAUIU'
    'ChJfcXVvdGVfdG90YWxfcmFuZ2U=');

@$core.Deprecated('Use sellerFacingRequestedRangeDescriptor instead')
const SellerFacingRequestedRange$json = {
  '1': 'SellerFacingRequestedRange',
  '2': [
    {'1': 'basis_quantity', '3': 1, '4': 1, '5': 5, '10': 'basisQuantity'},
    {'1': 'basis_unit', '3': 2, '4': 1, '5': 9, '10': 'basisUnit'},
    {
      '1': 'lower_bound_amount',
      '3': 3,
      '4': 1,
      '5': 3,
      '10': 'lowerBoundAmount'
    },
    {
      '1': 'upper_bound_amount',
      '3': 4,
      '4': 1,
      '5': 3,
      '10': 'upperBoundAmount'
    },
  ],
};

/// Descriptor for `SellerFacingRequestedRange`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sellerFacingRequestedRangeDescriptor = $convert.base64Decode(
    'ChpTZWxsZXJGYWNpbmdSZXF1ZXN0ZWRSYW5nZRIlCg5iYXNpc19xdWFudGl0eRgBIAEoBVINYm'
    'FzaXNRdWFudGl0eRIdCgpiYXNpc191bml0GAIgASgJUgliYXNpc1VuaXQSLAoSbG93ZXJfYm91'
    'bmRfYW1vdW50GAMgASgDUhBsb3dlckJvdW5kQW1vdW50EiwKEnVwcGVyX2JvdW5kX2Ftb3VudB'
    'gEIAEoA1IQdXBwZXJCb3VuZEFtb3VudA==');

@$core.Deprecated('Use sellerFacingRfqDescriptor instead')
const SellerFacingRfq$json = {
  '1': 'SellerFacingRfq',
  '2': [
    {'1': 'item', '3': 1, '4': 1, '5': 9, '10': 'item'},
    {'1': 'item_label', '3': 2, '4': 1, '5': 9, '10': 'itemLabel'},
    {
      '1': 'quantity',
      '3': 3,
      '4': 1,
      '5': 5,
      '9': 0,
      '10': 'quantity',
      '17': true
    },
    {'1': 'unit', '3': 4, '4': 1, '5': 9, '9': 1, '10': 'unit', '17': true},
    {
      '1': 'packaging',
      '3': 5,
      '4': 1,
      '5': 9,
      '9': 2,
      '10': 'packaging',
      '17': true
    },
    {'1': 'delivery_city', '3': 6, '4': 1, '5': 9, '10': 'deliveryCity'},
    {
      '1': 'delivery_date',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.google.type.Date',
      '9': 3,
      '10': 'deliveryDate',
      '17': true
    },
    {
      '1': 'response_deadline',
      '3': 8,
      '4': 1,
      '5': 11,
      '6': '.google.type.Date',
      '9': 4,
      '10': 'responseDeadline',
      '17': true
    },
    {'1': 'buyer_note', '3': 9, '4': 1, '5': 9, '10': 'buyerNote'},
    {
      '1': 'buyer_requested_range',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.SellerFacingRequestedRange',
      '10': 'buyerRequestedRange'
    },
    {
      '1': 'publication_version',
      '3': 11,
      '4': 1,
      '5': 9,
      '10': 'publicationVersion'
    },
  ],
  '8': [
    {'1': '_quantity'},
    {'1': '_unit'},
    {'1': '_packaging'},
    {'1': '_delivery_date'},
    {'1': '_response_deadline'},
  ],
};

/// Descriptor for `SellerFacingRfq`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sellerFacingRfqDescriptor = $convert.base64Decode(
    'Cg9TZWxsZXJGYWNpbmdSZnESEgoEaXRlbRgBIAEoCVIEaXRlbRIdCgppdGVtX2xhYmVsGAIgAS'
    'gJUglpdGVtTGFiZWwSHwoIcXVhbnRpdHkYAyABKAVIAFIIcXVhbnRpdHmIAQESFwoEdW5pdBgE'
    'IAEoCUgBUgR1bml0iAEBEiEKCXBhY2thZ2luZxgFIAEoCUgCUglwYWNrYWdpbmeIAQESIwoNZG'
    'VsaXZlcnlfY2l0eRgGIAEoCVIMZGVsaXZlcnlDaXR5EjsKDWRlbGl2ZXJ5X2RhdGUYByABKAsy'
    'ES5nb29nbGUudHlwZS5EYXRlSANSDGRlbGl2ZXJ5RGF0ZYgBARJDChFyZXNwb25zZV9kZWFkbG'
    'luZRgIIAEoCzIRLmdvb2dsZS50eXBlLkRhdGVIBFIQcmVzcG9uc2VEZWFkbGluZYgBARIdCgpi'
    'dXllcl9ub3RlGAkgASgJUglidXllck5vdGUSagoVYnV5ZXJfcmVxdWVzdGVkX3JhbmdlGAogAS'
    'gLMjYuaGF3a3IucHJvY3VyZW1lbnQudjFhbHBoYTEuU2VsbGVyRmFjaW5nUmVxdWVzdGVkUmFu'
    'Z2VSE2J1eWVyUmVxdWVzdGVkUmFuZ2USLwoTcHVibGljYXRpb25fdmVyc2lvbhgLIAEoCVIScH'
    'VibGljYXRpb25WZXJzaW9uQgsKCV9xdWFudGl0eUIHCgVfdW5pdEIMCgpfcGFja2FnaW5nQhAK'
    'Dl9kZWxpdmVyeV9kYXRlQhQKEl9yZXNwb25zZV9kZWFkbGluZQ==');

@$core.Deprecated('Use previewRfqPublicationRequestDescriptor instead')
const PreviewRfqPublicationRequest$json = {
  '1': 'PreviewRfqPublicationRequest',
  '2': [
    {'1': 'request_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'requestId'},
    {
      '1': 'client_request_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'clientRequestId'
    },
    {
      '1': 'expected_revision_id',
      '3': 3,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'expectedRevisionId'
    },
    {
      '1': 'buyer_requested_range',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.BuyerRequestedRange',
      '8': {},
      '10': 'buyerRequestedRange'
    },
    {'1': 'buyer_note', '3': 5, '4': 1, '5': 9, '10': 'buyerNote'},
    {
      '1': 'audience_mode',
      '3': 6,
      '4': 1,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.AudienceMode',
      '8': {},
      '10': 'audienceMode'
    },
    {'1': 'candidate_set_id', '3': 7, '4': 1, '5': 9, '10': 'candidateSetId'},
    {
      '1': 'selected_candidate_ids',
      '3': 8,
      '4': 3,
      '5': 9,
      '10': 'selectedCandidateIds'
    },
    {
      '1': 'response_deadline',
      '3': 9,
      '4': 1,
      '5': 11,
      '6': '.google.type.Date',
      '9': 0,
      '10': 'responseDeadline',
      '17': true
    },
  ],
  '8': [
    {'1': '_response_deadline'},
  ],
};

/// Descriptor for `PreviewRfqPublicationRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List previewRfqPublicationRequestDescriptor = $convert.base64Decode(
    'ChxQcmV2aWV3UmZxUHVibGljYXRpb25SZXF1ZXN0EiIKCnJlcXVlc3RfaWQYASABKAlCA+BBAl'
    'IJcmVxdWVzdElkEjcKEWNsaWVudF9yZXF1ZXN0X2lkGAIgASgJQgvgQQLijM/XCAIIAVIPY2xp'
    'ZW50UmVxdWVzdElkEjUKFGV4cGVjdGVkX3JldmlzaW9uX2lkGAMgASgJQgPgQQJSEmV4cGVjdG'
    'VkUmV2aXNpb25JZBJoChVidXllcl9yZXF1ZXN0ZWRfcmFuZ2UYBCABKAsyLy5oYXdrci5wcm9j'
    'dXJlbWVudC52MWFscGhhMS5CdXllclJlcXVlc3RlZFJhbmdlQgPgQQJSE2J1eWVyUmVxdWVzdG'
    'VkUmFuZ2USHQoKYnV5ZXJfbm90ZRgFIAEoCVIJYnV5ZXJOb3RlElIKDWF1ZGllbmNlX21vZGUY'
    'BiABKA4yKC5oYXdrci5wcm9jdXJlbWVudC52MWFscGhhMS5BdWRpZW5jZU1vZGVCA+BBAlIMYX'
    'VkaWVuY2VNb2RlEigKEGNhbmRpZGF0ZV9zZXRfaWQYByABKAlSDmNhbmRpZGF0ZVNldElkEjQK'
    'FnNlbGVjdGVkX2NhbmRpZGF0ZV9pZHMYCCADKAlSFHNlbGVjdGVkQ2FuZGlkYXRlSWRzEkMKEX'
    'Jlc3BvbnNlX2RlYWRsaW5lGAkgASgLMhEuZ29vZ2xlLnR5cGUuRGF0ZUgAUhByZXNwb25zZURl'
    'YWRsaW5liAEBQhQKEl9yZXNwb25zZV9kZWFkbGluZQ==');

@$core.Deprecated('Use previewRfqPublicationResponseDescriptor instead')
const PreviewRfqPublicationResponse$json = {
  '1': 'PreviewRfqPublicationResponse',
  '2': [
    {'1': 'approval_id', '3': 1, '4': 1, '5': 9, '10': 'approvalId'},
    {'1': 'approval_version', '3': 2, '4': 1, '5': 9, '10': 'approvalVersion'},
    {'1': 'audience_hash', '3': 3, '4': 1, '5': 9, '10': 'audienceHash'},
    {
      '1': 'seller_facing_payload_hash',
      '3': 4,
      '4': 1,
      '5': 9,
      '10': 'sellerFacingPayloadHash'
    },
    {
      '1': 'seller_facing_payload',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.SellerFacingRfq',
      '10': 'sellerFacingPayload'
    },
    {
      '1': 'expire_time',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'expireTime'
    },
    {
      '1': 'audience_policy_id',
      '3': 7,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'audiencePolicyId',
      '17': true
    },
    {
      '1': 'audience_policy_version',
      '3': 8,
      '4': 1,
      '5': 5,
      '9': 1,
      '10': 'audiencePolicyVersion',
      '17': true
    },
    {'1': 'recipient_count', '3': 9, '4': 1, '5': 5, '10': 'recipientCount'},
  ],
  '8': [
    {'1': '_audience_policy_id'},
    {'1': '_audience_policy_version'},
  ],
};

/// Descriptor for `PreviewRfqPublicationResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List previewRfqPublicationResponseDescriptor = $convert.base64Decode(
    'Ch1QcmV2aWV3UmZxUHVibGljYXRpb25SZXNwb25zZRIfCgthcHByb3ZhbF9pZBgBIAEoCVIKYX'
    'Bwcm92YWxJZBIpChBhcHByb3ZhbF92ZXJzaW9uGAIgASgJUg9hcHByb3ZhbFZlcnNpb24SIwoN'
    'YXVkaWVuY2VfaGFzaBgDIAEoCVIMYXVkaWVuY2VIYXNoEjsKGnNlbGxlcl9mYWNpbmdfcGF5bG'
    '9hZF9oYXNoGAQgASgJUhdzZWxsZXJGYWNpbmdQYXlsb2FkSGFzaBJfChVzZWxsZXJfZmFjaW5n'
    'X3BheWxvYWQYBSABKAsyKy5oYXdrci5wcm9jdXJlbWVudC52MWFscGhhMS5TZWxsZXJGYWNpbm'
    'dSZnFSE3NlbGxlckZhY2luZ1BheWxvYWQSOwoLZXhwaXJlX3RpbWUYBiABKAsyGi5nb29nbGUu'
    'cHJvdG9idWYuVGltZXN0YW1wUgpleHBpcmVUaW1lEjEKEmF1ZGllbmNlX3BvbGljeV9pZBgHIA'
    'EoCUgAUhBhdWRpZW5jZVBvbGljeUlkiAEBEjsKF2F1ZGllbmNlX3BvbGljeV92ZXJzaW9uGAgg'
    'ASgFSAFSFWF1ZGllbmNlUG9saWN5VmVyc2lvbogBARInCg9yZWNpcGllbnRfY291bnQYCSABKA'
    'VSDnJlY2lwaWVudENvdW50QhUKE19hdWRpZW5jZV9wb2xpY3lfaWRCGgoYX2F1ZGllbmNlX3Bv'
    'bGljeV92ZXJzaW9u');

@$core.Deprecated('Use executeRfqPublicationRequestDescriptor instead')
const ExecuteRfqPublicationRequest$json = {
  '1': 'ExecuteRfqPublicationRequest',
  '2': [
    {'1': 'request_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'requestId'},
    {
      '1': 'client_request_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'clientRequestId'
    },
    {'1': 'approval_id', '3': 3, '4': 1, '5': 9, '8': {}, '10': 'approvalId'},
    {
      '1': 'approval_version',
      '3': 4,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'approvalVersion'
    },
    {
      '1': 'expected_revision_id',
      '3': 5,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'expectedRevisionId'
    },
    {'1': 'payload_hash', '3': 6, '4': 1, '5': 9, '8': {}, '10': 'payloadHash'},
  ],
};

/// Descriptor for `ExecuteRfqPublicationRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List executeRfqPublicationRequestDescriptor = $convert.base64Decode(
    'ChxFeGVjdXRlUmZxUHVibGljYXRpb25SZXF1ZXN0EiIKCnJlcXVlc3RfaWQYASABKAlCA+BBAl'
    'IJcmVxdWVzdElkEjcKEWNsaWVudF9yZXF1ZXN0X2lkGAIgASgJQgvgQQLijM/XCAIIAVIPY2xp'
    'ZW50UmVxdWVzdElkEiQKC2FwcHJvdmFsX2lkGAMgASgJQgPgQQJSCmFwcHJvdmFsSWQSLgoQYX'
    'Bwcm92YWxfdmVyc2lvbhgEIAEoCUID4EECUg9hcHByb3ZhbFZlcnNpb24SNQoUZXhwZWN0ZWRf'
    'cmV2aXNpb25faWQYBSABKAlCA+BBAlISZXhwZWN0ZWRSZXZpc2lvbklkEiYKDHBheWxvYWRfaG'
    'FzaBgGIAEoCUID4EECUgtwYXlsb2FkSGFzaA==');

@$core.Deprecated('Use executeRfqPublicationResponseDescriptor instead')
const ExecuteRfqPublicationResponse$json = {
  '1': 'ExecuteRfqPublicationResponse',
  '2': [
    {'1': 'publication_id', '3': 1, '4': 1, '5': 9, '10': 'publicationId'},
    {
      '1': 'publication_version',
      '3': 2,
      '4': 1,
      '5': 9,
      '10': 'publicationVersion'
    },
    {
      '1': 'state',
      '3': 3,
      '4': 1,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.PublicationState',
      '10': 'state'
    },
    {
      '1': 'audience_mode',
      '3': 4,
      '4': 1,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.AudienceMode',
      '10': 'audienceMode'
    },
    {'1': 'audience_hash', '3': 5, '4': 1, '5': 9, '10': 'audienceHash'},
    {
      '1': 'seller_facing_payload_hash',
      '3': 6,
      '4': 1,
      '5': 9,
      '10': 'sellerFacingPayloadHash'
    },
    {
      '1': 'publish_time',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'publishTime'
    },
    {'1': 'replayed', '3': 8, '4': 1, '5': 8, '10': 'replayed'},
  ],
};

/// Descriptor for `ExecuteRfqPublicationResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List executeRfqPublicationResponseDescriptor = $convert.base64Decode(
    'Ch1FeGVjdXRlUmZxUHVibGljYXRpb25SZXNwb25zZRIlCg5wdWJsaWNhdGlvbl9pZBgBIAEoCV'
    'INcHVibGljYXRpb25JZBIvChNwdWJsaWNhdGlvbl92ZXJzaW9uGAIgASgJUhJwdWJsaWNhdGlv'
    'blZlcnNpb24SQgoFc3RhdGUYAyABKA4yLC5oYXdrci5wcm9jdXJlbWVudC52MWFscGhhMS5QdW'
    'JsaWNhdGlvblN0YXRlUgVzdGF0ZRJNCg1hdWRpZW5jZV9tb2RlGAQgASgOMiguaGF3a3IucHJv'
    'Y3VyZW1lbnQudjFhbHBoYTEuQXVkaWVuY2VNb2RlUgxhdWRpZW5jZU1vZGUSIwoNYXVkaWVuY2'
    'VfaGFzaBgFIAEoCVIMYXVkaWVuY2VIYXNoEjsKGnNlbGxlcl9mYWNpbmdfcGF5bG9hZF9oYXNo'
    'GAYgASgJUhdzZWxsZXJGYWNpbmdQYXlsb2FkSGFzaBI9CgxwdWJsaXNoX3RpbWUYByABKAsyGi'
    '5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wUgtwdWJsaXNoVGltZRIaCghyZXBsYXllZBgIIAEo'
    'CFIIcmVwbGF5ZWQ=');

@$core.Deprecated('Use listConversationsRequestDescriptor instead')
const ListConversationsRequest$json = {
  '1': 'ListConversationsRequest',
  '2': [
    {'1': 'page_size', '3': 1, '4': 1, '5': 5, '10': 'pageSize'},
    {'1': 'page_token', '3': 2, '4': 1, '5': 9, '10': 'pageToken'},
  ],
};

/// Descriptor for `ListConversationsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listConversationsRequestDescriptor =
    $convert.base64Decode(
        'ChhMaXN0Q29udmVyc2F0aW9uc1JlcXVlc3QSGwoJcGFnZV9zaXplGAEgASgFUghwYWdlU2l6ZR'
        'IdCgpwYWdlX3Rva2VuGAIgASgJUglwYWdlVG9rZW4=');

@$core.Deprecated('Use listConversationsResponseDescriptor instead')
const ListConversationsResponse$json = {
  '1': 'ListConversationsResponse',
  '2': [
    {
      '1': 'conversations',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.ConversationSummary',
      '10': 'conversations'
    },
    {'1': 'next_page_token', '3': 2, '4': 1, '5': 9, '10': 'nextPageToken'},
  ],
};

/// Descriptor for `ListConversationsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listConversationsResponseDescriptor = $convert.base64Decode(
    'ChlMaXN0Q29udmVyc2F0aW9uc1Jlc3BvbnNlElUKDWNvbnZlcnNhdGlvbnMYASADKAsyLy5oYX'
    'drci5wcm9jdXJlbWVudC52MWFscGhhMS5Db252ZXJzYXRpb25TdW1tYXJ5Ug1jb252ZXJzYXRp'
    'b25zEiYKD25leHRfcGFnZV90b2tlbhgCIAEoCVINbmV4dFBhZ2VUb2tlbg==');

@$core.Deprecated('Use getConversationMessagesRequestDescriptor instead')
const GetConversationMessagesRequest$json = {
  '1': 'GetConversationMessagesRequest',
  '2': [
    {
      '1': 'conversation_id',
      '3': 1,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'conversationId'
    },
  ],
};

/// Descriptor for `GetConversationMessagesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getConversationMessagesRequestDescriptor =
    $convert.base64Decode(
        'Ch5HZXRDb252ZXJzYXRpb25NZXNzYWdlc1JlcXVlc3QSLAoPY29udmVyc2F0aW9uX2lkGAEgAS'
        'gJQgPgQQJSDmNvbnZlcnNhdGlvbklk');

@$core.Deprecated('Use getConversationMessagesResponseDescriptor instead')
const GetConversationMessagesResponse$json = {
  '1': 'GetConversationMessagesResponse',
  '2': [
    {'1': 'conversation_id', '3': 1, '4': 1, '5': 9, '10': 'conversationId'},
    {'1': 'request_id', '3': 2, '4': 1, '5': 9, '10': 'requestId'},
    {
      '1': 'status',
      '3': 3,
      '4': 1,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.RequestStatus',
      '10': 'status'
    },
    {'1': 'title', '3': 4, '4': 1, '5': 9, '10': 'title'},
    {
      '1': 'messages',
      '3': 5,
      '4': 3,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.ConversationMessage',
      '10': 'messages'
    },
  ],
};

/// Descriptor for `GetConversationMessagesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getConversationMessagesResponseDescriptor = $convert.base64Decode(
    'Ch9HZXRDb252ZXJzYXRpb25NZXNzYWdlc1Jlc3BvbnNlEicKD2NvbnZlcnNhdGlvbl9pZBgBIA'
    'EoCVIOY29udmVyc2F0aW9uSWQSHQoKcmVxdWVzdF9pZBgCIAEoCVIJcmVxdWVzdElkEkEKBnN0'
    'YXR1cxgDIAEoDjIpLmhhd2tyLnByb2N1cmVtZW50LnYxYWxwaGExLlJlcXVlc3RTdGF0dXNSBn'
    'N0YXR1cxIUCgV0aXRsZRgEIAEoCVIFdGl0bGUSSwoIbWVzc2FnZXMYBSADKAsyLy5oYXdrci5w'
    'cm9jdXJlbWVudC52MWFscGhhMS5Db252ZXJzYXRpb25NZXNzYWdlUghtZXNzYWdlcw==');

@$core.Deprecated('Use getRuntimeSnapshotRequestDescriptor instead')
const GetRuntimeSnapshotRequest$json = {
  '1': 'GetRuntimeSnapshotRequest',
  '2': [
    {
      '1': 'conversation_id',
      '3': 1,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'conversationId'
    },
  ],
};

/// Descriptor for `GetRuntimeSnapshotRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getRuntimeSnapshotRequestDescriptor =
    $convert.base64Decode(
        'ChlHZXRSdW50aW1lU25hcHNob3RSZXF1ZXN0EiwKD2NvbnZlcnNhdGlvbl9pZBgBIAEoCUID4E'
        'ECUg5jb252ZXJzYXRpb25JZA==');

@$core.Deprecated('Use getRuntimeSnapshotResponseDescriptor instead')
const GetRuntimeSnapshotResponse$json = {
  '1': 'GetRuntimeSnapshotResponse',
  '2': [
    {'1': 'conversation_id', '3': 1, '4': 1, '5': 9, '10': 'conversationId'},
    {'1': 'request_id', '3': 2, '4': 1, '5': 9, '10': 'requestId'},
    {
      '1': 'status',
      '3': 3,
      '4': 1,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.RequestStatus',
      '10': 'status'
    },
    {'1': 'title', '3': 4, '4': 1, '5': 9, '10': 'title'},
    {
      '1': 'messages',
      '3': 5,
      '4': 3,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.ConversationMessage',
      '10': 'messages'
    },
    {
      '1': 'runs',
      '3': 6,
      '4': 3,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.RunSummary',
      '10': 'runs'
    },
    {'1': 'active_run_id', '3': 7, '4': 1, '5': 9, '10': 'activeRunId'},
    {
      '1': 'open_confirmation',
      '3': 8,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.OpenConfirmation',
      '9': 0,
      '10': 'openConfirmation',
      '17': true
    },
    {
      '1': 'published_rfq',
      '3': 9,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.PublishedRfq',
      '9': 1,
      '10': 'publishedRfq',
      '17': true
    },
  ],
  '8': [
    {'1': '_open_confirmation'},
    {'1': '_published_rfq'},
  ],
};

/// Descriptor for `GetRuntimeSnapshotResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getRuntimeSnapshotResponseDescriptor = $convert.base64Decode(
    'ChpHZXRSdW50aW1lU25hcHNob3RSZXNwb25zZRInCg9jb252ZXJzYXRpb25faWQYASABKAlSDm'
    'NvbnZlcnNhdGlvbklkEh0KCnJlcXVlc3RfaWQYAiABKAlSCXJlcXVlc3RJZBJBCgZzdGF0dXMY'
    'AyABKA4yKS5oYXdrci5wcm9jdXJlbWVudC52MWFscGhhMS5SZXF1ZXN0U3RhdHVzUgZzdGF0dX'
    'MSFAoFdGl0bGUYBCABKAlSBXRpdGxlEksKCG1lc3NhZ2VzGAUgAygLMi8uaGF3a3IucHJvY3Vy'
    'ZW1lbnQudjFhbHBoYTEuQ29udmVyc2F0aW9uTWVzc2FnZVIIbWVzc2FnZXMSOgoEcnVucxgGIA'
    'MoCzImLmhhd2tyLnByb2N1cmVtZW50LnYxYWxwaGExLlJ1blN1bW1hcnlSBHJ1bnMSIgoNYWN0'
    'aXZlX3J1bl9pZBgHIAEoCVILYWN0aXZlUnVuSWQSXgoRb3Blbl9jb25maXJtYXRpb24YCCABKA'
    'syLC5oYXdrci5wcm9jdXJlbWVudC52MWFscGhhMS5PcGVuQ29uZmlybWF0aW9uSABSEG9wZW5D'
    'b25maXJtYXRpb26IAQESUgoNcHVibGlzaGVkX3JmcRgJIAEoCzIoLmhhd2tyLnByb2N1cmVtZW'
    '50LnYxYWxwaGExLlB1Ymxpc2hlZFJmcUgBUgxwdWJsaXNoZWRSZnGIAQFCFAoSX29wZW5fY29u'
    'ZmlybWF0aW9uQhAKDl9wdWJsaXNoZWRfcmZx');

@$core.Deprecated('Use renameConversationRequestDescriptor instead')
const RenameConversationRequest$json = {
  '1': 'RenameConversationRequest',
  '2': [
    {
      '1': 'conversation_id',
      '3': 1,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'conversationId'
    },
    {'1': 'title', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'title'},
    {
      '1': 'client_request_id',
      '3': 3,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'clientRequestId'
    },
  ],
};

/// Descriptor for `RenameConversationRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List renameConversationRequestDescriptor = $convert.base64Decode(
    'ChlSZW5hbWVDb252ZXJzYXRpb25SZXF1ZXN0EiwKD2NvbnZlcnNhdGlvbl9pZBgBIAEoCUID4E'
    'ECUg5jb252ZXJzYXRpb25JZBIZCgV0aXRsZRgCIAEoCUID4EECUgV0aXRsZRI3ChFjbGllbnRf'
    'cmVxdWVzdF9pZBgDIAEoCUIL4EEC4ozP1wgCCAFSD2NsaWVudFJlcXVlc3RJZA==');

@$core.Deprecated('Use renameConversationResponseDescriptor instead')
const RenameConversationResponse$json = {
  '1': 'RenameConversationResponse',
};

/// Descriptor for `RenameConversationResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List renameConversationResponseDescriptor =
    $convert.base64Decode('ChpSZW5hbWVDb252ZXJzYXRpb25SZXNwb25zZQ==');

@$core.Deprecated('Use deleteConversationRequestDescriptor instead')
const DeleteConversationRequest$json = {
  '1': 'DeleteConversationRequest',
  '2': [
    {
      '1': 'conversation_id',
      '3': 1,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'conversationId'
    },
    {
      '1': 'client_request_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'clientRequestId'
    },
  ],
};

/// Descriptor for `DeleteConversationRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteConversationRequestDescriptor = $convert.base64Decode(
    'ChlEZWxldGVDb252ZXJzYXRpb25SZXF1ZXN0EiwKD2NvbnZlcnNhdGlvbl9pZBgBIAEoCUID4E'
    'ECUg5jb252ZXJzYXRpb25JZBI3ChFjbGllbnRfcmVxdWVzdF9pZBgCIAEoCUIL4EEC4ozP1wgC'
    'CAFSD2NsaWVudFJlcXVlc3RJZA==');

@$core.Deprecated('Use deleteConversationResponseDescriptor instead')
const DeleteConversationResponse$json = {
  '1': 'DeleteConversationResponse',
};

/// Descriptor for `DeleteConversationResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteConversationResponseDescriptor =
    $convert.base64Decode('ChpEZWxldGVDb252ZXJzYXRpb25SZXNwb25zZQ==');

@$core.Deprecated('Use sellerQuoteDescriptor instead')
const SellerQuote$json = {
  '1': 'SellerQuote',
  '2': [
    {
      '1': 'response_type',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.SellerResponseType',
      '8': {},
      '10': 'responseType'
    },
    {
      '1': 'offered_quality',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.OfferedQuality',
      '8': {},
      '10': 'offeredQuality'
    },
    {
      '1': 'unit_basis',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.UnitBasis',
      '10': 'unitBasis'
    },
    {'1': 'offered_quantity', '3': 4, '4': 1, '5': 9, '10': 'offeredQuantity'},
    {
      '1': 'offered_quantity_unit',
      '3': 5,
      '4': 1,
      '5': 9,
      '10': 'offeredQuantityUnit'
    },
    {'1': 'conversion_basis', '3': 6, '4': 1, '5': 9, '10': 'conversionBasis'},
    {'1': 'goods_amount', '3': 7, '4': 1, '5': 3, '10': 'goodsAmount'},
    {'1': 'shipping_amount', '3': 8, '4': 1, '5': 3, '10': 'shippingAmount'},
    {'1': 'tax_amount', '3': 9, '4': 1, '5': 3, '10': 'taxAmount'},
    {
      '1': 'tax_inclusion_basis',
      '3': 10,
      '4': 1,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.TaxInclusionBasis',
      '10': 'taxInclusionBasis'
    },
    {'1': 'total_amount', '3': 11, '4': 1, '5': 3, '10': 'totalAmount'},
    {'1': 'lead_time_days', '3': 12, '4': 1, '5': 5, '10': 'leadTimeDays'},
    {
      '1': 'promised_delivery_time',
      '3': 13,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'promisedDeliveryTime'
    },
    {
      '1': 'valid_until_time',
      '3': 14,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'validUntilTime'
    },
    {'1': 'terms_note', '3': 15, '4': 1, '5': 9, '10': 'termsNote'},
    {
      '1': 'deviation_disclosure',
      '3': 16,
      '4': 1,
      '5': 9,
      '10': 'deviationDisclosure'
    },
  ],
};

/// Descriptor for `SellerQuote`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sellerQuoteDescriptor = $convert.base64Decode(
    'CgtTZWxsZXJRdW90ZRJYCg1yZXNwb25zZV90eXBlGAEgASgOMi4uaGF3a3IucHJvY3VyZW1lbn'
    'QudjFhbHBoYTEuU2VsbGVyUmVzcG9uc2VUeXBlQgPgQQJSDHJlc3BvbnNlVHlwZRJYCg9vZmZl'
    'cmVkX3F1YWxpdHkYAiABKAsyKi5oYXdrci5wcm9jdXJlbWVudC52MWFscGhhMS5PZmZlcmVkUX'
    'VhbGl0eUID4EECUg5vZmZlcmVkUXVhbGl0eRJECgp1bml0X2Jhc2lzGAMgASgLMiUuaGF3a3Iu'
    'cHJvY3VyZW1lbnQudjFhbHBoYTEuVW5pdEJhc2lzUgl1bml0QmFzaXMSKQoQb2ZmZXJlZF9xdW'
    'FudGl0eRgEIAEoCVIPb2ZmZXJlZFF1YW50aXR5EjIKFW9mZmVyZWRfcXVhbnRpdHlfdW5pdBgF'
    'IAEoCVITb2ZmZXJlZFF1YW50aXR5VW5pdBIpChBjb252ZXJzaW9uX2Jhc2lzGAYgASgJUg9jb2'
    '52ZXJzaW9uQmFzaXMSIQoMZ29vZHNfYW1vdW50GAcgASgDUgtnb29kc0Ftb3VudBInCg9zaGlw'
    'cGluZ19hbW91bnQYCCABKANSDnNoaXBwaW5nQW1vdW50Eh0KCnRheF9hbW91bnQYCSABKANSCX'
    'RheEFtb3VudBJdChN0YXhfaW5jbHVzaW9uX2Jhc2lzGAogASgOMi0uaGF3a3IucHJvY3VyZW1l'
    'bnQudjFhbHBoYTEuVGF4SW5jbHVzaW9uQmFzaXNSEXRheEluY2x1c2lvbkJhc2lzEiEKDHRvdG'
    'FsX2Ftb3VudBgLIAEoA1ILdG90YWxBbW91bnQSJAoObGVhZF90aW1lX2RheXMYDCABKAVSDGxl'
    'YWRUaW1lRGF5cxJQChZwcm9taXNlZF9kZWxpdmVyeV90aW1lGA0gASgLMhouZ29vZ2xlLnByb3'
    'RvYnVmLlRpbWVzdGFtcFIUcHJvbWlzZWREZWxpdmVyeVRpbWUSRAoQdmFsaWRfdW50aWxfdGlt'
    'ZRgOIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBSDnZhbGlkVW50aWxUaW1lEh0KCn'
    'Rlcm1zX25vdGUYDyABKAlSCXRlcm1zTm90ZRIxChRkZXZpYXRpb25fZGlzY2xvc3VyZRgQIAEo'
    'CVITZGV2aWF0aW9uRGlzY2xvc3VyZQ==');

@$core.Deprecated('Use offeredQualityDescriptor instead')
const OfferedQuality$json = {
  '1': 'OfferedQuality',
  '2': [
    {'1': 'grade', '3': 1, '4': 1, '5': 9, '10': 'grade'},
    {'1': 'inspection_notes', '3': 2, '4': 1, '5': 9, '10': 'inspectionNotes'},
  ],
};

/// Descriptor for `OfferedQuality`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List offeredQualityDescriptor = $convert.base64Decode(
    'Cg5PZmZlcmVkUXVhbGl0eRIUCgVncmFkZRgBIAEoCVIFZ3JhZGUSKQoQaW5zcGVjdGlvbl9ub3'
    'RlcxgCIAEoCVIPaW5zcGVjdGlvbk5vdGVz');

@$core.Deprecated('Use unitBasisDescriptor instead')
const UnitBasis$json = {
  '1': 'UnitBasis',
  '2': [
    {'1': 'quantity', '3': 1, '4': 1, '5': 9, '10': 'quantity'},
    {'1': 'unit', '3': 2, '4': 1, '5': 9, '10': 'unit'},
  ],
};

/// Descriptor for `UnitBasis`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List unitBasisDescriptor = $convert.base64Decode(
    'CglVbml0QmFzaXMSGgoIcXVhbnRpdHkYASABKAlSCHF1YW50aXR5EhIKBHVuaXQYAiABKAlSBH'
    'VuaXQ=');

@$core.Deprecated('Use buyerFacingSellerResponseDescriptor instead')
const BuyerFacingSellerResponse$json = {
  '1': 'BuyerFacingSellerResponse',
  '2': [
    {
      '1': 'seller_response_id',
      '3': 1,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'sellerResponseId'
    },
    {
      '1': 'seller_response_version',
      '3': 2,
      '4': 1,
      '5': 9,
      '10': 'sellerResponseVersion'
    },
    {
      '1': 'status',
      '3': 3,
      '4': 1,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.SellerResponseStatus',
      '10': 'status'
    },
    {'1': 'payload_hash', '3': 4, '4': 1, '5': 9, '10': 'payloadHash'},
    {
      '1': 'submit_time',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'submitTime'
    },
    {
      '1': 'store',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.StoreCard',
      '10': 'store'
    },
    {
      '1': 'quote',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.SellerQuote',
      '10': 'quote'
    },
    {
      '1': 'outside_buyer_requested_range',
      '3': 8,
      '4': 1,
      '5': 8,
      '10': 'outsideBuyerRequestedRange'
    },
    {'1': 'shortlisted', '3': 9, '4': 1, '5': 8, '10': 'shortlisted'},
    {
      '1': 'acceptability',
      '3': 10,
      '4': 1,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.QuoteAcceptability',
      '10': 'acceptability'
    },
  ],
};

/// Descriptor for `BuyerFacingSellerResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List buyerFacingSellerResponseDescriptor = $convert.base64Decode(
    'ChlCdXllckZhY2luZ1NlbGxlclJlc3BvbnNlEjEKEnNlbGxlcl9yZXNwb25zZV9pZBgBIAEoCU'
    'ID4EEIUhBzZWxsZXJSZXNwb25zZUlkEjYKF3NlbGxlcl9yZXNwb25zZV92ZXJzaW9uGAIgASgJ'
    'UhVzZWxsZXJSZXNwb25zZVZlcnNpb24SSAoGc3RhdHVzGAMgASgOMjAuaGF3a3IucHJvY3VyZW'
    '1lbnQudjFhbHBoYTEuU2VsbGVyUmVzcG9uc2VTdGF0dXNSBnN0YXR1cxIhCgxwYXlsb2FkX2hh'
    'c2gYBCABKAlSC3BheWxvYWRIYXNoEjsKC3N1Ym1pdF90aW1lGAUgASgLMhouZ29vZ2xlLnByb3'
    'RvYnVmLlRpbWVzdGFtcFIKc3VibWl0VGltZRI2CgVzdG9yZRgGIAEoCzIgLmhhd2tyLmNvbW1v'
    'bi52MWFscGhhMS5TdG9yZUNhcmRSBXN0b3JlEj0KBXF1b3RlGAcgASgLMicuaGF3a3IucHJvY3'
    'VyZW1lbnQudjFhbHBoYTEuU2VsbGVyUXVvdGVSBXF1b3RlEkEKHW91dHNpZGVfYnV5ZXJfcmVx'
    'dWVzdGVkX3JhbmdlGAggASgIUhpvdXRzaWRlQnV5ZXJSZXF1ZXN0ZWRSYW5nZRIgCgtzaG9ydG'
    'xpc3RlZBgJIAEoCFILc2hvcnRsaXN0ZWQSVAoNYWNjZXB0YWJpbGl0eRgKIAEoDjIuLmhhd2ty'
    'LnByb2N1cmVtZW50LnYxYWxwaGExLlF1b3RlQWNjZXB0YWJpbGl0eVINYWNjZXB0YWJpbGl0eQ'
    '==');

@$core.Deprecated('Use listSellerResponsesRequestDescriptor instead')
const ListSellerResponsesRequest$json = {
  '1': 'ListSellerResponsesRequest',
  '2': [
    {'1': 'request_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'requestId'},
    {
      '1': 'publication_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'publicationId'
    },
    {'1': 'page_size', '3': 3, '4': 1, '5': 5, '10': 'pageSize'},
    {'1': 'page_token', '3': 4, '4': 1, '5': 9, '10': 'pageToken'},
  ],
};

/// Descriptor for `ListSellerResponsesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listSellerResponsesRequestDescriptor = $convert.base64Decode(
    'ChpMaXN0U2VsbGVyUmVzcG9uc2VzUmVxdWVzdBIiCgpyZXF1ZXN0X2lkGAEgASgJQgPgQQJSCX'
    'JlcXVlc3RJZBIqCg5wdWJsaWNhdGlvbl9pZBgCIAEoCUID4EECUg1wdWJsaWNhdGlvbklkEhsK'
    'CXBhZ2Vfc2l6ZRgDIAEoBVIIcGFnZVNpemUSHQoKcGFnZV90b2tlbhgEIAEoCVIJcGFnZVRva2'
    'Vu');

@$core.Deprecated('Use listSellerResponsesResponseDescriptor instead')
const ListSellerResponsesResponse$json = {
  '1': 'ListSellerResponsesResponse',
  '2': [
    {
      '1': 'seller_responses',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.BuyerFacingSellerResponse',
      '10': 'sellerResponses'
    },
    {'1': 'next_page_token', '3': 2, '4': 1, '5': 9, '10': 'nextPageToken'},
  ],
};

/// Descriptor for `ListSellerResponsesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listSellerResponsesResponseDescriptor = $convert.base64Decode(
    'ChtMaXN0U2VsbGVyUmVzcG9uc2VzUmVzcG9uc2USYAoQc2VsbGVyX3Jlc3BvbnNlcxgBIAMoCz'
    'I1Lmhhd2tyLnByb2N1cmVtZW50LnYxYWxwaGExLkJ1eWVyRmFjaW5nU2VsbGVyUmVzcG9uc2VS'
    'D3NlbGxlclJlc3BvbnNlcxImCg9uZXh0X3BhZ2VfdG9rZW4YAiABKAlSDW5leHRQYWdlVG9rZW'
    '4=');

@$core.Deprecated('Use putShortlistEntryRequestDescriptor instead')
const PutShortlistEntryRequest$json = {
  '1': 'PutShortlistEntryRequest',
  '2': [
    {'1': 'request_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'requestId'},
    {
      '1': 'seller_response_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'sellerResponseId'
    },
    {
      '1': 'seller_response_version',
      '3': 3,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'sellerResponseVersion'
    },
  ],
};

/// Descriptor for `PutShortlistEntryRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List putShortlistEntryRequestDescriptor = $convert.base64Decode(
    'ChhQdXRTaG9ydGxpc3RFbnRyeVJlcXVlc3QSIgoKcmVxdWVzdF9pZBgBIAEoCUID4EECUglyZX'
    'F1ZXN0SWQSMQoSc2VsbGVyX3Jlc3BvbnNlX2lkGAIgASgJQgPgQQJSEHNlbGxlclJlc3BvbnNl'
    'SWQSOwoXc2VsbGVyX3Jlc3BvbnNlX3ZlcnNpb24YAyABKAlCA+BBAlIVc2VsbGVyUmVzcG9uc2'
    'VWZXJzaW9u');

@$core.Deprecated('Use putShortlistEntryResponseDescriptor instead')
const PutShortlistEntryResponse$json = {
  '1': 'PutShortlistEntryResponse',
  '2': [
    {
      '1': 'seller_response_id',
      '3': 1,
      '4': 1,
      '5': 9,
      '10': 'sellerResponseId'
    },
    {
      '1': 'seller_response_version',
      '3': 2,
      '4': 1,
      '5': 9,
      '10': 'sellerResponseVersion'
    },
    {'1': 'present', '3': 3, '4': 1, '5': 8, '10': 'present'},
  ],
};

/// Descriptor for `PutShortlistEntryResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List putShortlistEntryResponseDescriptor = $convert.base64Decode(
    'ChlQdXRTaG9ydGxpc3RFbnRyeVJlc3BvbnNlEiwKEnNlbGxlcl9yZXNwb25zZV9pZBgBIAEoCV'
    'IQc2VsbGVyUmVzcG9uc2VJZBI2ChdzZWxsZXJfcmVzcG9uc2VfdmVyc2lvbhgCIAEoCVIVc2Vs'
    'bGVyUmVzcG9uc2VWZXJzaW9uEhgKB3ByZXNlbnQYAyABKAhSB3ByZXNlbnQ=');

@$core.Deprecated('Use deleteShortlistEntryRequestDescriptor instead')
const DeleteShortlistEntryRequest$json = {
  '1': 'DeleteShortlistEntryRequest',
  '2': [
    {'1': 'request_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'requestId'},
    {
      '1': 'seller_response_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'sellerResponseId'
    },
    {
      '1': 'seller_response_version',
      '3': 3,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'sellerResponseVersion'
    },
  ],
};

/// Descriptor for `DeleteShortlistEntryRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteShortlistEntryRequestDescriptor = $convert.base64Decode(
    'ChtEZWxldGVTaG9ydGxpc3RFbnRyeVJlcXVlc3QSIgoKcmVxdWVzdF9pZBgBIAEoCUID4EECUg'
    'lyZXF1ZXN0SWQSMQoSc2VsbGVyX3Jlc3BvbnNlX2lkGAIgASgJQgPgQQJSEHNlbGxlclJlc3Bv'
    'bnNlSWQSOwoXc2VsbGVyX3Jlc3BvbnNlX3ZlcnNpb24YAyABKAlCA+BBAlIVc2VsbGVyUmVzcG'
    '9uc2VWZXJzaW9u');

@$core.Deprecated('Use deleteShortlistEntryResponseDescriptor instead')
const DeleteShortlistEntryResponse$json = {
  '1': 'DeleteShortlistEntryResponse',
  '2': [
    {
      '1': 'seller_response_id',
      '3': 1,
      '4': 1,
      '5': 9,
      '10': 'sellerResponseId'
    },
    {
      '1': 'seller_response_version',
      '3': 2,
      '4': 1,
      '5': 9,
      '10': 'sellerResponseVersion'
    },
    {'1': 'present', '3': 3, '4': 1, '5': 8, '10': 'present'},
  ],
};

/// Descriptor for `DeleteShortlistEntryResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteShortlistEntryResponseDescriptor =
    $convert.base64Decode(
        'ChxEZWxldGVTaG9ydGxpc3RFbnRyeVJlc3BvbnNlEiwKEnNlbGxlcl9yZXNwb25zZV9pZBgBIA'
        'EoCVIQc2VsbGVyUmVzcG9uc2VJZBI2ChdzZWxsZXJfcmVzcG9uc2VfdmVyc2lvbhgCIAEoCVIV'
        'c2VsbGVyUmVzcG9uc2VWZXJzaW9uEhgKB3ByZXNlbnQYAyABKAhSB3ByZXNlbnQ=');

@$core.Deprecated('Use acceptSellerQuoteRequestDescriptor instead')
const AcceptSellerQuoteRequest$json = {
  '1': 'AcceptSellerQuoteRequest',
  '2': [
    {'1': 'request_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'requestId'},
    {
      '1': 'client_request_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'clientRequestId'
    },
    {
      '1': 'seller_response_id',
      '3': 3,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'sellerResponseId'
    },
    {
      '1': 'expected_seller_response_version',
      '3': 4,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'expectedSellerResponseVersion'
    },
    {
      '1': 'expected_seller_facing_hash',
      '3': 5,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'expectedSellerFacingHash'
    },
  ],
};

/// Descriptor for `AcceptSellerQuoteRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List acceptSellerQuoteRequestDescriptor = $convert.base64Decode(
    'ChhBY2NlcHRTZWxsZXJRdW90ZVJlcXVlc3QSIgoKcmVxdWVzdF9pZBgBIAEoCUID4EECUglyZX'
    'F1ZXN0SWQSNwoRY2xpZW50X3JlcXVlc3RfaWQYAiABKAlCC+BBAuKMz9cIAggBUg9jbGllbnRS'
    'ZXF1ZXN0SWQSMQoSc2VsbGVyX3Jlc3BvbnNlX2lkGAMgASgJQgPgQQJSEHNlbGxlclJlc3Bvbn'
    'NlSWQSTAogZXhwZWN0ZWRfc2VsbGVyX3Jlc3BvbnNlX3ZlcnNpb24YBCABKAlCA+BBAlIdZXhw'
    'ZWN0ZWRTZWxsZXJSZXNwb25zZVZlcnNpb24SQgobZXhwZWN0ZWRfc2VsbGVyX2ZhY2luZ19oYX'
    'NoGAUgASgJQgPgQQJSGGV4cGVjdGVkU2VsbGVyRmFjaW5nSGFzaA==');

@$core.Deprecated('Use acceptSellerQuoteResponseDescriptor instead')
const AcceptSellerQuoteResponse$json = {
  '1': 'AcceptSellerQuoteResponse',
  '2': [
    {
      '1': 'buyer_acceptance_id',
      '3': 1,
      '4': 1,
      '5': 9,
      '10': 'buyerAcceptanceId'
    },
    {
      '1': 'seller_response_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '10': 'sellerResponseId'
    },
    {
      '1': 'seller_response_version',
      '3': 3,
      '4': 1,
      '5': 9,
      '10': 'sellerResponseVersion'
    },
    {
      '1': 'accepted_deal_price',
      '3': 4,
      '4': 1,
      '5': 3,
      '10': 'acceptedDealPrice'
    },
    {
      '1': 'accept_time',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'acceptTime'
    },
    {
      '1': 'order_provisioning_state',
      '3': 6,
      '4': 1,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.OrderProvisioningState',
      '10': 'orderProvisioningState'
    },
    {'1': 'order_no', '3': 7, '4': 1, '5': 9, '10': 'orderNo'},
    {
      '1': 'payment_deadline',
      '3': 8,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 0,
      '10': 'paymentDeadline',
      '17': true
    },
    {'1': 'replayed', '3': 9, '4': 1, '5': 8, '10': 'replayed'},
  ],
  '8': [
    {'1': '_payment_deadline'},
  ],
};

/// Descriptor for `AcceptSellerQuoteResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List acceptSellerQuoteResponseDescriptor = $convert.base64Decode(
    'ChlBY2NlcHRTZWxsZXJRdW90ZVJlc3BvbnNlEi4KE2J1eWVyX2FjY2VwdGFuY2VfaWQYASABKA'
    'lSEWJ1eWVyQWNjZXB0YW5jZUlkEiwKEnNlbGxlcl9yZXNwb25zZV9pZBgCIAEoCVIQc2VsbGVy'
    'UmVzcG9uc2VJZBI2ChdzZWxsZXJfcmVzcG9uc2VfdmVyc2lvbhgDIAEoCVIVc2VsbGVyUmVzcG'
    '9uc2VWZXJzaW9uEi4KE2FjY2VwdGVkX2RlYWxfcHJpY2UYBCABKANSEWFjY2VwdGVkRGVhbFBy'
    'aWNlEjsKC2FjY2VwdF90aW1lGAUgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcFIKYW'
    'NjZXB0VGltZRJsChhvcmRlcl9wcm92aXNpb25pbmdfc3RhdGUYBiABKA4yMi5oYXdrci5wcm9j'
    'dXJlbWVudC52MWFscGhhMS5PcmRlclByb3Zpc2lvbmluZ1N0YXRlUhZvcmRlclByb3Zpc2lvbm'
    'luZ1N0YXRlEhkKCG9yZGVyX25vGAcgASgJUgdvcmRlck5vEkoKEHBheW1lbnRfZGVhZGxpbmUY'
    'CCABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wSABSD3BheW1lbnREZWFkbGluZYgBAR'
    'IaCghyZXBsYXllZBgJIAEoCFIIcmVwbGF5ZWRCEwoRX3BheW1lbnRfZGVhZGxpbmU=');

@$core.Deprecated('Use watchRunRequestDescriptor instead')
const WatchRunRequest$json = {
  '1': 'WatchRunRequest',
  '2': [
    {'1': 'run_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'runId'},
    {'1': 'cursor', '3': 2, '4': 1, '5': 9, '10': 'cursor'},
  ],
};

/// Descriptor for `WatchRunRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List watchRunRequestDescriptor = $convert.base64Decode(
    'Cg9XYXRjaFJ1blJlcXVlc3QSGgoGcnVuX2lkGAEgASgJQgPgQQJSBXJ1bklkEhYKBmN1cnNvch'
    'gCIAEoCVIGY3Vyc29y');

@$core.Deprecated('Use watchRunResponseDescriptor instead')
const WatchRunResponse$json = {
  '1': 'WatchRunResponse',
  '2': [
    {'1': 'cursor', '3': 1, '4': 1, '5': 9, '10': 'cursor'},
    {
      '1': 'event_time',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'eventTime'
    },
    {
      '1': 'run_accepted',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.RunAccepted',
      '9': 0,
      '10': 'runAccepted'
    },
    {
      '1': 'text',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.TextDelta',
      '9': 0,
      '10': 'text'
    },
    {
      '1': 'step',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.StepEvent',
      '9': 0,
      '10': 'step'
    },
    {
      '1': 'activity',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.Activity',
      '9': 0,
      '10': 'activity'
    },
    {
      '1': 'message',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.MessageEvent',
      '9': 0,
      '10': 'message'
    },
    {
      '1': 'tool_result',
      '3': 8,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.ToolResult',
      '9': 0,
      '10': 'toolResult'
    },
    {
      '1': 'turn_result',
      '3': 9,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.TurnResult',
      '9': 0,
      '10': 'turnResult'
    },
    {
      '1': 'finished',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.RunFinished',
      '9': 0,
      '10': 'finished'
    },
  ],
  '8': [
    {'1': 'event'},
  ],
  '9': [
    {'1': 11, '2': 12},
  ],
  '10': ['error'],
};

/// Descriptor for `WatchRunResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List watchRunResponseDescriptor = $convert.base64Decode(
    'ChBXYXRjaFJ1blJlc3BvbnNlEhYKBmN1cnNvchgBIAEoCVIGY3Vyc29yEjkKCmV2ZW50X3RpbW'
    'UYAiABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wUglldmVudFRpbWUSTAoMcnVuX2Fj'
    'Y2VwdGVkGAMgASgLMicuaGF3a3IucHJvY3VyZW1lbnQudjFhbHBoYTEuUnVuQWNjZXB0ZWRIAF'
    'ILcnVuQWNjZXB0ZWQSOwoEdGV4dBgEIAEoCzIlLmhhd2tyLnByb2N1cmVtZW50LnYxYWxwaGEx'
    'LlRleHREZWx0YUgAUgR0ZXh0EjsKBHN0ZXAYBSABKAsyJS5oYXdrci5wcm9jdXJlbWVudC52MW'
    'FscGhhMS5TdGVwRXZlbnRIAFIEc3RlcBJCCghhY3Rpdml0eRgGIAEoCzIkLmhhd2tyLnByb2N1'
    'cmVtZW50LnYxYWxwaGExLkFjdGl2aXR5SABSCGFjdGl2aXR5EkQKB21lc3NhZ2UYByABKAsyKC'
    '5oYXdrci5wcm9jdXJlbWVudC52MWFscGhhMS5NZXNzYWdlRXZlbnRIAFIHbWVzc2FnZRJJCgt0'
    'b29sX3Jlc3VsdBgIIAEoCzImLmhhd2tyLnByb2N1cmVtZW50LnYxYWxwaGExLlRvb2xSZXN1bH'
    'RIAFIKdG9vbFJlc3VsdBJJCgt0dXJuX3Jlc3VsdBgJIAEoCzImLmhhd2tyLnByb2N1cmVtZW50'
    'LnYxYWxwaGExLlR1cm5SZXN1bHRIAFIKdHVyblJlc3VsdBJFCghmaW5pc2hlZBgKIAEoCzInLm'
    'hhd2tyLnByb2N1cmVtZW50LnYxYWxwaGExLlJ1bkZpbmlzaGVkSABSCGZpbmlzaGVkQgcKBWV2'
    'ZW50SgQICxAMUgVlcnJvcg==');

@$core.Deprecated('Use runAcceptedDescriptor instead')
const RunAccepted$json = {
  '1': 'RunAccepted',
  '2': [
    {'1': 'run_id', '3': 1, '4': 1, '5': 9, '10': 'runId'},
    {'1': 'conversation_id', '3': 2, '4': 1, '5': 9, '10': 'conversationId'},
  ],
  '9': [
    {'1': 3, '2': 4},
  ],
  '10': ['run_version'],
};

/// Descriptor for `RunAccepted`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List runAcceptedDescriptor = $convert.base64Decode(
    'CgtSdW5BY2NlcHRlZBIVCgZydW5faWQYASABKAlSBXJ1bklkEicKD2NvbnZlcnNhdGlvbl9pZB'
    'gCIAEoCVIOY29udmVyc2F0aW9uSWRKBAgDEARSC3J1bl92ZXJzaW9u');

@$core.Deprecated('Use textDeltaDescriptor instead')
const TextDelta$json = {
  '1': 'TextDelta',
  '2': [
    {'1': 'text', '3': 1, '4': 1, '5': 9, '10': 'text'},
  ],
};

/// Descriptor for `TextDelta`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List textDeltaDescriptor =
    $convert.base64Decode('CglUZXh0RGVsdGESEgoEdGV4dBgBIAEoCVIEdGV4dA==');

@$core.Deprecated('Use stepEventDescriptor instead')
const StepEvent$json = {
  '1': 'StepEvent',
  '2': [
    {'1': 'seq', '3': 1, '4': 1, '5': 5, '10': 'seq'},
    {
      '1': 'step',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.Step',
      '10': 'step'
    },
  ],
};

/// Descriptor for `StepEvent`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List stepEventDescriptor = $convert.base64Decode(
    'CglTdGVwRXZlbnQSEAoDc2VxGAEgASgFUgNzZXESNAoEc3RlcBgCIAEoCzIgLmhhd2tyLnByb2'
    'N1cmVtZW50LnYxYWxwaGExLlN0ZXBSBHN0ZXA=');

@$core.Deprecated('Use activityDescriptor instead')
const Activity$json = {
  '1': 'Activity',
  '2': [
    {'1': 'row_id', '3': 1, '4': 1, '5': 9, '10': 'rowId'},
    {'1': 'attempt', '3': 2, '4': 1, '5': 5, '10': 'attempt'},
    {'1': 'max_attempts', '3': 3, '4': 1, '5': 5, '10': 'maxAttempts'},
    {'1': 'retry_group_key', '3': 4, '4': 1, '5': 9, '10': 'retryGroupKey'},
    {'1': 'code', '3': 5, '4': 1, '5': 9, '10': 'code'},
    {'1': 'label', '3': 6, '4': 1, '5': 9, '10': 'label'},
    {'1': 'text', '3': 7, '4': 1, '5': 9, '10': 'text'},
    {
      '1': 'status',
      '3': 8,
      '4': 1,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.StepStatus',
      '10': 'status'
    },
    {
      '1': 'parent_row_id',
      '3': 9,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'parentRowId',
      '17': true
    },
    {'1': 'count', '3': 10, '4': 1, '5': 5, '9': 1, '10': 'count', '17': true},
  ],
  '8': [
    {'1': '_parent_row_id'},
    {'1': '_count'},
  ],
};

/// Descriptor for `Activity`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List activityDescriptor = $convert.base64Decode(
    'CghBY3Rpdml0eRIVCgZyb3dfaWQYASABKAlSBXJvd0lkEhgKB2F0dGVtcHQYAiABKAVSB2F0dG'
    'VtcHQSIQoMbWF4X2F0dGVtcHRzGAMgASgFUgttYXhBdHRlbXB0cxImCg9yZXRyeV9ncm91cF9r'
    'ZXkYBCABKAlSDXJldHJ5R3JvdXBLZXkSEgoEY29kZRgFIAEoCVIEY29kZRIUCgVsYWJlbBgGIA'
    'EoCVIFbGFiZWwSEgoEdGV4dBgHIAEoCVIEdGV4dBI+CgZzdGF0dXMYCCABKA4yJi5oYXdrci5w'
    'cm9jdXJlbWVudC52MWFscGhhMS5TdGVwU3RhdHVzUgZzdGF0dXMSJwoNcGFyZW50X3Jvd19pZB'
    'gJIAEoCUgAUgtwYXJlbnRSb3dJZIgBARIZCgVjb3VudBgKIAEoBUgBUgVjb3VudIgBAUIQCg5f'
    'cGFyZW50X3Jvd19pZEIICgZfY291bnQ=');

@$core.Deprecated('Use messageEventDescriptor instead')
const MessageEvent$json = {
  '1': 'MessageEvent',
  '2': [
    {'1': 'reply', '3': 1, '4': 1, '5': 9, '10': 'reply'},
  ],
};

/// Descriptor for `MessageEvent`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List messageEventDescriptor =
    $convert.base64Decode('CgxNZXNzYWdlRXZlbnQSFAoFcmVwbHkYASABKAlSBXJlcGx5');

@$core.Deprecated('Use toolResultDescriptor instead')
const ToolResult$json = {
  '1': 'ToolResult',
  '2': [
    {'1': 'tool_name', '3': 1, '4': 1, '5': 9, '10': 'toolName'},
    {'1': 'failed', '3': 2, '4': 1, '5': 8, '10': 'failed'},
  ],
};

/// Descriptor for `ToolResult`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List toolResultDescriptor = $convert.base64Decode(
    'CgpUb29sUmVzdWx0EhsKCXRvb2xfbmFtZRgBIAEoCVIIdG9vbE5hbWUSFgoGZmFpbGVkGAIgAS'
    'gIUgZmYWlsZWQ=');

@$core.Deprecated('Use runFinishedDescriptor instead')
const RunFinished$json = {
  '1': 'RunFinished',
  '2': [
    {
      '1': 'status',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.RunStatus',
      '10': 'status'
    },
    {
      '1': 'failure_reason',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.RunFailureReason',
      '10': 'failureReason'
    },
  ],
  '9': [
    {'1': 3, '2': 4},
  ],
  '10': ['localized_message'],
};

/// Descriptor for `RunFinished`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List runFinishedDescriptor = $convert.base64Decode(
    'CgtSdW5GaW5pc2hlZBI9CgZzdGF0dXMYASABKA4yJS5oYXdrci5wcm9jdXJlbWVudC52MWFscG'
    'hhMS5SdW5TdGF0dXNSBnN0YXR1cxJTCg5mYWlsdXJlX3JlYXNvbhgCIAEoDjIsLmhhd2tyLnBy'
    'b2N1cmVtZW50LnYxYWxwaGExLlJ1bkZhaWx1cmVSZWFzb25SDWZhaWx1cmVSZWFzb25KBAgDEA'
    'RSEWxvY2FsaXplZF9tZXNzYWdl');

@$core.Deprecated('Use cancelRunRequestDescriptor instead')
const CancelRunRequest$json = {
  '1': 'CancelRunRequest',
  '2': [
    {'1': 'run_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'runId'},
  ],
  '9': [
    {'1': 2, '2': 3},
  ],
  '10': ['expected_run_version'],
};

/// Descriptor for `CancelRunRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List cancelRunRequestDescriptor = $convert.base64Decode(
    'ChBDYW5jZWxSdW5SZXF1ZXN0EhoKBnJ1bl9pZBgBIAEoCUID4EECUgVydW5JZEoECAIQA1IUZX'
    'hwZWN0ZWRfcnVuX3ZlcnNpb24=');

@$core.Deprecated('Use cancelRunResponseDescriptor instead')
const CancelRunResponse$json = {
  '1': 'CancelRunResponse',
  '2': [
    {'1': 'run_id', '3': 1, '4': 1, '5': 9, '10': 'runId'},
    {
      '1': 'run_status',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.RunStatus',
      '10': 'runStatus'
    },
    {
      '1': 'cancel_disposition',
      '3': 3,
      '4': 1,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.CancelDisposition',
      '10': 'cancelDisposition'
    },
  ],
  '9': [
    {'1': 4, '2': 5},
  ],
  '10': ['run_version'],
};

/// Descriptor for `CancelRunResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List cancelRunResponseDescriptor = $convert.base64Decode(
    'ChFDYW5jZWxSdW5SZXNwb25zZRIVCgZydW5faWQYASABKAlSBXJ1bklkEkQKCnJ1bl9zdGF0dX'
    'MYAiABKA4yJS5oYXdrci5wcm9jdXJlbWVudC52MWFscGhhMS5SdW5TdGF0dXNSCXJ1blN0YXR1'
    'cxJcChJjYW5jZWxfZGlzcG9zaXRpb24YAyABKA4yLS5oYXdrci5wcm9jdXJlbWVudC52MWFscG'
    'hhMS5DYW5jZWxEaXNwb3NpdGlvblIRY2FuY2VsRGlzcG9zaXRpb25KBAgEEAVSC3J1bl92ZXJz'
    'aW9u');

@$core.Deprecated('Use clarificationResponseDescriptor instead')
const ClarificationResponse$json = {
  '1': 'ClarificationResponse',
  '2': [
    {'1': 'option_id', '3': 1, '4': 1, '5': 9, '10': 'optionId'},
    {'1': 'free_text', '3': 2, '4': 1, '5': 9, '10': 'freeText'},
  ],
};

/// Descriptor for `ClarificationResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List clarificationResponseDescriptor = $convert.base64Decode(
    'ChVDbGFyaWZpY2F0aW9uUmVzcG9uc2USGwoJb3B0aW9uX2lkGAEgASgJUghvcHRpb25JZBIbCg'
    'lmcmVlX3RleHQYAiABKAlSCGZyZWVUZXh0');

@$core.Deprecated('Use resumeRunRequestDescriptor instead')
const ResumeRunRequest$json = {
  '1': 'ResumeRunRequest',
  '2': [
    {'1': 'interrupt_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'interruptId'},
    {
      '1': 'client_request_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'clientRequestId'
    },
    {
      '1': 'clarification',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.ClarificationResponse',
      '9': 0,
      '10': 'clarification'
    },
  ],
  '8': [
    {'1': 'response'},
  ],
};

/// Descriptor for `ResumeRunRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List resumeRunRequestDescriptor = $convert.base64Decode(
    'ChBSZXN1bWVSdW5SZXF1ZXN0EiYKDGludGVycnVwdF9pZBgBIAEoCUID4EECUgtpbnRlcnJ1cH'
    'RJZBI3ChFjbGllbnRfcmVxdWVzdF9pZBgCIAEoCUIL4EEC4ozP1wgCCAFSD2NsaWVudFJlcXVl'
    'c3RJZBJZCg1jbGFyaWZpY2F0aW9uGAMgASgLMjEuaGF3a3IucHJvY3VyZW1lbnQudjFhbHBoYT'
    'EuQ2xhcmlmaWNhdGlvblJlc3BvbnNlSABSDWNsYXJpZmljYXRpb25CCgoIcmVzcG9uc2U=');

@$core.Deprecated('Use resumeRunResponseDescriptor instead')
const ResumeRunResponse$json = {
  '1': 'ResumeRunResponse',
  '2': [
    {'1': 'run_id', '3': 1, '4': 1, '5': 9, '10': 'runId'},
    {
      '1': 'run_status',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.RunStatus',
      '10': 'runStatus'
    },
  ],
  '9': [
    {'1': 3, '2': 4},
  ],
  '10': ['result'],
};

/// Descriptor for `ResumeRunResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List resumeRunResponseDescriptor = $convert.base64Decode(
    'ChFSZXN1bWVSdW5SZXNwb25zZRIVCgZydW5faWQYASABKAlSBXJ1bklkEkQKCnJ1bl9zdGF0dX'
    'MYAiABKA4yJS5oYXdrci5wcm9jdXJlbWVudC52MWFscGhhMS5SdW5TdGF0dXNSCXJ1blN0YXR1'
    'c0oECAMQBFIGcmVzdWx0');

@$core.Deprecated('Use sellerPublicationSummaryDescriptor instead')
const SellerPublicationSummary$json = {
  '1': 'SellerPublicationSummary',
  '2': [
    {'1': 'publication_id', '3': 1, '4': 1, '5': 9, '10': 'publicationId'},
    {
      '1': 'publication_version',
      '3': 2,
      '4': 1,
      '5': 9,
      '10': 'publicationVersion'
    },
    {
      '1': 'seller_facing_payload_hash',
      '3': 3,
      '4': 1,
      '5': 9,
      '10': 'sellerFacingPayloadHash'
    },
    {
      '1': 'delivery_state',
      '3': 4,
      '4': 1,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.InviteDeliveryState',
      '10': 'deliveryState'
    },
    {
      '1': 'publish_time',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'publishTime'
    },
  ],
};

/// Descriptor for `SellerPublicationSummary`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sellerPublicationSummaryDescriptor = $convert.base64Decode(
    'ChhTZWxsZXJQdWJsaWNhdGlvblN1bW1hcnkSJQoOcHVibGljYXRpb25faWQYASABKAlSDXB1Ym'
    'xpY2F0aW9uSWQSLwoTcHVibGljYXRpb25fdmVyc2lvbhgCIAEoCVIScHVibGljYXRpb25WZXJz'
    'aW9uEjsKGnNlbGxlcl9mYWNpbmdfcGF5bG9hZF9oYXNoGAMgASgJUhdzZWxsZXJGYWNpbmdQYX'
    'lsb2FkSGFzaBJWCg5kZWxpdmVyeV9zdGF0ZRgEIAEoDjIvLmhhd2tyLnByb2N1cmVtZW50LnYx'
    'YWxwaGExLkludml0ZURlbGl2ZXJ5U3RhdGVSDWRlbGl2ZXJ5U3RhdGUSPQoMcHVibGlzaF90aW'
    '1lGAUgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcFILcHVibGlzaFRpbWU=');

@$core.Deprecated('Use sellerResponseDraftDescriptor instead')
const SellerResponseDraft$json = {
  '1': 'SellerResponseDraft',
  '2': [
    {'1': 'draft_version', '3': 1, '4': 1, '5': 9, '10': 'draftVersion'},
    {
      '1': 'update_time',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'updateTime'
    },
    {
      '1': 'draft',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.SellerQuote',
      '10': 'draft'
    },
  ],
};

/// Descriptor for `SellerResponseDraft`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sellerResponseDraftDescriptor = $convert.base64Decode(
    'ChNTZWxsZXJSZXNwb25zZURyYWZ0EiMKDWRyYWZ0X3ZlcnNpb24YASABKAlSDGRyYWZ0VmVyc2'
    'lvbhJACgt1cGRhdGVfdGltZRgCIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBCA+BB'
    'A1IKdXBkYXRlVGltZRI9CgVkcmFmdBgDIAEoCzInLmhhd2tyLnByb2N1cmVtZW50LnYxYWxwaG'
    'ExLlNlbGxlclF1b3RlUgVkcmFmdA==');

@$core.Deprecated('Use sellerResponseVersionDescriptor instead')
const SellerResponseVersion$json = {
  '1': 'SellerResponseVersion',
  '2': [
    {
      '1': 'seller_response_id',
      '3': 1,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'sellerResponseId'
    },
    {
      '1': 'seller_response_version',
      '3': 2,
      '4': 1,
      '5': 9,
      '10': 'sellerResponseVersion'
    },
    {'1': 'publication_id', '3': 3, '4': 1, '5': 9, '10': 'publicationId'},
    {
      '1': 'status',
      '3': 4,
      '4': 1,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.SellerResponseStatus',
      '10': 'status'
    },
    {'1': 'payload_hash', '3': 5, '4': 1, '5': 9, '10': 'payloadHash'},
    {
      '1': 'submit_time',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'submitTime'
    },
    {
      '1': 'superseded_by_version',
      '3': 7,
      '4': 1,
      '5': 9,
      '10': 'supersededByVersion'
    },
    {
      '1': 'outside_buyer_requested_range',
      '3': 8,
      '4': 1,
      '5': 8,
      '10': 'outsideBuyerRequestedRange'
    },
    {
      '1': 'quote',
      '3': 9,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.SellerQuote',
      '10': 'quote'
    },
    {'1': 'replayed', '3': 10, '4': 1, '5': 8, '10': 'replayed'},
  ],
};

/// Descriptor for `SellerResponseVersion`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sellerResponseVersionDescriptor = $convert.base64Decode(
    'ChVTZWxsZXJSZXNwb25zZVZlcnNpb24SMQoSc2VsbGVyX3Jlc3BvbnNlX2lkGAEgASgJQgPgQQ'
    'hSEHNlbGxlclJlc3BvbnNlSWQSNgoXc2VsbGVyX3Jlc3BvbnNlX3ZlcnNpb24YAiABKAlSFXNl'
    'bGxlclJlc3BvbnNlVmVyc2lvbhIlCg5wdWJsaWNhdGlvbl9pZBgDIAEoCVINcHVibGljYXRpb2'
    '5JZBJICgZzdGF0dXMYBCABKA4yMC5oYXdrci5wcm9jdXJlbWVudC52MWFscGhhMS5TZWxsZXJS'
    'ZXNwb25zZVN0YXR1c1IGc3RhdHVzEiEKDHBheWxvYWRfaGFzaBgFIAEoCVILcGF5bG9hZEhhc2'
    'gSOwoLc3VibWl0X3RpbWUYBiABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wUgpzdWJt'
    'aXRUaW1lEjIKFXN1cGVyc2VkZWRfYnlfdmVyc2lvbhgHIAEoCVITc3VwZXJzZWRlZEJ5VmVyc2'
    'lvbhJBCh1vdXRzaWRlX2J1eWVyX3JlcXVlc3RlZF9yYW5nZRgIIAEoCFIab3V0c2lkZUJ1eWVy'
    'UmVxdWVzdGVkUmFuZ2USPQoFcXVvdGUYCSABKAsyJy5oYXdrci5wcm9jdXJlbWVudC52MWFscG'
    'hhMS5TZWxsZXJRdW90ZVIFcXVvdGUSGgoIcmVwbGF5ZWQYCiABKAhSCHJlcGxheWVk');

@$core.Deprecated('Use sellerResponseClosureDescriptor instead')
const SellerResponseClosure$json = {
  '1': 'SellerResponseClosure',
  '2': [
    {
      '1': 'seller_response_id',
      '3': 1,
      '4': 1,
      '5': 9,
      '10': 'sellerResponseId'
    },
    {'1': 'publication_id', '3': 2, '4': 1, '5': 9, '10': 'publicationId'},
    {'1': 'latest_version', '3': 3, '4': 1, '5': 9, '10': 'latestVersion'},
    {
      '1': 'withdrawn_version',
      '3': 4,
      '4': 1,
      '5': 9,
      '10': 'withdrawnVersion'
    },
    {
      '1': 'write_closed_time',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 0,
      '10': 'writeClosedTime',
      '17': true
    },
    {
      '1': 'write_closed_reason',
      '3': 6,
      '4': 1,
      '5': 9,
      '10': 'writeClosedReason'
    },
  ],
  '8': [
    {'1': '_write_closed_time'},
  ],
};

/// Descriptor for `SellerResponseClosure`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sellerResponseClosureDescriptor = $convert.base64Decode(
    'ChVTZWxsZXJSZXNwb25zZUNsb3N1cmUSLAoSc2VsbGVyX3Jlc3BvbnNlX2lkGAEgASgJUhBzZW'
    'xsZXJSZXNwb25zZUlkEiUKDnB1YmxpY2F0aW9uX2lkGAIgASgJUg1wdWJsaWNhdGlvbklkEiUK'
    'DmxhdGVzdF92ZXJzaW9uGAMgASgJUg1sYXRlc3RWZXJzaW9uEisKEXdpdGhkcmF3bl92ZXJzaW'
    '9uGAQgASgJUhB3aXRoZHJhd25WZXJzaW9uEksKEXdyaXRlX2Nsb3NlZF90aW1lGAUgASgLMhou'
    'Z29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcEgAUg93cml0ZUNsb3NlZFRpbWWIAQESLgoTd3JpdG'
    'VfY2xvc2VkX3JlYXNvbhgGIAEoCVIRd3JpdGVDbG9zZWRSZWFzb25CFAoSX3dyaXRlX2Nsb3Nl'
    'ZF90aW1l');

@$core.Deprecated('Use sellerPublicationListItemDescriptor instead')
const SellerPublicationListItem$json = {
  '1': 'SellerPublicationListItem',
  '2': [
    {
      '1': 'publication',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.SellerPublicationSummary',
      '10': 'publication'
    },
    {
      '1': 'seller_facing_payload',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.SellerFacingRfq',
      '10': 'sellerFacingPayload'
    },
    {
      '1': 'latest_response_status',
      '3': 3,
      '4': 1,
      '5': 14,
      '6': '.hawkr.procurement.v1alpha1.SellerResponseStatus',
      '9': 0,
      '10': 'latestResponseStatus',
      '17': true
    },
    {'1': 'has_draft', '3': 4, '4': 1, '5': 8, '10': 'hasDraft'},
  ],
  '8': [
    {'1': '_latest_response_status'},
  ],
};

/// Descriptor for `SellerPublicationListItem`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sellerPublicationListItemDescriptor = $convert.base64Decode(
    'ChlTZWxsZXJQdWJsaWNhdGlvbkxpc3RJdGVtElYKC3B1YmxpY2F0aW9uGAEgASgLMjQuaGF3a3'
    'IucHJvY3VyZW1lbnQudjFhbHBoYTEuU2VsbGVyUHVibGljYXRpb25TdW1tYXJ5UgtwdWJsaWNh'
    'dGlvbhJfChVzZWxsZXJfZmFjaW5nX3BheWxvYWQYAiABKAsyKy5oYXdrci5wcm9jdXJlbWVudC'
    '52MWFscGhhMS5TZWxsZXJGYWNpbmdSZnFSE3NlbGxlckZhY2luZ1BheWxvYWQSawoWbGF0ZXN0'
    'X3Jlc3BvbnNlX3N0YXR1cxgDIAEoDjIwLmhhd2tyLnByb2N1cmVtZW50LnYxYWxwaGExLlNlbG'
    'xlclJlc3BvbnNlU3RhdHVzSABSFGxhdGVzdFJlc3BvbnNlU3RhdHVziAEBEhsKCWhhc19kcmFm'
    'dBgEIAEoCFIIaGFzRHJhZnRCGQoXX2xhdGVzdF9yZXNwb25zZV9zdGF0dXM=');

@$core.Deprecated('Use listSellerPublicationsRequestDescriptor instead')
const ListSellerPublicationsRequest$json = {
  '1': 'ListSellerPublicationsRequest',
  '2': [
    {'1': 'page_size', '3': 1, '4': 1, '5': 5, '10': 'pageSize'},
    {'1': 'page_token', '3': 2, '4': 1, '5': 9, '10': 'pageToken'},
  ],
};

/// Descriptor for `ListSellerPublicationsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listSellerPublicationsRequestDescriptor =
    $convert.base64Decode(
        'Ch1MaXN0U2VsbGVyUHVibGljYXRpb25zUmVxdWVzdBIbCglwYWdlX3NpemUYASABKAVSCHBhZ2'
        'VTaXplEh0KCnBhZ2VfdG9rZW4YAiABKAlSCXBhZ2VUb2tlbg==');

@$core.Deprecated('Use listSellerPublicationsResponseDescriptor instead')
const ListSellerPublicationsResponse$json = {
  '1': 'ListSellerPublicationsResponse',
  '2': [
    {
      '1': 'publications',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.SellerPublicationListItem',
      '10': 'publications'
    },
    {'1': 'next_page_token', '3': 2, '4': 1, '5': 9, '10': 'nextPageToken'},
  ],
};

/// Descriptor for `ListSellerPublicationsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listSellerPublicationsResponseDescriptor =
    $convert.base64Decode(
        'Ch5MaXN0U2VsbGVyUHVibGljYXRpb25zUmVzcG9uc2USWQoMcHVibGljYXRpb25zGAEgAygLMj'
        'UuaGF3a3IucHJvY3VyZW1lbnQudjFhbHBoYTEuU2VsbGVyUHVibGljYXRpb25MaXN0SXRlbVIM'
        'cHVibGljYXRpb25zEiYKD25leHRfcGFnZV90b2tlbhgCIAEoCVINbmV4dFBhZ2VUb2tlbg==');

@$core.Deprecated('Use getSellerPublicationRequestDescriptor instead')
const GetSellerPublicationRequest$json = {
  '1': 'GetSellerPublicationRequest',
  '2': [
    {
      '1': 'publication_id',
      '3': 1,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'publicationId'
    },
  ],
};

/// Descriptor for `GetSellerPublicationRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getSellerPublicationRequestDescriptor =
    $convert.base64Decode(
        'ChtHZXRTZWxsZXJQdWJsaWNhdGlvblJlcXVlc3QSKgoOcHVibGljYXRpb25faWQYASABKAlCA+'
        'BBAlINcHVibGljYXRpb25JZA==');

@$core.Deprecated('Use getSellerPublicationResponseDescriptor instead')
const GetSellerPublicationResponse$json = {
  '1': 'GetSellerPublicationResponse',
  '2': [
    {
      '1': 'publication',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.SellerPublicationSummary',
      '10': 'publication'
    },
    {
      '1': 'draft',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.SellerResponseDraft',
      '9': 0,
      '10': 'draft',
      '17': true
    },
    {
      '1': 'latest_response',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.SellerResponseVersion',
      '9': 1,
      '10': 'latestResponse',
      '17': true
    },
    {
      '1': 'seller_facing_payload',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.SellerFacingRfq',
      '10': 'sellerFacingPayload'
    },
  ],
  '8': [
    {'1': '_draft'},
    {'1': '_latest_response'},
  ],
};

/// Descriptor for `GetSellerPublicationResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getSellerPublicationResponseDescriptor = $convert.base64Decode(
    'ChxHZXRTZWxsZXJQdWJsaWNhdGlvblJlc3BvbnNlElYKC3B1YmxpY2F0aW9uGAEgASgLMjQuaG'
    'F3a3IucHJvY3VyZW1lbnQudjFhbHBoYTEuU2VsbGVyUHVibGljYXRpb25TdW1tYXJ5UgtwdWJs'
    'aWNhdGlvbhJKCgVkcmFmdBgCIAEoCzIvLmhhd2tyLnByb2N1cmVtZW50LnYxYWxwaGExLlNlbG'
    'xlclJlc3BvbnNlRHJhZnRIAFIFZHJhZnSIAQESXwoPbGF0ZXN0X3Jlc3BvbnNlGAMgASgLMjEu'
    'aGF3a3IucHJvY3VyZW1lbnQudjFhbHBoYTEuU2VsbGVyUmVzcG9uc2VWZXJzaW9uSAFSDmxhdG'
    'VzdFJlc3BvbnNliAEBEl8KFXNlbGxlcl9mYWNpbmdfcGF5bG9hZBgEIAEoCzIrLmhhd2tyLnBy'
    'b2N1cmVtZW50LnYxYWxwaGExLlNlbGxlckZhY2luZ1JmcVITc2VsbGVyRmFjaW5nUGF5bG9hZE'
    'IICgZfZHJhZnRCEgoQX2xhdGVzdF9yZXNwb25zZQ==');

@$core.Deprecated('Use upsertSellerResponseDraftRequestDescriptor instead')
const UpsertSellerResponseDraftRequest$json = {
  '1': 'UpsertSellerResponseDraftRequest',
  '2': [
    {
      '1': 'publication_id',
      '3': 1,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'publicationId'
    },
    {
      '1': 'expected_draft_version',
      '3': 2,
      '4': 1,
      '5': 9,
      '10': 'expectedDraftVersion'
    },
    {
      '1': 'draft',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.SellerQuote',
      '8': {},
      '10': 'draft'
    },
  ],
};

/// Descriptor for `UpsertSellerResponseDraftRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List upsertSellerResponseDraftRequestDescriptor =
    $convert.base64Decode(
        'CiBVcHNlcnRTZWxsZXJSZXNwb25zZURyYWZ0UmVxdWVzdBIqCg5wdWJsaWNhdGlvbl9pZBgBIA'
        'EoCUID4EECUg1wdWJsaWNhdGlvbklkEjQKFmV4cGVjdGVkX2RyYWZ0X3ZlcnNpb24YAiABKAlS'
        'FGV4cGVjdGVkRHJhZnRWZXJzaW9uEkIKBWRyYWZ0GAMgASgLMicuaGF3a3IucHJvY3VyZW1lbn'
        'QudjFhbHBoYTEuU2VsbGVyUXVvdGVCA+BBAlIFZHJhZnQ=');

@$core.Deprecated('Use upsertSellerResponseDraftResponseDescriptor instead')
const UpsertSellerResponseDraftResponse$json = {
  '1': 'UpsertSellerResponseDraftResponse',
  '2': [
    {
      '1': 'draft',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.SellerResponseDraft',
      '10': 'draft'
    },
  ],
};

/// Descriptor for `UpsertSellerResponseDraftResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List upsertSellerResponseDraftResponseDescriptor =
    $convert.base64Decode(
        'CiFVcHNlcnRTZWxsZXJSZXNwb25zZURyYWZ0UmVzcG9uc2USRQoFZHJhZnQYASABKAsyLy5oYX'
        'drci5wcm9jdXJlbWVudC52MWFscGhhMS5TZWxsZXJSZXNwb25zZURyYWZ0UgVkcmFmdA==');

@$core.Deprecated('Use submitSellerResponseRequestDescriptor instead')
const SubmitSellerResponseRequest$json = {
  '1': 'SubmitSellerResponseRequest',
  '2': [
    {
      '1': 'publication_id',
      '3': 1,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'publicationId'
    },
    {
      '1': 'client_request_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'clientRequestId'
    },
    {
      '1': 'expected_publication_version',
      '3': 3,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'expectedPublicationVersion'
    },
    {
      '1': 'expected_seller_facing_payload_hash',
      '3': 4,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'expectedSellerFacingPayloadHash'
    },
    {
      '1': 'expected_seller_response_version',
      '3': 5,
      '4': 1,
      '5': 9,
      '10': 'expectedSellerResponseVersion'
    },
    {
      '1': 'quote',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.SellerQuote',
      '8': {},
      '10': 'quote'
    },
  ],
};

/// Descriptor for `SubmitSellerResponseRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List submitSellerResponseRequestDescriptor = $convert.base64Decode(
    'ChtTdWJtaXRTZWxsZXJSZXNwb25zZVJlcXVlc3QSKgoOcHVibGljYXRpb25faWQYASABKAlCA+'
    'BBAlINcHVibGljYXRpb25JZBI3ChFjbGllbnRfcmVxdWVzdF9pZBgCIAEoCUIL4EEC4ozP1wgC'
    'CAFSD2NsaWVudFJlcXVlc3RJZBJFChxleHBlY3RlZF9wdWJsaWNhdGlvbl92ZXJzaW9uGAMgAS'
    'gJQgPgQQJSGmV4cGVjdGVkUHVibGljYXRpb25WZXJzaW9uElEKI2V4cGVjdGVkX3NlbGxlcl9m'
    'YWNpbmdfcGF5bG9hZF9oYXNoGAQgASgJQgPgQQJSH2V4cGVjdGVkU2VsbGVyRmFjaW5nUGF5bG'
    '9hZEhhc2gSRwogZXhwZWN0ZWRfc2VsbGVyX3Jlc3BvbnNlX3ZlcnNpb24YBSABKAlSHWV4cGVj'
    'dGVkU2VsbGVyUmVzcG9uc2VWZXJzaW9uEkIKBXF1b3RlGAYgASgLMicuaGF3a3IucHJvY3VyZW'
    '1lbnQudjFhbHBoYTEuU2VsbGVyUXVvdGVCA+BBAlIFcXVvdGU=');

@$core.Deprecated('Use submitSellerResponseResponseDescriptor instead')
const SubmitSellerResponseResponse$json = {
  '1': 'SubmitSellerResponseResponse',
  '2': [
    {
      '1': 'seller_response',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.SellerResponseVersion',
      '10': 'sellerResponse'
    },
  ],
};

/// Descriptor for `SubmitSellerResponseResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List submitSellerResponseResponseDescriptor =
    $convert.base64Decode(
        'ChxTdWJtaXRTZWxsZXJSZXNwb25zZVJlc3BvbnNlEloKD3NlbGxlcl9yZXNwb25zZRgBIAEoCz'
        'IxLmhhd2tyLnByb2N1cmVtZW50LnYxYWxwaGExLlNlbGxlclJlc3BvbnNlVmVyc2lvblIOc2Vs'
        'bGVyUmVzcG9uc2U=');

@$core.Deprecated('Use withdrawSellerResponseRequestDescriptor instead')
const WithdrawSellerResponseRequest$json = {
  '1': 'WithdrawSellerResponseRequest',
  '2': [
    {
      '1': 'publication_id',
      '3': 1,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'publicationId'
    },
    {
      '1': 'seller_response_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'sellerResponseId'
    },
    {
      '1': 'client_request_id',
      '3': 3,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'clientRequestId'
    },
    {
      '1': 'expected_seller_response_version',
      '3': 4,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'expectedSellerResponseVersion'
    },
    {'1': 'reason_code', '3': 5, '4': 1, '5': 9, '10': 'reasonCode'},
  ],
};

/// Descriptor for `WithdrawSellerResponseRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List withdrawSellerResponseRequestDescriptor = $convert.base64Decode(
    'Ch1XaXRoZHJhd1NlbGxlclJlc3BvbnNlUmVxdWVzdBIqCg5wdWJsaWNhdGlvbl9pZBgBIAEoCU'
    'ID4EECUg1wdWJsaWNhdGlvbklkEjEKEnNlbGxlcl9yZXNwb25zZV9pZBgCIAEoCUID4EECUhBz'
    'ZWxsZXJSZXNwb25zZUlkEjcKEWNsaWVudF9yZXF1ZXN0X2lkGAMgASgJQgvgQQLijM/XCAIIAV'
    'IPY2xpZW50UmVxdWVzdElkEkwKIGV4cGVjdGVkX3NlbGxlcl9yZXNwb25zZV92ZXJzaW9uGAQg'
    'ASgJQgPgQQJSHWV4cGVjdGVkU2VsbGVyUmVzcG9uc2VWZXJzaW9uEh8KC3JlYXNvbl9jb2RlGA'
    'UgASgJUgpyZWFzb25Db2Rl');

@$core.Deprecated('Use withdrawSellerResponseResponseDescriptor instead')
const WithdrawSellerResponseResponse$json = {
  '1': 'WithdrawSellerResponseResponse',
  '2': [
    {
      '1': 'closure',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.SellerResponseClosure',
      '10': 'closure'
    },
  ],
};

/// Descriptor for `WithdrawSellerResponseResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List withdrawSellerResponseResponseDescriptor =
    $convert.base64Decode(
        'Ch5XaXRoZHJhd1NlbGxlclJlc3BvbnNlUmVzcG9uc2USSwoHY2xvc3VyZRgBIAEoCzIxLmhhd2'
        'tyLnByb2N1cmVtZW50LnYxYWxwaGExLlNlbGxlclJlc3BvbnNlQ2xvc3VyZVIHY2xvc3VyZQ==');

@$core.Deprecated('Use declineRfqPublicationRequestDescriptor instead')
const DeclineRfqPublicationRequest$json = {
  '1': 'DeclineRfqPublicationRequest',
  '2': [
    {
      '1': 'publication_id',
      '3': 1,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'publicationId'
    },
    {
      '1': 'client_request_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'clientRequestId'
    },
    {'1': 'reason_code', '3': 3, '4': 1, '5': 9, '10': 'reasonCode'},
  ],
};

/// Descriptor for `DeclineRfqPublicationRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List declineRfqPublicationRequestDescriptor = $convert.base64Decode(
    'ChxEZWNsaW5lUmZxUHVibGljYXRpb25SZXF1ZXN0EioKDnB1YmxpY2F0aW9uX2lkGAEgASgJQg'
    'PgQQJSDXB1YmxpY2F0aW9uSWQSNwoRY2xpZW50X3JlcXVlc3RfaWQYAiABKAlCC+BBAuKMz9cI'
    'AggBUg9jbGllbnRSZXF1ZXN0SWQSHwoLcmVhc29uX2NvZGUYAyABKAlSCnJlYXNvbkNvZGU=');

@$core.Deprecated('Use declineRfqPublicationResponseDescriptor instead')
const DeclineRfqPublicationResponse$json = {
  '1': 'DeclineRfqPublicationResponse',
  '2': [
    {
      '1': 'closure',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.procurement.v1alpha1.SellerResponseClosure',
      '10': 'closure'
    },
  ],
};

/// Descriptor for `DeclineRfqPublicationResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List declineRfqPublicationResponseDescriptor =
    $convert.base64Decode(
        'Ch1EZWNsaW5lUmZxUHVibGljYXRpb25SZXNwb25zZRJLCgdjbG9zdXJlGAEgASgLMjEuaGF3a3'
        'IucHJvY3VyZW1lbnQudjFhbHBoYTEuU2VsbGVyUmVzcG9uc2VDbG9zdXJlUgdjbG9zdXJl');

const $core.Map<$core.String, $core.dynamic>
    BuyerProcurementAppServiceBase$json = {
  '1': 'BuyerProcurementAppService',
  '2': [
    {
      '1': 'SubmitTurn',
      '2': '.hawkr.procurement.v1alpha1.SubmitTurnRequest',
      '3': '.hawkr.procurement.v1alpha1.SubmitTurnResponse'
    },
    {
      '1': 'GetSnapshot',
      '2': '.hawkr.procurement.v1alpha1.GetSnapshotRequest',
      '3': '.hawkr.procurement.v1alpha1.GetSnapshotResponse',
      '4': {'34': 1},
    },
    {
      '1': 'StartCandidateSearch',
      '2': '.hawkr.procurement.v1alpha1.StartCandidateSearchRequest',
      '3': '.hawkr.procurement.v1alpha1.StartCandidateSearchResponse'
    },
    {
      '1': 'GetCandidateSearch',
      '2': '.hawkr.procurement.v1alpha1.GetCandidateSearchRequest',
      '3': '.hawkr.procurement.v1alpha1.GetCandidateSearchResponse',
      '4': {'34': 1},
    },
    {
      '1': 'ToggleSelection',
      '2': '.hawkr.procurement.v1alpha1.ToggleSelectionRequest',
      '3': '.hawkr.procurement.v1alpha1.ToggleSelectionResponse'
    },
    {
      '1': 'SubmitAnswer',
      '2': '.hawkr.procurement.v1alpha1.SubmitAnswerRequest',
      '3': '.hawkr.procurement.v1alpha1.SubmitAnswerResponse'
    },
    {
      '1': 'Rewind',
      '2': '.hawkr.procurement.v1alpha1.RewindRequest',
      '3': '.hawkr.procurement.v1alpha1.RewindResponse'
    },
    {
      '1': 'PreviewRfqPrice',
      '2': '.hawkr.procurement.v1alpha1.PreviewRfqPriceRequest',
      '3': '.hawkr.procurement.v1alpha1.PreviewRfqPriceResponse',
      '4': {'34': 1},
    },
    {
      '1': 'PreviewRfqPublication',
      '2': '.hawkr.procurement.v1alpha1.PreviewRfqPublicationRequest',
      '3': '.hawkr.procurement.v1alpha1.PreviewRfqPublicationResponse'
    },
    {
      '1': 'ExecuteRfqPublication',
      '2': '.hawkr.procurement.v1alpha1.ExecuteRfqPublicationRequest',
      '3': '.hawkr.procurement.v1alpha1.ExecuteRfqPublicationResponse'
    },
    {
      '1': 'ListConversations',
      '2': '.hawkr.procurement.v1alpha1.ListConversationsRequest',
      '3': '.hawkr.procurement.v1alpha1.ListConversationsResponse',
      '4': {'34': 1},
    },
    {
      '1': 'GetConversationMessages',
      '2': '.hawkr.procurement.v1alpha1.GetConversationMessagesRequest',
      '3': '.hawkr.procurement.v1alpha1.GetConversationMessagesResponse',
      '4': {'34': 1},
    },
    {
      '1': 'GetRuntimeSnapshot',
      '2': '.hawkr.procurement.v1alpha1.GetRuntimeSnapshotRequest',
      '3': '.hawkr.procurement.v1alpha1.GetRuntimeSnapshotResponse',
      '4': {'34': 1},
    },
    {
      '1': 'ListSellerResponses',
      '2': '.hawkr.procurement.v1alpha1.ListSellerResponsesRequest',
      '3': '.hawkr.procurement.v1alpha1.ListSellerResponsesResponse',
      '4': {'34': 1},
    },
    {
      '1': 'PutShortlistEntry',
      '2': '.hawkr.procurement.v1alpha1.PutShortlistEntryRequest',
      '3': '.hawkr.procurement.v1alpha1.PutShortlistEntryResponse'
    },
    {
      '1': 'DeleteShortlistEntry',
      '2': '.hawkr.procurement.v1alpha1.DeleteShortlistEntryRequest',
      '3': '.hawkr.procurement.v1alpha1.DeleteShortlistEntryResponse'
    },
    {
      '1': 'AcceptSellerQuote',
      '2': '.hawkr.procurement.v1alpha1.AcceptSellerQuoteRequest',
      '3': '.hawkr.procurement.v1alpha1.AcceptSellerQuoteResponse'
    },
    {
      '1': 'RenameConversation',
      '2': '.hawkr.procurement.v1alpha1.RenameConversationRequest',
      '3': '.hawkr.procurement.v1alpha1.RenameConversationResponse'
    },
    {
      '1': 'DeleteConversation',
      '2': '.hawkr.procurement.v1alpha1.DeleteConversationRequest',
      '3': '.hawkr.procurement.v1alpha1.DeleteConversationResponse'
    },
    {
      '1': 'WatchRun',
      '2': '.hawkr.procurement.v1alpha1.WatchRunRequest',
      '3': '.hawkr.procurement.v1alpha1.WatchRunResponse',
      '6': true
    },
    {
      '1': 'CancelRun',
      '2': '.hawkr.procurement.v1alpha1.CancelRunRequest',
      '3': '.hawkr.procurement.v1alpha1.CancelRunResponse'
    },
    {
      '1': 'ResumeRun',
      '2': '.hawkr.procurement.v1alpha1.ResumeRunRequest',
      '3': '.hawkr.procurement.v1alpha1.ResumeRunResponse'
    },
  ],
};

@$core.Deprecated('Use buyerProcurementAppServiceDescriptor instead')
const $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
    BuyerProcurementAppServiceBase$messageJson = {
  '.hawkr.procurement.v1alpha1.SubmitTurnRequest': SubmitTurnRequest$json,
  '.hawkr.procurement.v1alpha1.SubmitTurnResponse': SubmitTurnResponse$json,
  '.hawkr.procurement.v1alpha1.GetSnapshotRequest': GetSnapshotRequest$json,
  '.hawkr.procurement.v1alpha1.GetSnapshotResponse': GetSnapshotResponse$json,
  '.hawkr.procurement.v1alpha1.Slots': Slots$json,
  '.google.type.Date': $0.Date$json,
  '.hawkr.procurement.v1alpha1.ItemResolution': ItemResolution$json,
  '.hawkr.procurement.v1alpha1.RunSummary': RunSummary$json,
  '.hawkr.procurement.v1alpha1.OpenConfirmation': OpenConfirmation$json,
  '.hawkr.procurement.v1alpha1.ClarifyQuestion': ClarifyQuestion$json,
  '.hawkr.procurement.v1alpha1.QuestionOption': QuestionOption$json,
  '.hawkr.procurement.v1alpha1.PublishedRfq': PublishedRfq$json,
  '.google.protobuf.Timestamp': $1.Timestamp$json,
  '.hawkr.procurement.v1alpha1.StartCandidateSearchRequest':
      StartCandidateSearchRequest$json,
  '.hawkr.procurement.v1alpha1.StartCandidateSearchResponse':
      StartCandidateSearchResponse$json,
  '.hawkr.procurement.v1alpha1.GetCandidateSearchRequest':
      GetCandidateSearchRequest$json,
  '.hawkr.procurement.v1alpha1.GetCandidateSearchResponse':
      GetCandidateSearchResponse$json,
  '.hawkr.procurement.v1alpha1.Candidate': Candidate$json,
  '.hawkr.procurement.v1alpha1.MatchedFact': MatchedFact$json,
  '.hawkr.procurement.v1alpha1.UnverifiableFact': UnverifiableFact$json,
  '.hawkr.common.v1alpha1.StoreCard': $2.StoreCard$json,
  '.hawkr.common.v1alpha1.Image': $3.Image$json,
  '.hawkr.procurement.v1alpha1.ToggleSelectionRequest':
      ToggleSelectionRequest$json,
  '.hawkr.procurement.v1alpha1.ToggleSelectionResponse':
      ToggleSelectionResponse$json,
  '.hawkr.procurement.v1alpha1.SubmitAnswerRequest': SubmitAnswerRequest$json,
  '.hawkr.procurement.v1alpha1.SubmitAnswerResponse': SubmitAnswerResponse$json,
  '.hawkr.procurement.v1alpha1.TurnResult': TurnResult$json,
  '.hawkr.procurement.v1alpha1.RejectedField': RejectedField$json,
  '.hawkr.procurement.v1alpha1.RecordedField': RecordedField$json,
  '.hawkr.procurement.v1alpha1.Step': Step$json,
  '.hawkr.procurement.v1alpha1.Step.SafeArgsEntry': Step_SafeArgsEntry$json,
  '.hawkr.procurement.v1alpha1.Block': Block$json,
  '.hawkr.procurement.v1alpha1.EvidenceRef': EvidenceRef$json,
  '.hawkr.procurement.v1alpha1.Interrupt': Interrupt$json,
  '.hawkr.procurement.v1alpha1.QuotaSnapshot': QuotaSnapshot$json,
  '.hawkr.procurement.v1alpha1.RewindRequest': RewindRequest$json,
  '.hawkr.procurement.v1alpha1.RewindResponse': RewindResponse$json,
  '.hawkr.procurement.v1alpha1.PreviewRfqPriceRequest':
      PreviewRfqPriceRequest$json,
  '.hawkr.procurement.v1alpha1.BuyerRequestedRange': BuyerRequestedRange$json,
  '.hawkr.procurement.v1alpha1.PreviewRfqPriceResponse':
      PreviewRfqPriceResponse$json,
  '.hawkr.procurement.v1alpha1.QuoteTotalRange': QuoteTotalRange$json,
  '.hawkr.procurement.v1alpha1.PreviewRfqPublicationRequest':
      PreviewRfqPublicationRequest$json,
  '.hawkr.procurement.v1alpha1.PreviewRfqPublicationResponse':
      PreviewRfqPublicationResponse$json,
  '.hawkr.procurement.v1alpha1.SellerFacingRfq': SellerFacingRfq$json,
  '.hawkr.procurement.v1alpha1.SellerFacingRequestedRange':
      SellerFacingRequestedRange$json,
  '.hawkr.procurement.v1alpha1.ExecuteRfqPublicationRequest':
      ExecuteRfqPublicationRequest$json,
  '.hawkr.procurement.v1alpha1.ExecuteRfqPublicationResponse':
      ExecuteRfqPublicationResponse$json,
  '.hawkr.procurement.v1alpha1.ListConversationsRequest':
      ListConversationsRequest$json,
  '.hawkr.procurement.v1alpha1.ListConversationsResponse':
      ListConversationsResponse$json,
  '.hawkr.procurement.v1alpha1.ConversationSummary': ConversationSummary$json,
  '.hawkr.procurement.v1alpha1.GetConversationMessagesRequest':
      GetConversationMessagesRequest$json,
  '.hawkr.procurement.v1alpha1.GetConversationMessagesResponse':
      GetConversationMessagesResponse$json,
  '.hawkr.procurement.v1alpha1.ConversationMessage': ConversationMessage$json,
  '.hawkr.procurement.v1alpha1.GetRuntimeSnapshotRequest':
      GetRuntimeSnapshotRequest$json,
  '.hawkr.procurement.v1alpha1.GetRuntimeSnapshotResponse':
      GetRuntimeSnapshotResponse$json,
  '.hawkr.procurement.v1alpha1.ListSellerResponsesRequest':
      ListSellerResponsesRequest$json,
  '.hawkr.procurement.v1alpha1.ListSellerResponsesResponse':
      ListSellerResponsesResponse$json,
  '.hawkr.procurement.v1alpha1.BuyerFacingSellerResponse':
      BuyerFacingSellerResponse$json,
  '.hawkr.procurement.v1alpha1.SellerQuote': SellerQuote$json,
  '.hawkr.procurement.v1alpha1.OfferedQuality': OfferedQuality$json,
  '.hawkr.procurement.v1alpha1.UnitBasis': UnitBasis$json,
  '.hawkr.procurement.v1alpha1.PutShortlistEntryRequest':
      PutShortlistEntryRequest$json,
  '.hawkr.procurement.v1alpha1.PutShortlistEntryResponse':
      PutShortlistEntryResponse$json,
  '.hawkr.procurement.v1alpha1.DeleteShortlistEntryRequest':
      DeleteShortlistEntryRequest$json,
  '.hawkr.procurement.v1alpha1.DeleteShortlistEntryResponse':
      DeleteShortlistEntryResponse$json,
  '.hawkr.procurement.v1alpha1.AcceptSellerQuoteRequest':
      AcceptSellerQuoteRequest$json,
  '.hawkr.procurement.v1alpha1.AcceptSellerQuoteResponse':
      AcceptSellerQuoteResponse$json,
  '.hawkr.procurement.v1alpha1.RenameConversationRequest':
      RenameConversationRequest$json,
  '.hawkr.procurement.v1alpha1.RenameConversationResponse':
      RenameConversationResponse$json,
  '.hawkr.procurement.v1alpha1.DeleteConversationRequest':
      DeleteConversationRequest$json,
  '.hawkr.procurement.v1alpha1.DeleteConversationResponse':
      DeleteConversationResponse$json,
  '.hawkr.procurement.v1alpha1.WatchRunRequest': WatchRunRequest$json,
  '.hawkr.procurement.v1alpha1.WatchRunResponse': WatchRunResponse$json,
  '.hawkr.procurement.v1alpha1.RunAccepted': RunAccepted$json,
  '.hawkr.procurement.v1alpha1.TextDelta': TextDelta$json,
  '.hawkr.procurement.v1alpha1.StepEvent': StepEvent$json,
  '.hawkr.procurement.v1alpha1.Activity': Activity$json,
  '.hawkr.procurement.v1alpha1.MessageEvent': MessageEvent$json,
  '.hawkr.procurement.v1alpha1.ToolResult': ToolResult$json,
  '.hawkr.procurement.v1alpha1.RunFinished': RunFinished$json,
  '.hawkr.procurement.v1alpha1.CancelRunRequest': CancelRunRequest$json,
  '.hawkr.procurement.v1alpha1.CancelRunResponse': CancelRunResponse$json,
  '.hawkr.procurement.v1alpha1.ResumeRunRequest': ResumeRunRequest$json,
  '.hawkr.procurement.v1alpha1.ClarificationResponse':
      ClarificationResponse$json,
  '.hawkr.procurement.v1alpha1.ResumeRunResponse': ResumeRunResponse$json,
};

/// Descriptor for `BuyerProcurementAppService`. Decode as a `google.protobuf.ServiceDescriptorProto`.
final $typed_data.Uint8List buyerProcurementAppServiceDescriptor = $convert.base64Decode(
    'ChpCdXllclByb2N1cmVtZW50QXBwU2VydmljZRJrCgpTdWJtaXRUdXJuEi0uaGF3a3IucHJvY3'
    'VyZW1lbnQudjFhbHBoYTEuU3VibWl0VHVyblJlcXVlc3QaLi5oYXdrci5wcm9jdXJlbWVudC52'
    'MWFscGhhMS5TdWJtaXRUdXJuUmVzcG9uc2UScwoLR2V0U25hcHNob3QSLi5oYXdrci5wcm9jdX'
    'JlbWVudC52MWFscGhhMS5HZXRTbmFwc2hvdFJlcXVlc3QaLy5oYXdrci5wcm9jdXJlbWVudC52'
    'MWFscGhhMS5HZXRTbmFwc2hvdFJlc3BvbnNlIgOQAgESiQEKFFN0YXJ0Q2FuZGlkYXRlU2Vhcm'
    'NoEjcuaGF3a3IucHJvY3VyZW1lbnQudjFhbHBoYTEuU3RhcnRDYW5kaWRhdGVTZWFyY2hSZXF1'
    'ZXN0GjguaGF3a3IucHJvY3VyZW1lbnQudjFhbHBoYTEuU3RhcnRDYW5kaWRhdGVTZWFyY2hSZX'
    'Nwb25zZRKIAQoSR2V0Q2FuZGlkYXRlU2VhcmNoEjUuaGF3a3IucHJvY3VyZW1lbnQudjFhbHBo'
    'YTEuR2V0Q2FuZGlkYXRlU2VhcmNoUmVxdWVzdBo2Lmhhd2tyLnByb2N1cmVtZW50LnYxYWxwaG'
    'ExLkdldENhbmRpZGF0ZVNlYXJjaFJlc3BvbnNlIgOQAgESegoPVG9nZ2xlU2VsZWN0aW9uEjIu'
    'aGF3a3IucHJvY3VyZW1lbnQudjFhbHBoYTEuVG9nZ2xlU2VsZWN0aW9uUmVxdWVzdBozLmhhd2'
    'tyLnByb2N1cmVtZW50LnYxYWxwaGExLlRvZ2dsZVNlbGVjdGlvblJlc3BvbnNlEnEKDFN1Ym1p'
    'dEFuc3dlchIvLmhhd2tyLnByb2N1cmVtZW50LnYxYWxwaGExLlN1Ym1pdEFuc3dlclJlcXVlc3'
    'QaMC5oYXdrci5wcm9jdXJlbWVudC52MWFscGhhMS5TdWJtaXRBbnN3ZXJSZXNwb25zZRJfCgZS'
    'ZXdpbmQSKS5oYXdrci5wcm9jdXJlbWVudC52MWFscGhhMS5SZXdpbmRSZXF1ZXN0GiouaGF3a3'
    'IucHJvY3VyZW1lbnQudjFhbHBoYTEuUmV3aW5kUmVzcG9uc2USfwoPUHJldmlld1JmcVByaWNl'
    'EjIuaGF3a3IucHJvY3VyZW1lbnQudjFhbHBoYTEuUHJldmlld1JmcVByaWNlUmVxdWVzdBozLm'
    'hhd2tyLnByb2N1cmVtZW50LnYxYWxwaGExLlByZXZpZXdSZnFQcmljZVJlc3BvbnNlIgOQAgES'
    'jAEKFVByZXZpZXdSZnFQdWJsaWNhdGlvbhI4Lmhhd2tyLnByb2N1cmVtZW50LnYxYWxwaGExLl'
    'ByZXZpZXdSZnFQdWJsaWNhdGlvblJlcXVlc3QaOS5oYXdrci5wcm9jdXJlbWVudC52MWFscGhh'
    'MS5QcmV2aWV3UmZxUHVibGljYXRpb25SZXNwb25zZRKMAQoVRXhlY3V0ZVJmcVB1YmxpY2F0aW'
    '9uEjguaGF3a3IucHJvY3VyZW1lbnQudjFhbHBoYTEuRXhlY3V0ZVJmcVB1YmxpY2F0aW9uUmVx'
    'dWVzdBo5Lmhhd2tyLnByb2N1cmVtZW50LnYxYWxwaGExLkV4ZWN1dGVSZnFQdWJsaWNhdGlvbl'
    'Jlc3BvbnNlEoUBChFMaXN0Q29udmVyc2F0aW9ucxI0Lmhhd2tyLnByb2N1cmVtZW50LnYxYWxw'
    'aGExLkxpc3RDb252ZXJzYXRpb25zUmVxdWVzdBo1Lmhhd2tyLnByb2N1cmVtZW50LnYxYWxwaG'
    'ExLkxpc3RDb252ZXJzYXRpb25zUmVzcG9uc2UiA5ACARKXAQoXR2V0Q29udmVyc2F0aW9uTWVz'
    'c2FnZXMSOi5oYXdrci5wcm9jdXJlbWVudC52MWFscGhhMS5HZXRDb252ZXJzYXRpb25NZXNzYW'
    'dlc1JlcXVlc3QaOy5oYXdrci5wcm9jdXJlbWVudC52MWFscGhhMS5HZXRDb252ZXJzYXRpb25N'
    'ZXNzYWdlc1Jlc3BvbnNlIgOQAgESiAEKEkdldFJ1bnRpbWVTbmFwc2hvdBI1Lmhhd2tyLnByb2'
    'N1cmVtZW50LnYxYWxwaGExLkdldFJ1bnRpbWVTbmFwc2hvdFJlcXVlc3QaNi5oYXdrci5wcm9j'
    'dXJlbWVudC52MWFscGhhMS5HZXRSdW50aW1lU25hcHNob3RSZXNwb25zZSIDkAIBEosBChNMaX'
    'N0U2VsbGVyUmVzcG9uc2VzEjYuaGF3a3IucHJvY3VyZW1lbnQudjFhbHBoYTEuTGlzdFNlbGxl'
    'clJlc3BvbnNlc1JlcXVlc3QaNy5oYXdrci5wcm9jdXJlbWVudC52MWFscGhhMS5MaXN0U2VsbG'
    'VyUmVzcG9uc2VzUmVzcG9uc2UiA5ACARKAAQoRUHV0U2hvcnRsaXN0RW50cnkSNC5oYXdrci5w'
    'cm9jdXJlbWVudC52MWFscGhhMS5QdXRTaG9ydGxpc3RFbnRyeVJlcXVlc3QaNS5oYXdrci5wcm'
    '9jdXJlbWVudC52MWFscGhhMS5QdXRTaG9ydGxpc3RFbnRyeVJlc3BvbnNlEokBChREZWxldGVT'
    'aG9ydGxpc3RFbnRyeRI3Lmhhd2tyLnByb2N1cmVtZW50LnYxYWxwaGExLkRlbGV0ZVNob3J0bG'
    'lzdEVudHJ5UmVxdWVzdBo4Lmhhd2tyLnByb2N1cmVtZW50LnYxYWxwaGExLkRlbGV0ZVNob3J0'
    'bGlzdEVudHJ5UmVzcG9uc2USgAEKEUFjY2VwdFNlbGxlclF1b3RlEjQuaGF3a3IucHJvY3VyZW'
    '1lbnQudjFhbHBoYTEuQWNjZXB0U2VsbGVyUXVvdGVSZXF1ZXN0GjUuaGF3a3IucHJvY3VyZW1l'
    'bnQudjFhbHBoYTEuQWNjZXB0U2VsbGVyUXVvdGVSZXNwb25zZRKDAQoSUmVuYW1lQ29udmVyc2'
    'F0aW9uEjUuaGF3a3IucHJvY3VyZW1lbnQudjFhbHBoYTEuUmVuYW1lQ29udmVyc2F0aW9uUmVx'
    'dWVzdBo2Lmhhd2tyLnByb2N1cmVtZW50LnYxYWxwaGExLlJlbmFtZUNvbnZlcnNhdGlvblJlc3'
    'BvbnNlEoMBChJEZWxldGVDb252ZXJzYXRpb24SNS5oYXdrci5wcm9jdXJlbWVudC52MWFscGhh'
    'MS5EZWxldGVDb252ZXJzYXRpb25SZXF1ZXN0GjYuaGF3a3IucHJvY3VyZW1lbnQudjFhbHBoYT'
    'EuRGVsZXRlQ29udmVyc2F0aW9uUmVzcG9uc2USZwoIV2F0Y2hSdW4SKy5oYXdrci5wcm9jdXJl'
    'bWVudC52MWFscGhhMS5XYXRjaFJ1blJlcXVlc3QaLC5oYXdrci5wcm9jdXJlbWVudC52MWFscG'
    'hhMS5XYXRjaFJ1blJlc3BvbnNlMAESaAoJQ2FuY2VsUnVuEiwuaGF3a3IucHJvY3VyZW1lbnQu'
    'djFhbHBoYTEuQ2FuY2VsUnVuUmVxdWVzdBotLmhhd2tyLnByb2N1cmVtZW50LnYxYWxwaGExLk'
    'NhbmNlbFJ1blJlc3BvbnNlEmgKCVJlc3VtZVJ1bhIsLmhhd2tyLnByb2N1cmVtZW50LnYxYWxw'
    'aGExLlJlc3VtZVJ1blJlcXVlc3QaLS5oYXdrci5wcm9jdXJlbWVudC52MWFscGhhMS5SZXN1bW'
    'VSdW5SZXNwb25zZQ==');

const $core.Map<$core.String, $core.dynamic>
    SellerProcurementAppServiceBase$json = {
  '1': 'SellerProcurementAppService',
  '2': [
    {
      '1': 'ListSellerPublications',
      '2': '.hawkr.procurement.v1alpha1.ListSellerPublicationsRequest',
      '3': '.hawkr.procurement.v1alpha1.ListSellerPublicationsResponse',
      '4': {'34': 1},
    },
    {
      '1': 'GetSellerPublication',
      '2': '.hawkr.procurement.v1alpha1.GetSellerPublicationRequest',
      '3': '.hawkr.procurement.v1alpha1.GetSellerPublicationResponse',
      '4': {'34': 1},
    },
    {
      '1': 'UpsertSellerResponseDraft',
      '2': '.hawkr.procurement.v1alpha1.UpsertSellerResponseDraftRequest',
      '3': '.hawkr.procurement.v1alpha1.UpsertSellerResponseDraftResponse'
    },
    {
      '1': 'SubmitSellerResponse',
      '2': '.hawkr.procurement.v1alpha1.SubmitSellerResponseRequest',
      '3': '.hawkr.procurement.v1alpha1.SubmitSellerResponseResponse'
    },
    {
      '1': 'WithdrawSellerResponse',
      '2': '.hawkr.procurement.v1alpha1.WithdrawSellerResponseRequest',
      '3': '.hawkr.procurement.v1alpha1.WithdrawSellerResponseResponse'
    },
    {
      '1': 'DeclineRfqPublication',
      '2': '.hawkr.procurement.v1alpha1.DeclineRfqPublicationRequest',
      '3': '.hawkr.procurement.v1alpha1.DeclineRfqPublicationResponse'
    },
  ],
};

@$core.Deprecated('Use sellerProcurementAppServiceDescriptor instead')
const $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
    SellerProcurementAppServiceBase$messageJson = {
  '.hawkr.procurement.v1alpha1.ListSellerPublicationsRequest':
      ListSellerPublicationsRequest$json,
  '.hawkr.procurement.v1alpha1.ListSellerPublicationsResponse':
      ListSellerPublicationsResponse$json,
  '.hawkr.procurement.v1alpha1.SellerPublicationListItem':
      SellerPublicationListItem$json,
  '.hawkr.procurement.v1alpha1.SellerPublicationSummary':
      SellerPublicationSummary$json,
  '.google.protobuf.Timestamp': $1.Timestamp$json,
  '.hawkr.procurement.v1alpha1.SellerFacingRfq': SellerFacingRfq$json,
  '.google.type.Date': $0.Date$json,
  '.hawkr.procurement.v1alpha1.SellerFacingRequestedRange':
      SellerFacingRequestedRange$json,
  '.hawkr.procurement.v1alpha1.GetSellerPublicationRequest':
      GetSellerPublicationRequest$json,
  '.hawkr.procurement.v1alpha1.GetSellerPublicationResponse':
      GetSellerPublicationResponse$json,
  '.hawkr.procurement.v1alpha1.SellerResponseDraft': SellerResponseDraft$json,
  '.hawkr.procurement.v1alpha1.SellerQuote': SellerQuote$json,
  '.hawkr.procurement.v1alpha1.OfferedQuality': OfferedQuality$json,
  '.hawkr.procurement.v1alpha1.UnitBasis': UnitBasis$json,
  '.hawkr.procurement.v1alpha1.SellerResponseVersion':
      SellerResponseVersion$json,
  '.hawkr.procurement.v1alpha1.UpsertSellerResponseDraftRequest':
      UpsertSellerResponseDraftRequest$json,
  '.hawkr.procurement.v1alpha1.UpsertSellerResponseDraftResponse':
      UpsertSellerResponseDraftResponse$json,
  '.hawkr.procurement.v1alpha1.SubmitSellerResponseRequest':
      SubmitSellerResponseRequest$json,
  '.hawkr.procurement.v1alpha1.SubmitSellerResponseResponse':
      SubmitSellerResponseResponse$json,
  '.hawkr.procurement.v1alpha1.WithdrawSellerResponseRequest':
      WithdrawSellerResponseRequest$json,
  '.hawkr.procurement.v1alpha1.WithdrawSellerResponseResponse':
      WithdrawSellerResponseResponse$json,
  '.hawkr.procurement.v1alpha1.SellerResponseClosure':
      SellerResponseClosure$json,
  '.hawkr.procurement.v1alpha1.DeclineRfqPublicationRequest':
      DeclineRfqPublicationRequest$json,
  '.hawkr.procurement.v1alpha1.DeclineRfqPublicationResponse':
      DeclineRfqPublicationResponse$json,
};

/// Descriptor for `SellerProcurementAppService`. Decode as a `google.protobuf.ServiceDescriptorProto`.
final $typed_data.Uint8List sellerProcurementAppServiceDescriptor = $convert.base64Decode(
    'ChtTZWxsZXJQcm9jdXJlbWVudEFwcFNlcnZpY2USlAEKFkxpc3RTZWxsZXJQdWJsaWNhdGlvbn'
    'MSOS5oYXdrci5wcm9jdXJlbWVudC52MWFscGhhMS5MaXN0U2VsbGVyUHVibGljYXRpb25zUmVx'
    'dWVzdBo6Lmhhd2tyLnByb2N1cmVtZW50LnYxYWxwaGExLkxpc3RTZWxsZXJQdWJsaWNhdGlvbn'
    'NSZXNwb25zZSIDkAIBEo4BChRHZXRTZWxsZXJQdWJsaWNhdGlvbhI3Lmhhd2tyLnByb2N1cmVt'
    'ZW50LnYxYWxwaGExLkdldFNlbGxlclB1YmxpY2F0aW9uUmVxdWVzdBo4Lmhhd2tyLnByb2N1cm'
    'VtZW50LnYxYWxwaGExLkdldFNlbGxlclB1YmxpY2F0aW9uUmVzcG9uc2UiA5ACARKYAQoZVXBz'
    'ZXJ0U2VsbGVyUmVzcG9uc2VEcmFmdBI8Lmhhd2tyLnByb2N1cmVtZW50LnYxYWxwaGExLlVwc2'
    'VydFNlbGxlclJlc3BvbnNlRHJhZnRSZXF1ZXN0Gj0uaGF3a3IucHJvY3VyZW1lbnQudjFhbHBo'
    'YTEuVXBzZXJ0U2VsbGVyUmVzcG9uc2VEcmFmdFJlc3BvbnNlEokBChRTdWJtaXRTZWxsZXJSZX'
    'Nwb25zZRI3Lmhhd2tyLnByb2N1cmVtZW50LnYxYWxwaGExLlN1Ym1pdFNlbGxlclJlc3BvbnNl'
    'UmVxdWVzdBo4Lmhhd2tyLnByb2N1cmVtZW50LnYxYWxwaGExLlN1Ym1pdFNlbGxlclJlc3Bvbn'
    'NlUmVzcG9uc2USjwEKFldpdGhkcmF3U2VsbGVyUmVzcG9uc2USOS5oYXdrci5wcm9jdXJlbWVu'
    'dC52MWFscGhhMS5XaXRoZHJhd1NlbGxlclJlc3BvbnNlUmVxdWVzdBo6Lmhhd2tyLnByb2N1cm'
    'VtZW50LnYxYWxwaGExLldpdGhkcmF3U2VsbGVyUmVzcG9uc2VSZXNwb25zZRKMAQoVRGVjbGlu'
    'ZVJmcVB1YmxpY2F0aW9uEjguaGF3a3IucHJvY3VyZW1lbnQudjFhbHBoYTEuRGVjbGluZVJmcV'
    'B1YmxpY2F0aW9uUmVxdWVzdBo5Lmhhd2tyLnByb2N1cmVtZW50LnYxYWxwaGExLkRlY2xpbmVS'
    'ZnFQdWJsaWNhdGlvblJlc3BvbnNl');
