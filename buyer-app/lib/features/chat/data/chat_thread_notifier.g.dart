// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'chat_thread_notifier.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(ChatThread)
final chatThreadProvider = ChatThreadFamily._();

final class ChatThreadProvider
    extends $NotifierProvider<ChatThread, ChatThreadState> {
  ChatThreadProvider._({
    required ChatThreadFamily super.from,
    required ChatThreadArg super.argument,
  }) : super(
         retry: null,
         name: r'chatThreadProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$chatThreadHash();

  @override
  String toString() {
    return r'chatThreadProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  ChatThread create() => ChatThread();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ChatThreadState value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ChatThreadState>(value),
    );
  }

  @override
  bool operator ==(Object other) {
    return other is ChatThreadProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$chatThreadHash() => r'5bf09c8adf3d5732a752e74113569915636f580c';

final class ChatThreadFamily extends $Family
    with
        $ClassFamilyOverride<
          ChatThread,
          ChatThreadState,
          ChatThreadState,
          ChatThreadState,
          ChatThreadArg
        > {
  ChatThreadFamily._()
    : super(
        retry: null,
        name: r'chatThreadProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  ChatThreadProvider call(ChatThreadArg arg) =>
      ChatThreadProvider._(argument: arg, from: this);

  @override
  String toString() => r'chatThreadProvider';
}

abstract class _$ChatThread extends $Notifier<ChatThreadState> {
  late final _$args = ref.$arg as ChatThreadArg;
  ChatThreadArg get arg => _$args;

  ChatThreadState build(ChatThreadArg arg);
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<ChatThreadState, ChatThreadState>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<ChatThreadState, ChatThreadState>,
              ChatThreadState,
              Object?,
              Object?
            >;
    element.handleCreate(ref, () => build(_$args));
  }
}
