// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'shipping_settings_repository.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(shippingSettingsRepository)
final shippingSettingsRepositoryProvider =
    ShippingSettingsRepositoryProvider._();

final class ShippingSettingsRepositoryProvider
    extends
        $FunctionalProvider<
          ShippingSettingsRepository,
          ShippingSettingsRepository,
          ShippingSettingsRepository
        >
    with $Provider<ShippingSettingsRepository> {
  ShippingSettingsRepositoryProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'shippingSettingsRepositoryProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$shippingSettingsRepositoryHash();

  @$internal
  @override
  $ProviderElement<ShippingSettingsRepository> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  ShippingSettingsRepository create(Ref ref) {
    return shippingSettingsRepository(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ShippingSettingsRepository value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ShippingSettingsRepository>(value),
    );
  }
}

String _$shippingSettingsRepositoryHash() =>
    r'84ab01890b06404efeb6534eb7698957e6b98e06';
