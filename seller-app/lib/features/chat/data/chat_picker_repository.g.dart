// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'chat_picker_repository.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(chatPickerRepository)
final chatPickerRepositoryProvider = ChatPickerRepositoryProvider._();

final class ChatPickerRepositoryProvider
    extends
        $FunctionalProvider<
          ChatPickerRepository,
          ChatPickerRepository,
          ChatPickerRepository
        >
    with $Provider<ChatPickerRepository> {
  ChatPickerRepositoryProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'chatPickerRepositoryProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$chatPickerRepositoryHash();

  @$internal
  @override
  $ProviderElement<ChatPickerRepository> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  ChatPickerRepository create(Ref ref) {
    return chatPickerRepository(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ChatPickerRepository value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ChatPickerRepository>(value),
    );
  }
}

String _$chatPickerRepositoryHash() =>
    r'0b15c524e41826041271c827065b895b7e9d34b3';
