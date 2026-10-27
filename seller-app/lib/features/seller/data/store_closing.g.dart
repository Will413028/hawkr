// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'store_closing.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(storeClosing)
final storeClosingProvider = StoreClosingProvider._();

final class StoreClosingProvider
    extends $FunctionalProvider<StoreClosing, StoreClosing, StoreClosing>
    with $Provider<StoreClosing> {
  StoreClosingProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'storeClosingProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$storeClosingHash();

  @$internal
  @override
  $ProviderElement<StoreClosing> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  StoreClosing create(Ref ref) {
    return storeClosing(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(StoreClosing value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<StoreClosing>(value),
    );
  }
}

String _$storeClosingHash() => r'50c0e7f47385e791709a35633148530c799b60ac';
