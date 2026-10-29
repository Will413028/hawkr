// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'social_repository.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(socialRepository)
final socialRepositoryProvider = SocialRepositoryProvider._();

final class SocialRepositoryProvider
    extends
        $FunctionalProvider<
          SocialRepository,
          SocialRepository,
          SocialRepository
        >
    with $Provider<SocialRepository> {
  SocialRepositoryProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'socialRepositoryProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$socialRepositoryHash();

  @$internal
  @override
  $ProviderElement<SocialRepository> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  SocialRepository create(Ref ref) {
    return socialRepository(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(SocialRepository value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<SocialRepository>(value),
    );
  }
}

String _$socialRepositoryHash() => r'4daa930c3547a7933bb1af36c5937c57f8e8a582';
