// This is a generated file - do not edit.
//
// Generated from hawkr/payment/v1alpha1/payment.proto.

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

@$core.Deprecated('Use paymentAccountStatusDescriptor instead')
const PaymentAccountStatus$json = {
  '1': 'PaymentAccountStatus',
  '2': [
    {'1': 'PAYMENT_ACCOUNT_STATUS_UNSPECIFIED', '2': 0},
    {'1': 'PAYMENT_ACCOUNT_STATUS_NOT_CREATED', '2': 1},
    {'1': 'PAYMENT_ACCOUNT_STATUS_UNDER_REVIEW', '2': 2},
    {'1': 'PAYMENT_ACCOUNT_STATUS_ACTION_REQUIRED', '2': 3},
    {'1': 'PAYMENT_ACCOUNT_STATUS_ENABLED', '2': 4},
    {'1': 'PAYMENT_ACCOUNT_STATUS_DISABLED', '2': 5},
  ],
};

/// Descriptor for `PaymentAccountStatus`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List paymentAccountStatusDescriptor = $convert.base64Decode(
    'ChRQYXltZW50QWNjb3VudFN0YXR1cxImCiJQQVlNRU5UX0FDQ09VTlRfU1RBVFVTX1VOU1BFQ0'
    'lGSUVEEAASJgoiUEFZTUVOVF9BQ0NPVU5UX1NUQVRVU19OT1RfQ1JFQVRFRBABEicKI1BBWU1F'
    'TlRfQUNDT1VOVF9TVEFUVVNfVU5ERVJfUkVWSUVXEAISKgomUEFZTUVOVF9BQ0NPVU5UX1NUQV'
    'RVU19BQ1RJT05fUkVRVUlSRUQQAxIiCh5QQVlNRU5UX0FDQ09VTlRfU1RBVFVTX0VOQUJMRUQQ'
    'BBIjCh9QQVlNRU5UX0FDQ09VTlRfU1RBVFVTX0RJU0FCTEVEEAU=');

@$core.Deprecated('Use requirementCollectionDescriptor instead')
const RequirementCollection$json = {
  '1': 'RequirementCollection',
  '2': [
    {'1': 'REQUIREMENT_COLLECTION_UNSPECIFIED', '2': 0},
    {'1': 'REQUIREMENT_COLLECTION_PROVIDER', '2': 1},
    {'1': 'REQUIREMENT_COLLECTION_PLATFORM', '2': 2},
  ],
};

/// Descriptor for `RequirementCollection`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List requirementCollectionDescriptor = $convert.base64Decode(
    'ChVSZXF1aXJlbWVudENvbGxlY3Rpb24SJgoiUkVRVUlSRU1FTlRfQ09MTEVDVElPTl9VTlNQRU'
    'NJRklFRBAAEiMKH1JFUVVJUkVNRU5UX0NPTExFQ1RJT05fUFJPVklERVIQARIjCh9SRVFVSVJF'
    'TUVOVF9DT0xMRUNUSU9OX1BMQVRGT1JNEAI=');

@$core.Deprecated('Use onboardingSurfaceDescriptor instead')
const OnboardingSurface$json = {
  '1': 'OnboardingSurface',
  '2': [
    {'1': 'ONBOARDING_SURFACE_UNSPECIFIED', '2': 0},
    {'1': 'ONBOARDING_SURFACE_HOSTED', '2': 1},
    {'1': 'ONBOARDING_SURFACE_EMBEDDED', '2': 2},
  ],
};

/// Descriptor for `OnboardingSurface`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List onboardingSurfaceDescriptor = $convert.base64Decode(
    'ChFPbmJvYXJkaW5nU3VyZmFjZRIiCh5PTkJPQVJESU5HX1NVUkZBQ0VfVU5TUEVDSUZJRUQQAB'
    'IdChlPTkJPQVJESU5HX1NVUkZBQ0VfSE9TVEVEEAESHwobT05CT0FSRElOR19TVVJGQUNFX0VN'
    'QkVEREVEEAI=');
