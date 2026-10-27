// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'subscriber_remote_service.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(subscriberRemoteService)
final subscriberRemoteServiceProvider = SubscriberRemoteServiceProvider._();

final class SubscriberRemoteServiceProvider
    extends
        $FunctionalProvider<
          SubscriberRemoteService,
          SubscriberRemoteService,
          SubscriberRemoteService
        >
    with $Provider<SubscriberRemoteService> {
  SubscriberRemoteServiceProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'subscriberRemoteServiceProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$subscriberRemoteServiceHash();

  @$internal
  @override
  $ProviderElement<SubscriberRemoteService> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  SubscriberRemoteService create(Ref ref) {
    return subscriberRemoteService(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(SubscriberRemoteService value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<SubscriberRemoteService>(value),
    );
  }
}

String _$subscriberRemoteServiceHash() =>
    r'b1ffe12ca75d2afa1a796e7c119ed9a6067238c1';
