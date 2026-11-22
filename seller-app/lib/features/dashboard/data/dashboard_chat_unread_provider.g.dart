// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'dashboard_chat_unread_provider.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 儀表板聊天未讀數：`GetStoreUnreadCount`。讀取失敗時丟出錯誤（儀表板的徽章把錯誤當成沒有數字，不當成 0）。

@ProviderFor(dashboardChatUnread)
final dashboardChatUnreadProvider = DashboardChatUnreadProvider._();

/// 儀表板聊天未讀數：`GetStoreUnreadCount`。讀取失敗時丟出錯誤（儀表板的徽章把錯誤當成沒有數字，不當成 0）。

final class DashboardChatUnreadProvider
    extends $FunctionalProvider<AsyncValue<int>, int, FutureOr<int>>
    with $FutureModifier<int>, $FutureProvider<int> {
  /// 儀表板聊天未讀數：`GetStoreUnreadCount`。讀取失敗時丟出錯誤（儀表板的徽章把錯誤當成沒有數字，不當成 0）。
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
    r'c11d2ea819803d404a5e42789859dd4246ea3cce';
