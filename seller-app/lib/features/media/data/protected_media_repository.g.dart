// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'protected_media_repository.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(protectedMediaRepository)
final protectedMediaRepositoryProvider = ProtectedMediaRepositoryProvider._();

final class ProtectedMediaRepositoryProvider
    extends
        $FunctionalProvider<
          ProtectedMediaRepository,
          ProtectedMediaRepository,
          ProtectedMediaRepository
        >
    with $Provider<ProtectedMediaRepository> {
  ProtectedMediaRepositoryProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'protectedMediaRepositoryProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$protectedMediaRepositoryHash();

  @$internal
  @override
  $ProviderElement<ProtectedMediaRepository> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  ProtectedMediaRepository create(Ref ref) {
    return protectedMediaRepository(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ProtectedMediaRepository value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ProtectedMediaRepository>(value),
    );
  }
}

String _$protectedMediaRepositoryHash() =>
    r'eb4cb5cc006ac06f5763c34cf52c1adb5f636989';
