// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'notification_signals_provider.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 登入中（session 還原或登入完成）連上通知串流，登出或換帳號就斷線重連。

@ProviderFor(notificationSignals)
final notificationSignalsProvider = NotificationSignalsProvider._();

/// 登入中（session 還原或登入完成）連上通知串流，登出或換帳號就斷線重連。

final class NotificationSignalsProvider
    extends
        $FunctionalProvider<
          NotificationSignals,
          NotificationSignals,
          NotificationSignals
        >
    with $Provider<NotificationSignals> {
  /// 登入中（session 還原或登入完成）連上通知串流，登出或換帳號就斷線重連。
  NotificationSignalsProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'notificationSignalsProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$notificationSignalsHash();

  @$internal
  @override
  $ProviderElement<NotificationSignals> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  NotificationSignals create(Ref ref) {
    return notificationSignals(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(NotificationSignals value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<NotificationSignals>(value),
    );
  }
}

String _$notificationSignalsHash() =>
    r'ccf861ffdb58b23857dbdc256e3ab7527eea76bf';
