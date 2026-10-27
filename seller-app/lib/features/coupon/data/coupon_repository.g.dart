// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'coupon_repository.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(couponRepository)
final couponRepositoryProvider = CouponRepositoryProvider._();

final class CouponRepositoryProvider
    extends
        $FunctionalProvider<
          CouponRepository,
          CouponRepository,
          CouponRepository
        >
    with $Provider<CouponRepository> {
  CouponRepositoryProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'couponRepositoryProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$couponRepositoryHash();

  @$internal
  @override
  $ProviderElement<CouponRepository> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  CouponRepository create(Ref ref) {
    return couponRepository(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(CouponRepository value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<CouponRepository>(value),
    );
  }
}

String _$couponRepositoryHash() => r'962c3b65c58d7bb979168d9ca19b33635ea4ee24';
