// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'chat_conversations_notifier.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 店家的對話列表。讀取失敗是 [AsyncError]，與「沒有對話」（空的 items）分開。

@ProviderFor(ChatConversations)
final chatConversationsProvider = ChatConversationsProvider._();

/// 店家的對話列表。讀取失敗是 [AsyncError]，與「沒有對話」（空的 items）分開。
final class ChatConversationsProvider
    extends $AsyncNotifierProvider<ChatConversations, ChatConversationsState> {
  /// 店家的對話列表。讀取失敗是 [AsyncError]，與「沒有對話」（空的 items）分開。
  ChatConversationsProvider._()
    : super(
        from: null,
        argument: null,
        retry: _noAutoRetry,
        name: r'chatConversationsProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$chatConversationsHash();

  @$internal
  @override
  ChatConversations create() => ChatConversations();
}

String _$chatConversationsHash() => r'c37433c67d2ff48963620d59f769445b70cc06f8';

/// 店家的對話列表。讀取失敗是 [AsyncError]，與「沒有對話」（空的 items）分開。

abstract class _$ChatConversations
    extends $AsyncNotifier<ChatConversationsState> {
  FutureOr<ChatConversationsState> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref =
        this.ref
            as $Ref<AsyncValue<ChatConversationsState>, ChatConversationsState>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<
                AsyncValue<ChatConversationsState>,
                ChatConversationsState
              >,
              AsyncValue<ChatConversationsState>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
