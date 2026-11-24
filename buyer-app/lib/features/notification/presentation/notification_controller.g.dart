// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'notification_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 通知列表 AsyncNotifier
///
/// 管理通知列表 + 分頁 + 未讀數量。
/// 使用 `AsyncValue<NotificationListState>` 表示整體狀態。

@ProviderFor(NotificationListController)
final notificationListControllerProvider =
    NotificationListControllerProvider._();

/// 通知列表 AsyncNotifier
///
/// 管理通知列表 + 分頁 + 未讀數量。
/// 使用 `AsyncValue<NotificationListState>` 表示整體狀態。
final class NotificationListControllerProvider
    extends
        $AsyncNotifierProvider<
          NotificationListController,
          NotificationListState
        > {
  /// 通知列表 AsyncNotifier
  ///
  /// 管理通知列表 + 分頁 + 未讀數量。
  /// 使用 `AsyncValue<NotificationListState>` 表示整體狀態。
  NotificationListControllerProvider._()
    : super(
        from: null,
        argument: null,
        retry: _disableNotificationAutomaticRetry,
        name: r'notificationListControllerProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$notificationListControllerHash();

  @$internal
  @override
  NotificationListController create() => NotificationListController();
}

String _$notificationListControllerHash() =>
    r'1e993da9625f802d32ce61bcd3b1becf395d628a';

/// 通知列表 AsyncNotifier
///
/// 管理通知列表 + 分頁 + 未讀數量。
/// 使用 `AsyncValue<NotificationListState>` 表示整體狀態。

abstract class _$NotificationListController
    extends $AsyncNotifier<NotificationListState> {
  FutureOr<NotificationListState> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref =
        this.ref
            as $Ref<AsyncValue<NotificationListState>, NotificationListState>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<
                AsyncValue<NotificationListState>,
                NotificationListState
              >,
              AsyncValue<NotificationListState>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}

/// 通知偏好 AsyncNotifier：伺服器依買家受眾列出實際會送出的（分類、渠道）項目，畫面只呈現這些。

@ProviderFor(NotificationPreferencesController)
final notificationPreferencesControllerProvider =
    NotificationPreferencesControllerProvider._();

/// 通知偏好 AsyncNotifier：伺服器依買家受眾列出實際會送出的（分類、渠道）項目，畫面只呈現這些。
final class NotificationPreferencesControllerProvider
    extends
        $AsyncNotifierProvider<
          NotificationPreferencesController,
          List<pb.PreferenceEntry>
        > {
  /// 通知偏好 AsyncNotifier：伺服器依買家受眾列出實際會送出的（分類、渠道）項目，畫面只呈現這些。
  NotificationPreferencesControllerProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'notificationPreferencesControllerProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() =>
      _$notificationPreferencesControllerHash();

  @$internal
  @override
  NotificationPreferencesController create() =>
      NotificationPreferencesController();
}

String _$notificationPreferencesControllerHash() =>
    r'ba616e5514277602517e9496acecab6111829987';

/// 通知偏好 AsyncNotifier：伺服器依買家受眾列出實際會送出的（分類、渠道）項目，畫面只呈現這些。

abstract class _$NotificationPreferencesController
    extends $AsyncNotifier<List<pb.PreferenceEntry>> {
  FutureOr<List<pb.PreferenceEntry>> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref =
        this.ref
            as $Ref<
              AsyncValue<List<pb.PreferenceEntry>>,
              List<pb.PreferenceEntry>
            >;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<
                AsyncValue<List<pb.PreferenceEntry>>,
                List<pb.PreferenceEntry>
              >,
              AsyncValue<List<pb.PreferenceEntry>>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
