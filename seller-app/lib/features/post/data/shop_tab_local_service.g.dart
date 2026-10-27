// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'shop_tab_local_service.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(shopTabLocalService)
final shopTabLocalServiceProvider = ShopTabLocalServiceProvider._();

final class ShopTabLocalServiceProvider
    extends
        $FunctionalProvider<
          ShopTabLocalService,
          ShopTabLocalService,
          ShopTabLocalService
        >
    with $Provider<ShopTabLocalService> {
  ShopTabLocalServiceProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'shopTabLocalServiceProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$shopTabLocalServiceHash();

  @$internal
  @override
  $ProviderElement<ShopTabLocalService> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  ShopTabLocalService create(Ref ref) {
    return shopTabLocalService(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ShopTabLocalService value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ShopTabLocalService>(value),
    );
  }
}

String _$shopTabLocalServiceHash() =>
    r'c7b0ff9caaffd25b18f1341ffbcf0ba2d7f0de0e';
