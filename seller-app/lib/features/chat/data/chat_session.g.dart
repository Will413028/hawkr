// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'chat_session.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(chatSession)
final chatSessionProvider = ChatSessionProvider._();

final class ChatSessionProvider
    extends $FunctionalProvider<ChatSession, ChatSession, ChatSession>
    with $Provider<ChatSession> {
  ChatSessionProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'chatSessionProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$chatSessionHash();

  @$internal
  @override
  $ProviderElement<ChatSession> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  ChatSession create(Ref ref) {
    return chatSession(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ChatSession value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ChatSession>(value),
    );
  }
}

String _$chatSessionHash() => r'd1fb5ed8bec95ff2f32918392ad9f11ba00eb492';
