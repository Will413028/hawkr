// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'seller_repository.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(sellerRepository)
final sellerRepositoryProvider = SellerRepositoryProvider._();

final class SellerRepositoryProvider
    extends
        $FunctionalProvider<
          SellerRepository,
          SellerRepository,
          SellerRepository
        >
    with $Provider<SellerRepository> {
  SellerRepositoryProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'sellerRepositoryProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$sellerRepositoryHash();

  @$internal
  @override
  $ProviderElement<SellerRepository> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  SellerRepository create(Ref ref) {
    return sellerRepository(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(SellerRepository value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<SellerRepository>(value),
    );
  }
}

String _$sellerRepositoryHash() => r'b3a010a40a02d3e21c6e17a1d2889758868839a4';
