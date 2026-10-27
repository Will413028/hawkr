// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'ai_edit_history_local_service.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(aiEditHistoryLocalService)
final aiEditHistoryLocalServiceProvider = AiEditHistoryLocalServiceProvider._();

final class AiEditHistoryLocalServiceProvider
    extends
        $FunctionalProvider<
          AiEditHistoryLocalService,
          AiEditHistoryLocalService,
          AiEditHistoryLocalService
        >
    with $Provider<AiEditHistoryLocalService> {
  AiEditHistoryLocalServiceProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'aiEditHistoryLocalServiceProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$aiEditHistoryLocalServiceHash();

  @$internal
  @override
  $ProviderElement<AiEditHistoryLocalService> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  AiEditHistoryLocalService create(Ref ref) {
    return aiEditHistoryLocalService(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(AiEditHistoryLocalService value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<AiEditHistoryLocalService>(value),
    );
  }
}

String _$aiEditHistoryLocalServiceHash() =>
    r'1b2ef47d970420b1ccc38029c7bca1866b40f4db';
