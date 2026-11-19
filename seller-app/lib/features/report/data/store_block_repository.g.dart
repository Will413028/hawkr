// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'store_block_repository.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(storeBlockRepository)
final storeBlockRepositoryProvider = StoreBlockRepositoryProvider._();

final class StoreBlockRepositoryProvider
    extends
        $FunctionalProvider<
          StoreBlockRepository,
          StoreBlockRepository,
          StoreBlockRepository
        >
    with $Provider<StoreBlockRepository> {
  StoreBlockRepositoryProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'storeBlockRepositoryProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$storeBlockRepositoryHash();

  @$internal
  @override
  $ProviderElement<StoreBlockRepository> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  StoreBlockRepository create(Ref ref) {
    return storeBlockRepository(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(StoreBlockRepository value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<StoreBlockRepository>(value),
    );
  }
}

String _$storeBlockRepositoryHash() =>
    r'3210183e38ea24d6d571f3e1e98073bcf6de6d2d';
