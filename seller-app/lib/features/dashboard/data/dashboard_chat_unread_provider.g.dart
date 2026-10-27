// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'dashboard_chat_unread_provider.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 儀表板聊天未讀數 Provider
///
/// 呼叫 ChatRepository.getUnreadCount() 取得未讀訊息數量。
/// 由 DashboardScreen 在 FCM 事件觸發時 invalidate。

@ProviderFor(dashboardChatUnread)
final dashboardChatUnreadProvider = DashboardChatUnreadProvider._();

/// 儀表板聊天未讀數 Provider
///
/// 呼叫 ChatRepository.getUnreadCount() 取得未讀訊息數量。
/// 由 DashboardScreen 在 FCM 事件觸發時 invalidate。

final class DashboardChatUnreadProvider
    extends $FunctionalProvider<AsyncValue<int>, int, FutureOr<int>>
    with $FutureModifier<int>, $FutureProvider<int> {
  /// 儀表板聊天未讀數 Provider
  ///
  /// 呼叫 ChatRepository.getUnreadCount() 取得未讀訊息數量。
  /// 由 DashboardScreen 在 FCM 事件觸發時 invalidate。
  DashboardChatUnreadProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'dashboardChatUnreadProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$dashboardChatUnreadHash();

  @$internal
  @override
  $FutureProviderElement<int> $createElement($ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<int> create(Ref ref) {
    return dashboardChatUnread(ref);
  }
}

String _$dashboardChatUnreadHash() =>
    r'bc0ffa749209a7db30c959c6358af4a2f6f961b2';
