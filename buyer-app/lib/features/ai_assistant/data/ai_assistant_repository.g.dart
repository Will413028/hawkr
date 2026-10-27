// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'ai_assistant_repository.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(aiAssistantRepository)
final aiAssistantRepositoryProvider = AiAssistantRepositoryProvider._();

final class AiAssistantRepositoryProvider
    extends
        $FunctionalProvider<
          AiAssistantRepository,
          AiAssistantRepository,
          AiAssistantRepository
        >
    with $Provider<AiAssistantRepository> {
  AiAssistantRepositoryProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'aiAssistantRepositoryProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$aiAssistantRepositoryHash();

  @$internal
  @override
  $ProviderElement<AiAssistantRepository> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  AiAssistantRepository create(Ref ref) {
    return aiAssistantRepository(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(AiAssistantRepository value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<AiAssistantRepository>(value),
    );
  }
}

String _$aiAssistantRepositoryHash() =>
    r'2f34474c6e53b885f6ea23e975f78ee53ee215bf';
