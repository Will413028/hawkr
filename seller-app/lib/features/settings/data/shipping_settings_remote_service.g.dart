// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'shipping_settings_remote_service.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(shippingSettingsRemoteService)
final shippingSettingsRemoteServiceProvider =
    ShippingSettingsRemoteServiceProvider._();

final class ShippingSettingsRemoteServiceProvider
    extends
        $FunctionalProvider<
          ShippingSettingsRemoteService,
          ShippingSettingsRemoteService,
          ShippingSettingsRemoteService
        >
    with $Provider<ShippingSettingsRemoteService> {
  ShippingSettingsRemoteServiceProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'shippingSettingsRemoteServiceProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$shippingSettingsRemoteServiceHash();

  @$internal
  @override
  $ProviderElement<ShippingSettingsRemoteService> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  ShippingSettingsRemoteService create(Ref ref) {
    return shippingSettingsRemoteService(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ShippingSettingsRemoteService value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ShippingSettingsRemoteService>(
        value,
      ),
    );
  }
}

String _$shippingSettingsRemoteServiceHash() =>
    r'b0168acd41251bbadf5f681235a2cb5f46b00f24';
