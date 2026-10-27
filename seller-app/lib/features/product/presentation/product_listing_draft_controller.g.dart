// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'product_listing_draft_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// AI 對話助手累積的商品草稿。
///
/// Merge 策略：覆蓋單一欄位；tags / skus / certifications 收到新值時整體替換。
/// patchVersion 每次 merge 遞增，各 step screen 用它當 useEffect 的 dep。

@ProviderFor(ProductListingDraftController)
final productListingDraftControllerProvider =
    ProductListingDraftControllerProvider._();

/// AI 對話助手累積的商品草稿。
///
/// Merge 策略：覆蓋單一欄位；tags / skus / certifications 收到新值時整體替換。
/// patchVersion 每次 merge 遞增，各 step screen 用它當 useEffect 的 dep。
final class ProductListingDraftControllerProvider
    extends $NotifierProvider<ProductListingDraftController, PendingAiDraft> {
  /// AI 對話助手累積的商品草稿。
  ///
  /// Merge 策略：覆蓋單一欄位；tags / skus / certifications 收到新值時整體替換。
  /// patchVersion 每次 merge 遞增，各 step screen 用它當 useEffect 的 dep。
  ProductListingDraftControllerProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'productListingDraftControllerProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$productListingDraftControllerHash();

  @$internal
  @override
  ProductListingDraftController create() => ProductListingDraftController();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(PendingAiDraft value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<PendingAiDraft>(value),
    );
  }
}

String _$productListingDraftControllerHash() =>
    r'e2e5de4ec0c9b19173344eba43e6417911c3202c';

/// AI 對話助手累積的商品草稿。
///
/// Merge 策略：覆蓋單一欄位；tags / skus / certifications 收到新值時整體替換。
/// patchVersion 每次 merge 遞增，各 step screen 用它當 useEffect 的 dep。

abstract class _$ProductListingDraftController
    extends $Notifier<PendingAiDraft> {
  PendingAiDraft build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<PendingAiDraft, PendingAiDraft>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<PendingAiDraft, PendingAiDraft>,
              PendingAiDraft,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
