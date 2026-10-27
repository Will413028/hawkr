// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'subscriber_repository.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(subscriberRepository)
final subscriberRepositoryProvider = SubscriberRepositoryProvider._();

final class SubscriberRepositoryProvider
    extends
        $FunctionalProvider<
          SubscriberRepository,
          SubscriberRepository,
          SubscriberRepository
        >
    with $Provider<SubscriberRepository> {
  SubscriberRepositoryProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'subscriberRepositoryProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$subscriberRepositoryHash();

  @$internal
  @override
  $ProviderElement<SubscriberRepository> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  SubscriberRepository create(Ref ref) {
    return subscriberRepository(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(SubscriberRepository value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<SubscriberRepository>(value),
    );
  }
}

String _$subscriberRepositoryHash() =>
    r'10bd8a5656b1443ee48704c97d1c5cb22fda39af';
