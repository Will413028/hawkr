// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'notification_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 收件匣 AsyncNotifier：第一頁、分頁與未讀數都來自伺服器；已讀與刪除成功後以伺服器的答案為準。

@ProviderFor(NotificationListController)
final notificationListControllerProvider =
    NotificationListControllerProvider._();

/// 收件匣 AsyncNotifier：第一頁、分頁與未讀數都來自伺服器；已讀與刪除成功後以伺服器的答案為準。
final class NotificationListControllerProvider
    extends
        $AsyncNotifierProvider<
          NotificationListController,
          NotificationListState
        > {
  /// 收件匣 AsyncNotifier：第一頁、分頁與未讀數都來自伺服器；已讀與刪除成功後以伺服器的答案為準。
  NotificationListControllerProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
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
    r'b9dae0ae67e3fbd2fc55b1b6b35068c76416a461';

/// 收件匣 AsyncNotifier：第一頁、分頁與未讀數都來自伺服器；已讀與刪除成功後以伺服器的答案為準。

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

/// 偏好：伺服器回哪些 (分類, 渠道) 就列哪些；不可關的 `canDisable` 為 false。

@ProviderFor(NotificationPreferencesController)
final notificationPreferencesControllerProvider =
    NotificationPreferencesControllerProvider._();

/// 偏好：伺服器回哪些 (分類, 渠道) 就列哪些；不可關的 `canDisable` 為 false。
final class NotificationPreferencesControllerProvider
    extends
        $AsyncNotifierProvider<
          NotificationPreferencesController,
          List<PreferenceEntry>
        > {
  /// 偏好：伺服器回哪些 (分類, 渠道) 就列哪些；不可關的 `canDisable` 為 false。
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
    r'9731de9ac396a833e150b339229e647f75f3a3a9';

/// 偏好：伺服器回哪些 (分類, 渠道) 就列哪些；不可關的 `canDisable` 為 false。

abstract class _$NotificationPreferencesController
    extends $AsyncNotifier<List<PreferenceEntry>> {
  FutureOr<List<PreferenceEntry>> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref =
        this.ref
            as $Ref<AsyncValue<List<PreferenceEntry>>, List<PreferenceEntry>>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<
                AsyncValue<List<PreferenceEntry>>,
                List<PreferenceEntry>
              >,
              AsyncValue<List<PreferenceEntry>>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
