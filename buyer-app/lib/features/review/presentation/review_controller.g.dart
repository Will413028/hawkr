// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'review_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(ProductReviewsController)
final productReviewsControllerProvider = ProductReviewsControllerFamily._();

final class ProductReviewsControllerProvider
    extends $AsyncNotifierProvider<ProductReviewsController, List<Review>> {
  ProductReviewsControllerProvider._({
    required ProductReviewsControllerFamily super.from,
    required int super.argument,
  }) : super(
         retry: null,
         name: r'productReviewsControllerProvider',
         isAutoDispose: false,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$productReviewsControllerHash();

  @override
  String toString() {
    return r'productReviewsControllerProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  ProductReviewsController create() => ProductReviewsController();

  @override
  bool operator ==(Object other) {
    return other is ProductReviewsControllerProvider &&
        other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$productReviewsControllerHash() =>
    r'9c058da7e695d4f1cf956eb4c877ec8010ef17ab';

final class ProductReviewsControllerFamily extends $Family
    with
        $ClassFamilyOverride<
          ProductReviewsController,
          AsyncValue<List<Review>>,
          List<Review>,
          FutureOr<List<Review>>,
          int
        > {
  ProductReviewsControllerFamily._()
    : super(
        retry: null,
        name: r'productReviewsControllerProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: false,
      );

  ProductReviewsControllerProvider call(int skuId) =>
      ProductReviewsControllerProvider._(argument: skuId, from: this);

  @override
  String toString() => r'productReviewsControllerProvider';
}

abstract class _$ProductReviewsController extends $AsyncNotifier<List<Review>> {
  late final _$args = ref.$arg as int;
  int get skuId => _$args;

  FutureOr<List<Review>> build(int skuId);
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AsyncValue<List<Review>>, List<Review>>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<AsyncValue<List<Review>>, List<Review>>,
              AsyncValue<List<Review>>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, () => build(_$args));
  }
}

@ProviderFor(AddReviewController)
final addReviewControllerProvider = AddReviewControllerProvider._();

final class AddReviewControllerProvider
    extends $NotifierProvider<AddReviewController, AsyncValue<void>> {
  AddReviewControllerProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'addReviewControllerProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$addReviewControllerHash();

  @$internal
  @override
  AddReviewController create() => AddReviewController();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(AsyncValue<void> value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<AsyncValue<void>>(value),
    );
  }
}

String _$addReviewControllerHash() =>
    r'6c617df4498eaea652ba2e40e417ea847e5da297';

abstract class _$AddReviewController extends $Notifier<AsyncValue<void>> {
  AsyncValue<void> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AsyncValue<void>, AsyncValue<void>>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<AsyncValue<void>, AsyncValue<void>>,
              AsyncValue<void>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
