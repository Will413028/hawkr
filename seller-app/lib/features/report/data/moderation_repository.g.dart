// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'moderation_repository.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(moderationRepository)
final moderationRepositoryProvider = ModerationRepositoryProvider._();

final class ModerationRepositoryProvider
    extends
        $FunctionalProvider<
          ModerationRepository,
          ModerationRepository,
          ModerationRepository
        >
    with $Provider<ModerationRepository> {
  ModerationRepositoryProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'moderationRepositoryProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$moderationRepositoryHash();

  @$internal
  @override
  $ProviderElement<ModerationRepository> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  ModerationRepository create(Ref ref) {
    return moderationRepository(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ModerationRepository value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ModerationRepository>(value),
    );
  }
}

String _$moderationRepositoryHash() =>
    r'81f3c0eb84b34c2775f7a006120e60eef6c27006';
