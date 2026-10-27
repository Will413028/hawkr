// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'chat_picker_remote_service.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(chatPickerRemoteService)
final chatPickerRemoteServiceProvider = ChatPickerRemoteServiceProvider._();

final class ChatPickerRemoteServiceProvider
    extends
        $FunctionalProvider<
          ChatPickerRemoteService,
          ChatPickerRemoteService,
          ChatPickerRemoteService
        >
    with $Provider<ChatPickerRemoteService> {
  ChatPickerRemoteServiceProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'chatPickerRemoteServiceProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$chatPickerRemoteServiceHash();

  @$internal
  @override
  $ProviderElement<ChatPickerRemoteService> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  ChatPickerRemoteService create(Ref ref) {
    return chatPickerRemoteService(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ChatPickerRemoteService value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ChatPickerRemoteService>(value),
    );
  }
}

String _$chatPickerRemoteServiceHash() =>
    r'8d717627bdcbbc560ae1e00f6155681546c1ec46';
