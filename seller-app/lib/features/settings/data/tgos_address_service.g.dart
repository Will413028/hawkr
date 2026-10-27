// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'tgos_address_service.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(tgosAddressService)
final tgosAddressServiceProvider = TgosAddressServiceProvider._();

final class TgosAddressServiceProvider
    extends
        $FunctionalProvider<
          TgosAddressService,
          TgosAddressService,
          TgosAddressService
        >
    with $Provider<TgosAddressService> {
  TgosAddressServiceProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'tgosAddressServiceProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$tgosAddressServiceHash();

  @$internal
  @override
  $ProviderElement<TgosAddressService> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  TgosAddressService create(Ref ref) {
    return tgosAddressService(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(TgosAddressService value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<TgosAddressService>(value),
    );
  }
}

String _$tgosAddressServiceHash() =>
    r'751925e6d1d505032b80433d6e4c8214a0e8a43e';
