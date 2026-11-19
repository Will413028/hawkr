// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'blocking_repository.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(blockingRepository)
final blockingRepositoryProvider = BlockingRepositoryProvider._();

final class BlockingRepositoryProvider
    extends
        $FunctionalProvider<
          BlockingRepository,
          BlockingRepository,
          BlockingRepository
        >
    with $Provider<BlockingRepository> {
  BlockingRepositoryProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'blockingRepositoryProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$blockingRepositoryHash();

  @$internal
  @override
  $ProviderElement<BlockingRepository> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  BlockingRepository create(Ref ref) {
    return blockingRepository(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(BlockingRepository value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<BlockingRepository>(value),
    );
  }
}

String _$blockingRepositoryHash() =>
    r'1c007af5b005b168d92d6aac3ca72f4acfde3990';
