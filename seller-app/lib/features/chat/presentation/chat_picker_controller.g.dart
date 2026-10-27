// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'chat_picker_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(ChatPickerController)
final chatPickerControllerProvider = ChatPickerControllerProvider._();

final class ChatPickerControllerProvider
    extends $NotifierProvider<ChatPickerController, ChatPickerState> {
  ChatPickerControllerProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'chatPickerControllerProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$chatPickerControllerHash();

  @$internal
  @override
  ChatPickerController create() => ChatPickerController();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ChatPickerState value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ChatPickerState>(value),
    );
  }
}

String _$chatPickerControllerHash() =>
    r'810f42f921a221f108b3a30a7f5f7787462acc2e';

abstract class _$ChatPickerController extends $Notifier<ChatPickerState> {
  ChatPickerState build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<ChatPickerState, ChatPickerState>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<ChatPickerState, ChatPickerState>,
              ChatPickerState,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
