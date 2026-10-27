// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'follow_repository.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(followRepository)
final followRepositoryProvider = FollowRepositoryProvider._();

final class FollowRepositoryProvider
    extends
        $FunctionalProvider<
          FollowRepository,
          FollowRepository,
          FollowRepository
        >
    with $Provider<FollowRepository> {
  FollowRepositoryProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'followRepositoryProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$followRepositoryHash();

  @$internal
  @override
  $ProviderElement<FollowRepository> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  FollowRepository create(Ref ref) {
    return followRepository(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(FollowRepository value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<FollowRepository>(value),
    );
  }
}

String _$followRepositoryHash() => r'476a3b280ff5ff312af34273d566d87e61235ef2';
