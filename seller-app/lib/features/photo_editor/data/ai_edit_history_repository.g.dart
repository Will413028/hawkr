// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'ai_edit_history_repository.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(aiEditHistoryRepository)
final aiEditHistoryRepositoryProvider = AiEditHistoryRepositoryProvider._();

final class AiEditHistoryRepositoryProvider
    extends
        $FunctionalProvider<
          AiEditHistoryRepository,
          AiEditHistoryRepository,
          AiEditHistoryRepository
        >
    with $Provider<AiEditHistoryRepository> {
  AiEditHistoryRepositoryProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'aiEditHistoryRepositoryProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$aiEditHistoryRepositoryHash();

  @$internal
  @override
  $ProviderElement<AiEditHistoryRepository> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  AiEditHistoryRepository create(Ref ref) {
    return aiEditHistoryRepository(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(AiEditHistoryRepository value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<AiEditHistoryRepository>(value),
    );
  }
}

String _$aiEditHistoryRepositoryHash() =>
    r'ea371df3bcdcb0919b9a75208d35c120508e71cd';
