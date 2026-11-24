// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'dashboard_notification_unread_provider.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 儀表板通知未讀數 Provider
///
/// 呼叫 NotificationRepository.unreadCount() 取得未讀通知數量。
/// 僅用於 Dashboard 鈴鐺紅點顯示，由下拉刷新時 invalidate。

@ProviderFor(dashboardNotificationUnread)
final dashboardNotificationUnreadProvider =
    DashboardNotificationUnreadProvider._();

/// 儀表板通知未讀數 Provider
///
/// 呼叫 NotificationRepository.unreadCount() 取得未讀通知數量。
/// 僅用於 Dashboard 鈴鐺紅點顯示，由下拉刷新時 invalidate。

final class DashboardNotificationUnreadProvider
    extends $FunctionalProvider<AsyncValue<int>, int, FutureOr<int>>
    with $FutureModifier<int>, $FutureProvider<int> {
  /// 儀表板通知未讀數 Provider
  ///
  /// 呼叫 NotificationRepository.unreadCount() 取得未讀通知數量。
  /// 僅用於 Dashboard 鈴鐺紅點顯示，由下拉刷新時 invalidate。
  DashboardNotificationUnreadProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'dashboardNotificationUnreadProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$dashboardNotificationUnreadHash();

  @$internal
  @override
  $FutureProviderElement<int> $createElement($ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<int> create(Ref ref) {
    return dashboardNotificationUnread(ref);
  }
}

String _$dashboardNotificationUnreadHash() =>
    r'fb8bc9e55e9ea76d014446bb88bd24b343ea80cf';
