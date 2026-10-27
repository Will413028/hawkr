// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'protected_media_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// Command boundary used by views that need image bytes.
///
/// It is kept alive because callers may issue multiple parallel downloads via
/// `ref.read` without watching command state.

@ProviderFor(ProtectedMediaController)
final protectedMediaControllerProvider = ProtectedMediaControllerProvider._();

/// Command boundary used by views that need image bytes.
///
/// It is kept alive because callers may issue multiple parallel downloads via
/// `ref.read` without watching command state.
final class ProtectedMediaControllerProvider
    extends $NotifierProvider<ProtectedMediaController, void> {
  /// Command boundary used by views that need image bytes.
  ///
  /// It is kept alive because callers may issue multiple parallel downloads via
  /// `ref.read` without watching command state.
  ProtectedMediaControllerProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'protectedMediaControllerProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$protectedMediaControllerHash();

  @$internal
  @override
  ProtectedMediaController create() => ProtectedMediaController();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(void value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<void>(value),
    );
  }
}

String _$protectedMediaControllerHash() =>
    r'464384be44f0feb00e8c21d1210fdbc7d2614519';

/// Command boundary used by views that need image bytes.
///
/// It is kept alive because callers may issue multiple parallel downloads via
/// `ref.read` without watching command state.

abstract class _$ProtectedMediaController extends $Notifier<void> {
  void build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<void, void>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<void, void>,
              void,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
