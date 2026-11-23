// This is a generated file - do not edit.
//
// Generated from hawkr/notification/v1alpha1/notification.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

import 'notification.pb.dart' as $2;
import 'notification.pbjson.dart';

export 'notification.pb.dart';

abstract class NotificationAppServiceBase extends $pb.GeneratedService {
  $async.Future<$2.ListInboxItemsResponse> listInboxItems(
      $pb.ServerContext ctx, $2.ListInboxItemsRequest request);
  $async.Future<$2.GetUnreadCountResponse> getUnreadCount(
      $pb.ServerContext ctx, $2.GetUnreadCountRequest request);
  $async.Future<$2.MarkInboxItemReadResponse> markInboxItemRead(
      $pb.ServerContext ctx, $2.MarkInboxItemReadRequest request);
  $async.Future<$2.MarkAllInboxItemsReadResponse> markAllInboxItemsRead(
      $pb.ServerContext ctx, $2.MarkAllInboxItemsReadRequest request);
  $async.Future<$2.BatchDeleteInboxItemsResponse> batchDeleteInboxItems(
      $pb.ServerContext ctx, $2.BatchDeleteInboxItemsRequest request);
  $async.Future<$2.GetNotificationPreferencesResponse>
      getNotificationPreferences(
          $pb.ServerContext ctx, $2.GetNotificationPreferencesRequest request);
  $async.Future<$2.UpdateNotificationPreferenceResponse>
      updateNotificationPreference($pb.ServerContext ctx,
          $2.UpdateNotificationPreferenceRequest request);
  $async.Future<$2.RegisterDeviceTokenResponse> registerDeviceToken(
      $pb.ServerContext ctx, $2.RegisterDeviceTokenRequest request);
  $async.Future<$2.DeleteDeviceTokenResponse> deleteDeviceToken(
      $pb.ServerContext ctx, $2.DeleteDeviceTokenRequest request);
  $async.Future<$2.WatchNotificationsResponse> watchNotifications(
      $pb.ServerContext ctx, $2.WatchNotificationsRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'ListInboxItems':
        return $2.ListInboxItemsRequest();
      case 'GetUnreadCount':
        return $2.GetUnreadCountRequest();
      case 'MarkInboxItemRead':
        return $2.MarkInboxItemReadRequest();
      case 'MarkAllInboxItemsRead':
        return $2.MarkAllInboxItemsReadRequest();
      case 'BatchDeleteInboxItems':
        return $2.BatchDeleteInboxItemsRequest();
      case 'GetNotificationPreferences':
        return $2.GetNotificationPreferencesRequest();
      case 'UpdateNotificationPreference':
        return $2.UpdateNotificationPreferenceRequest();
      case 'RegisterDeviceToken':
        return $2.RegisterDeviceTokenRequest();
      case 'DeleteDeviceToken':
        return $2.DeleteDeviceTokenRequest();
      case 'WatchNotifications':
        return $2.WatchNotificationsRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'ListInboxItems':
        return listInboxItems(ctx, request as $2.ListInboxItemsRequest);
      case 'GetUnreadCount':
        return getUnreadCount(ctx, request as $2.GetUnreadCountRequest);
      case 'MarkInboxItemRead':
        return markInboxItemRead(ctx, request as $2.MarkInboxItemReadRequest);
      case 'MarkAllInboxItemsRead':
        return markAllInboxItemsRead(
            ctx, request as $2.MarkAllInboxItemsReadRequest);
      case 'BatchDeleteInboxItems':
        return batchDeleteInboxItems(
            ctx, request as $2.BatchDeleteInboxItemsRequest);
      case 'GetNotificationPreferences':
        return getNotificationPreferences(
            ctx, request as $2.GetNotificationPreferencesRequest);
      case 'UpdateNotificationPreference':
        return updateNotificationPreference(
            ctx, request as $2.UpdateNotificationPreferenceRequest);
      case 'RegisterDeviceToken':
        return registerDeviceToken(
            ctx, request as $2.RegisterDeviceTokenRequest);
      case 'DeleteDeviceToken':
        return deleteDeviceToken(ctx, request as $2.DeleteDeviceTokenRequest);
      case 'WatchNotifications':
        return watchNotifications(ctx, request as $2.WatchNotificationsRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json =>
      NotificationAppServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => NotificationAppServiceBase$messageJson;
}
