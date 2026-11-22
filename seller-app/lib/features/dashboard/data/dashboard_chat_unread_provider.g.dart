// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'dashboard_chat_unread_provider.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 儀表板聊天未讀數：`GetStoreUnreadCount`。讀取失敗時丟出錯誤（儀表板的徽章把錯誤當成沒有數字，不當成 0）。
/// 事件串流的 Ready 與 UnreadChanged 帶來新的總數時重讀，徽章不必等使用者重新整理。

@ProviderFor(dashboardChatUnread)
final dashboardChatUnreadProvider = DashboardChatUnreadProvider._();

/// 儀表板聊天未讀數：`GetStoreUnreadCount`。讀取失敗時丟出錯誤（儀表板的徽章把錯誤當成沒有數字，不當成 0）。
/// 事件串流的 Ready 與 UnreadChanged 帶來新的總數時重讀，徽章不必等使用者重新整理。

final class DashboardChatUnreadProvider
    extends $FunctionalProvider<AsyncValue<int>, int, FutureOr<int>>
    with $FutureModifier<int>, $FutureProvider<int> {
  /// 儀表板聊天未讀數：`GetStoreUnreadCount`。讀取失敗時丟出錯誤（儀表板的徽章把錯誤當成沒有數字，不當成 0）。
  /// 事件串流的 Ready 與 UnreadChanged 帶來新的總數時重讀，徽章不必等使用者重新整理。
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
    r'774a44731621183a3368f34fd9385a1d77526988';
