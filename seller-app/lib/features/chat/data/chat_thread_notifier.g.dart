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
    extends $AsyncNotifierProvider<ChatThread, List<ChatMessage>> {
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

  @override
  bool operator ==(Object other) {
    return other is ChatThreadProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$chatThreadHash() => r'ccf28ba40d1a4585ccda9aaa1cedcf8560e57210';

final class ChatThreadFamily extends $Family
    with
        $ClassFamilyOverride<
          ChatThread,
          AsyncValue<List<ChatMessage>>,
          List<ChatMessage>,
          FutureOr<List<ChatMessage>>,
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

abstract class _$ChatThread extends $AsyncNotifier<List<ChatMessage>> {
  late final _$args = ref.$arg as ChatThreadArg;
  ChatThreadArg get arg => _$args;

  FutureOr<List<ChatMessage>> build(ChatThreadArg arg);
  @$mustCallSuper
  @override
  void runBuild() {
    final ref =
        this.ref as $Ref<AsyncValue<List<ChatMessage>>, List<ChatMessage>>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<AsyncValue<List<ChatMessage>>, List<ChatMessage>>,
              AsyncValue<List<ChatMessage>>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, () => build(_$args));
  }
}
