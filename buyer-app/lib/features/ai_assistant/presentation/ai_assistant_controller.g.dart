// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'ai_assistant_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(AiAssistantController)
final aiAssistantControllerProvider = AiAssistantControllerProvider._();

final class AiAssistantControllerProvider
    extends $NotifierProvider<AiAssistantController, List<AiMessage>> {
  AiAssistantControllerProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'aiAssistantControllerProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$aiAssistantControllerHash();

  @$internal
  @override
  AiAssistantController create() => AiAssistantController();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(List<AiMessage> value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<List<AiMessage>>(value),
    );
  }
}

String _$aiAssistantControllerHash() =>
    r'507c25793920f005716f5212fd6ba501dcc13f6e';

abstract class _$AiAssistantController extends $Notifier<List<AiMessage>> {
  List<AiMessage> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<List<AiMessage>, List<AiMessage>>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<List<AiMessage>, List<AiMessage>>,
              List<AiMessage>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
