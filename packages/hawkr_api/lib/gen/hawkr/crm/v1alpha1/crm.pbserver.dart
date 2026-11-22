// This is a generated file - do not edit.
//
// Generated from hawkr/crm/v1alpha1/crm.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

import 'crm.pb.dart' as $3;
import 'crm.pbjson.dart';

export 'crm.pb.dart';

abstract class CrmAppServiceBase extends $pb.GeneratedService {
  $async.Future<$3.ListCustomersResponse> listCustomers(
      $pb.ServerContext ctx, $3.ListCustomersRequest request);
  $async.Future<$3.GetCustomerResponse> getCustomer(
      $pb.ServerContext ctx, $3.GetCustomerRequest request);
  $async.Future<$3.UpdateCustomerResponse> updateCustomer(
      $pb.ServerContext ctx, $3.UpdateCustomerRequest request);
  $async.Future<$3.CreateTagResponse> createTag(
      $pb.ServerContext ctx, $3.CreateTagRequest request);
  $async.Future<$3.ListTagsResponse> listTags(
      $pb.ServerContext ctx, $3.ListTagsRequest request);
  $async.Future<$3.UpdateTagResponse> updateTag(
      $pb.ServerContext ctx, $3.UpdateTagRequest request);
  $async.Future<$3.DeleteTagResponse> deleteTag(
      $pb.ServerContext ctx, $3.DeleteTagRequest request);
  $async.Future<$3.AddCustomerTagsResponse> addCustomerTags(
      $pb.ServerContext ctx, $3.AddCustomerTagsRequest request);
  $async.Future<$3.RemoveCustomerTagResponse> removeCustomerTag(
      $pb.ServerContext ctx, $3.RemoveCustomerTagRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'ListCustomers':
        return $3.ListCustomersRequest();
      case 'GetCustomer':
        return $3.GetCustomerRequest();
      case 'UpdateCustomer':
        return $3.UpdateCustomerRequest();
      case 'CreateTag':
        return $3.CreateTagRequest();
      case 'ListTags':
        return $3.ListTagsRequest();
      case 'UpdateTag':
        return $3.UpdateTagRequest();
      case 'DeleteTag':
        return $3.DeleteTagRequest();
      case 'AddCustomerTags':
        return $3.AddCustomerTagsRequest();
      case 'RemoveCustomerTag':
        return $3.RemoveCustomerTagRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'ListCustomers':
        return listCustomers(ctx, request as $3.ListCustomersRequest);
      case 'GetCustomer':
        return getCustomer(ctx, request as $3.GetCustomerRequest);
      case 'UpdateCustomer':
        return updateCustomer(ctx, request as $3.UpdateCustomerRequest);
      case 'CreateTag':
        return createTag(ctx, request as $3.CreateTagRequest);
      case 'ListTags':
        return listTags(ctx, request as $3.ListTagsRequest);
      case 'UpdateTag':
        return updateTag(ctx, request as $3.UpdateTagRequest);
      case 'DeleteTag':
        return deleteTag(ctx, request as $3.DeleteTagRequest);
      case 'AddCustomerTags':
        return addCustomerTags(ctx, request as $3.AddCustomerTagsRequest);
      case 'RemoveCustomerTag':
        return removeCustomerTag(ctx, request as $3.RemoveCustomerTagRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json => CrmAppServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => CrmAppServiceBase$messageJson;
}
