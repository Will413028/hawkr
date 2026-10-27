// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'ai_edit_history_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(AiEditHistoryController)
final aiEditHistoryControllerProvider = AiEditHistoryControllerProvider._();

final class AiEditHistoryControllerProvider
    extends
        $AsyncNotifierProvider<
          AiEditHistoryController,
          List<AiEditHistoryEntry>
        > {
  AiEditHistoryControllerProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'aiEditHistoryControllerProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$aiEditHistoryControllerHash();

  @$internal
  @override
  AiEditHistoryController create() => AiEditHistoryController();
}

String _$aiEditHistoryControllerHash() =>
    r'7d25cbf4c66f10c31334f5729a06badc444d836c';

abstract class _$AiEditHistoryController
    extends $AsyncNotifier<List<AiEditHistoryEntry>> {
  FutureOr<List<AiEditHistoryEntry>> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref =
        this.ref
            as $Ref<
              AsyncValue<List<AiEditHistoryEntry>>,
              List<AiEditHistoryEntry>
            >;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<
                AsyncValue<List<AiEditHistoryEntry>>,
                List<AiEditHistoryEntry>
              >,
              AsyncValue<List<AiEditHistoryEntry>>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
