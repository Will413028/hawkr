// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'store_editor.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(storeEditor)
final storeEditorProvider = StoreEditorProvider._();

final class StoreEditorProvider
    extends $FunctionalProvider<StoreEditor, StoreEditor, StoreEditor>
    with $Provider<StoreEditor> {
  StoreEditorProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'storeEditorProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$storeEditorHash();

  @$internal
  @override
  $ProviderElement<StoreEditor> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  StoreEditor create(Ref ref) {
    return storeEditor(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(StoreEditor value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<StoreEditor>(value),
    );
  }
}

String _$storeEditorHash() => r'08ab4edd45cbce5f45cb8d357d6ed97f0e424942';
