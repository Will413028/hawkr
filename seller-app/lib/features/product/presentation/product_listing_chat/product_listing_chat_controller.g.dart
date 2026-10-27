// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'product_listing_chat_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 對話式上架助手的狀態機：
/// - sendMessage：append user 訊息 → 建立空的 assistant 訊息 → 串流 SSE
/// - 收到 text event：把 delta 串接到最後一則 assistant 訊息
/// - 收到 draft_patch event：把 patch merge 到 [productListingDraftControllerProvider]
/// - 收到 done event：存下 sessionId，結束串流

@ProviderFor(ProductListingChatController)
final productListingChatControllerProvider =
    ProductListingChatControllerProvider._();

/// 對話式上架助手的狀態機：
/// - sendMessage：append user 訊息 → 建立空的 assistant 訊息 → 串流 SSE
/// - 收到 text event：把 delta 串接到最後一則 assistant 訊息
/// - 收到 draft_patch event：把 patch merge 到 [productListingDraftControllerProvider]
/// - 收到 done event：存下 sessionId，結束串流
final class ProductListingChatControllerProvider
    extends $NotifierProvider<ProductListingChatController, ChatState> {
  /// 對話式上架助手的狀態機：
  /// - sendMessage：append user 訊息 → 建立空的 assistant 訊息 → 串流 SSE
  /// - 收到 text event：把 delta 串接到最後一則 assistant 訊息
  /// - 收到 draft_patch event：把 patch merge 到 [productListingDraftControllerProvider]
  /// - 收到 done event：存下 sessionId，結束串流
  ProductListingChatControllerProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'productListingChatControllerProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$productListingChatControllerHash();

  @$internal
  @override
  ProductListingChatController create() => ProductListingChatController();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ChatState value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ChatState>(value),
    );
  }
}

String _$productListingChatControllerHash() =>
    r'189d1f419b0595895a7f7e9fe0f3b13bd9858dfc';

/// 對話式上架助手的狀態機：
/// - sendMessage：append user 訊息 → 建立空的 assistant 訊息 → 串流 SSE
/// - 收到 text event：把 delta 串接到最後一則 assistant 訊息
/// - 收到 draft_patch event：把 patch merge 到 [productListingDraftControllerProvider]
/// - 收到 done event：存下 sessionId，結束串流

abstract class _$ProductListingChatController extends $Notifier<ChatState> {
  ChatState build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<ChatState, ChatState>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<ChatState, ChatState>,
              ChatState,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
