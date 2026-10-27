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
    r'2fe6b0325c6413531bdf61d8686f2b02d69acc10';

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

/// 通知偏好設定 AsyncNotifier
///
/// 透過後端 API 讀寫通知偏好（6 個 category toggle）。

@ProviderFor(NotificationPreferencesController)
final notificationPreferencesControllerProvider =
    NotificationPreferencesControllerProvider._();

/// 通知偏好設定 AsyncNotifier
///
/// 透過後端 API 讀寫通知偏好（6 個 category toggle）。
final class NotificationPreferencesControllerProvider
    extends
        $AsyncNotifierProvider<
          NotificationPreferencesController,
          List<NotificationPreferenceItem>
        > {
  /// 通知偏好設定 AsyncNotifier
  ///
  /// 透過後端 API 讀寫通知偏好（6 個 category toggle）。
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
    r'f6b4faaf5f4e0765190fc25b1c08bac1d2308dd9';

/// 通知偏好設定 AsyncNotifier
///
/// 透過後端 API 讀寫通知偏好（6 個 category toggle）。

abstract class _$NotificationPreferencesController
    extends $AsyncNotifier<List<NotificationPreferenceItem>> {
  FutureOr<List<NotificationPreferenceItem>> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref =
        this.ref
            as $Ref<
              AsyncValue<List<NotificationPreferenceItem>>,
              List<NotificationPreferenceItem>
            >;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<
                AsyncValue<List<NotificationPreferenceItem>>,
                List<NotificationPreferenceItem>
              >,
              AsyncValue<List<NotificationPreferenceItem>>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
