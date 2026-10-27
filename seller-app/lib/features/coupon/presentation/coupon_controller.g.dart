// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'coupon_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// Controller for the merchant coupon list.
///
/// Provides CRUD operations and keeps the local list in sync.

@ProviderFor(CouponListController)
final couponListControllerProvider = CouponListControllerProvider._();

/// Controller for the merchant coupon list.
///
/// Provides CRUD operations and keeps the local list in sync.
final class CouponListControllerProvider
    extends $AsyncNotifierProvider<CouponListController, CouponListState> {
  /// Controller for the merchant coupon list.
  ///
  /// Provides CRUD operations and keeps the local list in sync.
  CouponListControllerProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'couponListControllerProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$couponListControllerHash();

  @$internal
  @override
  CouponListController create() => CouponListController();
}

String _$couponListControllerHash() =>
    r'4575f00a4f8226278404c42671e1350643de5897';

/// Controller for the merchant coupon list.
///
/// Provides CRUD operations and keeps the local list in sync.

abstract class _$CouponListController extends $AsyncNotifier<CouponListState> {
  FutureOr<CouponListState> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AsyncValue<CouponListState>, CouponListState>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<AsyncValue<CouponListState>, CouponListState>,
              AsyncValue<CouponListState>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
