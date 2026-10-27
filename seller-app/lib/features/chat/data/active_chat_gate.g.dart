// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'active_chat_gate.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// Singleton gate notifier，App 整個生命週期只有一個 state。

@ProviderFor(ActiveChatGate)
final activeChatGateProvider = ActiveChatGateProvider._();

/// Singleton gate notifier，App 整個生命週期只有一個 state。
final class ActiveChatGateProvider
    extends $NotifierProvider<ActiveChatGate, ActiveChat> {
  /// Singleton gate notifier，App 整個生命週期只有一個 state。
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

String _$activeChatGateHash() => r'10946f896a7d1953b4af8dbca41706735da1c3b5';

/// Singleton gate notifier，App 整個生命週期只有一個 state。

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
