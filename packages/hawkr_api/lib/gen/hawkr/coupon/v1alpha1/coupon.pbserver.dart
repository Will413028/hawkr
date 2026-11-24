// This is a generated file - do not edit.
//
// Generated from hawkr/coupon/v1alpha1/coupon.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

import 'coupon.pb.dart' as $3;
import 'coupon.pbjson.dart';

export 'coupon.pb.dart';

abstract class CouponAppServiceBase extends $pb.GeneratedService {
  $async.Future<$3.ListMyVouchersResponse> listMyVouchers(
      $pb.ServerContext ctx, $3.ListMyVouchersRequest request);
  $async.Future<$3.ListClaimableCampaignsResponse> listClaimableCampaigns(
      $pb.ServerContext ctx, $3.ListClaimableCampaignsRequest request);
  $async.Future<$3.ClaimCouponResponse> claimCoupon(
      $pb.ServerContext ctx, $3.ClaimCouponRequest request);
  $async.Future<$3.CreateCampaignResponse> createCampaign(
      $pb.ServerContext ctx, $3.CreateCampaignRequest request);
  $async.Future<$3.ListCampaignsResponse> listCampaigns(
      $pb.ServerContext ctx, $3.ListCampaignsRequest request);
  $async.Future<$3.GetCampaignResponse> getCampaign(
      $pb.ServerContext ctx, $3.GetCampaignRequest request);
  $async.Future<$3.EndCampaignResponse> endCampaign(
      $pb.ServerContext ctx, $3.EndCampaignRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'ListMyVouchers':
        return $3.ListMyVouchersRequest();
      case 'ListClaimableCampaigns':
        return $3.ListClaimableCampaignsRequest();
      case 'ClaimCoupon':
        return $3.ClaimCouponRequest();
      case 'CreateCampaign':
        return $3.CreateCampaignRequest();
      case 'ListCampaigns':
        return $3.ListCampaignsRequest();
      case 'GetCampaign':
        return $3.GetCampaignRequest();
      case 'EndCampaign':
        return $3.EndCampaignRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'ListMyVouchers':
        return listMyVouchers(ctx, request as $3.ListMyVouchersRequest);
      case 'ListClaimableCampaigns':
        return listClaimableCampaigns(
            ctx, request as $3.ListClaimableCampaignsRequest);
      case 'ClaimCoupon':
        return claimCoupon(ctx, request as $3.ClaimCouponRequest);
      case 'CreateCampaign':
        return createCampaign(ctx, request as $3.CreateCampaignRequest);
      case 'ListCampaigns':
        return listCampaigns(ctx, request as $3.ListCampaignsRequest);
      case 'GetCampaign':
        return getCampaign(ctx, request as $3.GetCampaignRequest);
      case 'EndCampaign':
        return endCampaign(ctx, request as $3.EndCampaignRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json => CouponAppServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => CouponAppServiceBase$messageJson;
}
