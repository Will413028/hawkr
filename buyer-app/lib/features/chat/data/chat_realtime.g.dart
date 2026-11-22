// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'chat_realtime.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(chatRealtime)
final chatRealtimeProvider = ChatRealtimeProvider._();

final class ChatRealtimeProvider
    extends $FunctionalProvider<ChatRealtime, ChatRealtime, ChatRealtime>
    with $Provider<ChatRealtime> {
  ChatRealtimeProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'chatRealtimeProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$chatRealtimeHash();

  @$internal
  @override
  $ProviderElement<ChatRealtime> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  ChatRealtime create(Ref ref) {
    return chatRealtime(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ChatRealtime value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ChatRealtime>(value),
    );
  }
}

String _$chatRealtimeHash() => r'48add6b363c66d36fe6a837c7fcbdd2827d896de';
