// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'social_blocker.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(socialBlocker)
final socialBlockerProvider = SocialBlockerProvider._();

final class SocialBlockerProvider
    extends $FunctionalProvider<SocialBlocker, SocialBlocker, SocialBlocker>
    with $Provider<SocialBlocker> {
  SocialBlockerProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'socialBlockerProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$socialBlockerHash();

  @$internal
  @override
  $ProviderElement<SocialBlocker> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  SocialBlocker create(Ref ref) {
    return socialBlocker(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(SocialBlocker value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<SocialBlocker>(value),
    );
  }
}

String _$socialBlockerHash() => r'1e433fc05678756f1d77c25617396f6d1a92b746';
