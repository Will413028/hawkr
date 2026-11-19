// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'social_content_repository.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(socialContentRepository)
final socialContentRepositoryProvider = SocialContentRepositoryProvider._();

final class SocialContentRepositoryProvider
    extends
        $FunctionalProvider<
          SocialContentRepository,
          SocialContentRepository,
          SocialContentRepository
        >
    with $Provider<SocialContentRepository> {
  SocialContentRepositoryProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'socialContentRepositoryProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$socialContentRepositoryHash();

  @$internal
  @override
  $ProviderElement<SocialContentRepository> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  SocialContentRepository create(Ref ref) {
    return socialContentRepository(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(SocialContentRepository value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<SocialContentRepository>(value),
    );
  }
}

String _$socialContentRepositoryHash() =>
    r'3a6b238f294f22bc007dece2d2529e0786a8adb6';
