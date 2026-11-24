// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'notification_signal_service.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(notificationSignalService)
final notificationSignalServiceProvider = NotificationSignalServiceProvider._();

final class NotificationSignalServiceProvider
    extends
        $FunctionalProvider<
          NotificationSignalService,
          NotificationSignalService,
          NotificationSignalService
        >
    with $Provider<NotificationSignalService> {
  NotificationSignalServiceProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'notificationSignalServiceProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$notificationSignalServiceHash();

  @$internal
  @override
  $ProviderElement<NotificationSignalService> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  NotificationSignalService create(Ref ref) {
    return notificationSignalService(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(NotificationSignalService value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<NotificationSignalService>(value),
    );
  }
}

String _$notificationSignalServiceHash() =>
    r'5f43fc2245e988ae0e0e56ba463125d1a6ea3241';
