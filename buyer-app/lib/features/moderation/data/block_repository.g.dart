// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'block_repository.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(blockRepository)
final blockRepositoryProvider = BlockRepositoryProvider._();

final class BlockRepositoryProvider
    extends
        $FunctionalProvider<BlockRepository, BlockRepository, BlockRepository>
    with $Provider<BlockRepository> {
  BlockRepositoryProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'blockRepositoryProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$blockRepositoryHash();

  @$internal
  @override
  $ProviderElement<BlockRepository> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  BlockRepository create(Ref ref) {
    return blockRepository(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(BlockRepository value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<BlockRepository>(value),
    );
  }
}

String _$blockRepositoryHash() => r'5325d12880855a7b7d7d6672a30f0d5fd44a7ce5';
