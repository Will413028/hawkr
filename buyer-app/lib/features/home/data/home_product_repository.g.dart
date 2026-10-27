// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'home_product_repository.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(homeProductRepository)
final homeProductRepositoryProvider = HomeProductRepositoryProvider._();

final class HomeProductRepositoryProvider
    extends
        $FunctionalProvider<
          HomeProductRepository,
          HomeProductRepository,
          HomeProductRepository
        >
    with $Provider<HomeProductRepository> {
  HomeProductRepositoryProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'homeProductRepositoryProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$homeProductRepositoryHash();

  @$internal
  @override
  $ProviderElement<HomeProductRepository> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  HomeProductRepository create(Ref ref) {
    return homeProductRepository(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(HomeProductRepository value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<HomeProductRepository>(value),
    );
  }
}

String _$homeProductRepositoryHash() =>
    r'0fedc081405767972aa01f48ab86576bc20be685';
