// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'active_chat_gate.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(ActiveChatGate)
final activeChatGateProvider = ActiveChatGateProvider._();

final class ActiveChatGateProvider
    extends $NotifierProvider<ActiveChatGate, ActiveChat> {
  ActiveChatGateProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'activeChatGateProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$activeChatGateHash();

  @$internal
  @override
  ActiveChatGate create() => ActiveChatGate();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ActiveChat value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ActiveChat>(value),
    );
  }
}

String _$activeChatGateHash() => r'2d85541725233c5dbb2167c9da7f2fa267877f2a';

abstract class _$ActiveChatGate extends $Notifier<ActiveChat> {
  ActiveChat build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<ActiveChat, ActiveChat>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<ActiveChat, ActiveChat>,
              ActiveChat,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
