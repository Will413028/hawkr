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
    extends $AsyncNotifierProvider<ChatThread, ChatThreadData> {
  ChatThreadProvider._({
    required ChatThreadFamily super.from,
    required ChatThreadArg super.argument,
  }) : super(
         retry: _noAutoRetry,
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

String _$chatThreadHash() => r'95a6106980638fd3218a05119bf31567199b0364';

final class ChatThreadFamily extends $Family
    with
        $ClassFamilyOverride<
          ChatThread,
          AsyncValue<ChatThreadData>,
          ChatThreadData,
          FutureOr<ChatThreadData>,
          ChatThreadArg
        > {
  ChatThreadFamily._()
    : super(
        retry: _noAutoRetry,
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

abstract class _$ChatThread extends $AsyncNotifier<ChatThreadData> {
  late final _$args = ref.$arg as ChatThreadArg;
  ChatThreadArg get arg => _$args;

  FutureOr<ChatThreadData> build(ChatThreadArg arg);
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AsyncValue<ChatThreadData>, ChatThreadData>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<AsyncValue<ChatThreadData>, ChatThreadData>,
              AsyncValue<ChatThreadData>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, () => build(_$args));
  }
}
