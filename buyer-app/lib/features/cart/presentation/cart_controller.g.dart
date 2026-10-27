// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'cart_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// Badge and cart screen read the exact same controller snapshot.

@ProviderFor(cartCount)
final cartCountProvider = CartCountProvider._();

/// Badge and cart screen read the exact same controller snapshot.

final class CartCountProvider
    extends $FunctionalProvider<AsyncValue<int>, int, FutureOr<int>>
    with $FutureModifier<int>, $FutureProvider<int> {
  /// Badge and cart screen read the exact same controller snapshot.
  CartCountProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'cartCountProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$cartCountHash();

  @$internal
  @override
  $FutureProviderElement<int> $createElement($ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<int> create(Ref ref) {
    return cartCount(ref);
  }
}

String _$cartCountHash() => r'fb5a4f04358b9764a01067621630f6e64253af47';

@ProviderFor(CartController)
final cartControllerProvider = CartControllerProvider._();

final class CartControllerProvider
    extends $AsyncNotifierProvider<CartController, CartState> {
  CartControllerProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'cartControllerProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$cartControllerHash();

  @$internal
  @override
  CartController create() => CartController();
}

String _$cartControllerHash() => r'14c93c96dedecb6f623c1a57c1ffbe9b6a9eba2d';

abstract class _$CartController extends $AsyncNotifier<CartState> {
  FutureOr<CartState> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AsyncValue<CartState>, CartState>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<AsyncValue<CartState>, CartState>,
              AsyncValue<CartState>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
