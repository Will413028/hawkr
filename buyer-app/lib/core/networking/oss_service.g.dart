// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'oss_service.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(ossService)
final ossServiceProvider = OssServiceProvider._();

final class OssServiceProvider
    extends $FunctionalProvider<OssService, OssService, OssService>
    with $Provider<OssService> {
  OssServiceProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'ossServiceProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$ossServiceHash();

  @$internal
  @override
  $ProviderElement<OssService> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  OssService create(Ref ref) {
    return ossService(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(OssService value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<OssService>(value),
    );
  }
}

String _$ossServiceHash() => r'19f32a9201f5c7e163b6d72ba2ed2bdf6723453a';
