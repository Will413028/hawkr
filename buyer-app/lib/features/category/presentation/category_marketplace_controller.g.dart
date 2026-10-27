// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'category_marketplace_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// Session-scoped UI state for the category landing recommendation rail.
///
/// Retry is user-driven so a failed section never starts hidden repeat
/// requests and the request-count contract stays deterministic.

@ProviderFor(CategoryMarketplaceController)
final categoryMarketplaceControllerProvider =
    CategoryMarketplaceControllerProvider._();

/// Session-scoped UI state for the category landing recommendation rail.
///
/// Retry is user-driven so a failed section never starts hidden repeat
/// requests and the request-count contract stays deterministic.
final class CategoryMarketplaceControllerProvider
    extends
        $AsyncNotifierProvider<
          CategoryMarketplaceController,
          List<CategoryMarketplaceProduct>
        > {
  /// Session-scoped UI state for the category landing recommendation rail.
  ///
  /// Retry is user-driven so a failed section never starts hidden repeat
  /// requests and the request-count contract stays deterministic.
  CategoryMarketplaceControllerProvider._()
    : super(
        from: null,
        argument: null,
        retry: _disableAutomaticRetry,
        name: r'categoryMarketplaceControllerProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$categoryMarketplaceControllerHash();

  @$internal
  @override
  CategoryMarketplaceController create() => CategoryMarketplaceController();
}

String _$categoryMarketplaceControllerHash() =>
    r'df60e7d498680a6dccd7b620eb5135325eedec48';

/// Session-scoped UI state for the category landing recommendation rail.
///
/// Retry is user-driven so a failed section never starts hidden repeat
/// requests and the request-count contract stays deterministic.

abstract class _$CategoryMarketplaceController
    extends $AsyncNotifier<List<CategoryMarketplaceProduct>> {
  FutureOr<List<CategoryMarketplaceProduct>> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref =
        this.ref
            as $Ref<
              AsyncValue<List<CategoryMarketplaceProduct>>,
              List<CategoryMarketplaceProduct>
            >;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<
                AsyncValue<List<CategoryMarketplaceProduct>>,
                List<CategoryMarketplaceProduct>
              >,
              AsyncValue<List<CategoryMarketplaceProduct>>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
