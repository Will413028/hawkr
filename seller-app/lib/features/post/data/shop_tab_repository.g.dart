// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'shop_tab_repository.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(shopTabRepository)
final shopTabRepositoryProvider = ShopTabRepositoryProvider._();

final class ShopTabRepositoryProvider
    extends
        $FunctionalProvider<
          ShopTabRepository,
          ShopTabRepository,
          ShopTabRepository
        >
    with $Provider<ShopTabRepository> {
  ShopTabRepositoryProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'shopTabRepositoryProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$shopTabRepositoryHash();

  @$internal
  @override
  $ProviderElement<ShopTabRepository> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  ShopTabRepository create(Ref ref) {
    return shopTabRepository(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ShopTabRepository value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ShopTabRepository>(value),
    );
  }
}

String _$shopTabRepositoryHash() => r'a9c827b6aa6f8f5772cc7424b572239d9cc0fd5c';
