// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'protected_media_remote_service.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(protectedMediaRemoteService)
final protectedMediaRemoteServiceProvider =
    ProtectedMediaRemoteServiceProvider._();

final class ProtectedMediaRemoteServiceProvider
    extends
        $FunctionalProvider<
          ProtectedMediaRemoteService,
          ProtectedMediaRemoteService,
          ProtectedMediaRemoteService
        >
    with $Provider<ProtectedMediaRemoteService> {
  ProtectedMediaRemoteServiceProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'protectedMediaRemoteServiceProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$protectedMediaRemoteServiceHash();

  @$internal
  @override
  $ProviderElement<ProtectedMediaRemoteService> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  ProtectedMediaRemoteService create(Ref ref) {
    return protectedMediaRemoteService(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ProtectedMediaRemoteService value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ProtectedMediaRemoteService>(value),
    );
  }
}

String _$protectedMediaRemoteServiceHash() =>
    r'73a5c5e8ad48f26a126e3503aa77a7e7944ab491';
