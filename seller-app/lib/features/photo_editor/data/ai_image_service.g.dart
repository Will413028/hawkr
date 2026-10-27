// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'ai_image_service.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(aiImageService)
final aiImageServiceProvider = AiImageServiceProvider._();

final class AiImageServiceProvider
    extends $FunctionalProvider<AiImageService, AiImageService, AiImageService>
    with $Provider<AiImageService> {
  AiImageServiceProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'aiImageServiceProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$aiImageServiceHash();

  @$internal
  @override
  $ProviderElement<AiImageService> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  AiImageService create(Ref ref) {
    return aiImageService(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(AiImageService value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<AiImageService>(value),
    );
  }
}

String _$aiImageServiceHash() => r'c7932d2d52fc16ee00623cdda4de6d3230b2e417';
