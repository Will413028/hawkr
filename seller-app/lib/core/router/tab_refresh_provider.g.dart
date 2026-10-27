// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'tab_refresh_provider.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// Counter provider — Shell scaffold 在切換到 Category tab 時遞增，
/// PageScreen 透過 ref.listen 偵測變化後觸發 banner 刷新。

@ProviderFor(PageTabRefresh)
final pageTabRefreshProvider = PageTabRefreshProvider._();

/// Counter provider — Shell scaffold 在切換到 Category tab 時遞增，
/// PageScreen 透過 ref.listen 偵測變化後觸發 banner 刷新。
final class PageTabRefreshProvider
    extends $NotifierProvider<PageTabRefresh, int> {
  /// Counter provider — Shell scaffold 在切換到 Category tab 時遞增，
  /// PageScreen 透過 ref.listen 偵測變化後觸發 banner 刷新。
  PageTabRefreshProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'pageTabRefreshProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$pageTabRefreshHash();

  @$internal
  @override
  PageTabRefresh create() => PageTabRefresh();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(int value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<int>(value),
    );
  }
}

String _$pageTabRefreshHash() => r'1f5ef6fc3f9cbd2119c51ca58c99cc16dc476bf5';

/// Counter provider — Shell scaffold 在切換到 Category tab 時遞增，
/// PageScreen 透過 ref.listen 偵測變化後觸發 banner 刷新。

abstract class _$PageTabRefresh extends $Notifier<int> {
  int build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<int, int>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<int, int>,
              int,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}

/// Counter provider — Shell scaffold 在切換到 Settings tab 時遞增，
/// ProfileScreen 透過 ref.listen 偵測變化後觸發認證狀態刷新。

@ProviderFor(SettingsTabRefresh)
final settingsTabRefreshProvider = SettingsTabRefreshProvider._();

/// Counter provider — Shell scaffold 在切換到 Settings tab 時遞增，
/// ProfileScreen 透過 ref.listen 偵測變化後觸發認證狀態刷新。
final class SettingsTabRefreshProvider
    extends $NotifierProvider<SettingsTabRefresh, int> {
  /// Counter provider — Shell scaffold 在切換到 Settings tab 時遞增，
  /// ProfileScreen 透過 ref.listen 偵測變化後觸發認證狀態刷新。
  SettingsTabRefreshProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'settingsTabRefreshProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$settingsTabRefreshHash();

  @$internal
  @override
  SettingsTabRefresh create() => SettingsTabRefresh();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(int value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<int>(value),
    );
  }
}

String _$settingsTabRefreshHash() =>
    r'8acc4a537618199dccb10c6c72f4d12f1975550e';

/// Counter provider — Shell scaffold 在切換到 Settings tab 時遞增，
/// ProfileScreen 透過 ref.listen 偵測變化後觸發認證狀態刷新。

abstract class _$SettingsTabRefresh extends $Notifier<int> {
  int build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<int, int>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<int, int>,
              int,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}

/// Counter provider — Shell scaffold 在切換到 Orders tab 時遞增，
/// OrderListScreen 透過 ref.listen 偵測變化後重新拉取最新訂單。

@ProviderFor(OrdersTabRefresh)
final ordersTabRefreshProvider = OrdersTabRefreshProvider._();

/// Counter provider — Shell scaffold 在切換到 Orders tab 時遞增，
/// OrderListScreen 透過 ref.listen 偵測變化後重新拉取最新訂單。
final class OrdersTabRefreshProvider
    extends $NotifierProvider<OrdersTabRefresh, int> {
  /// Counter provider — Shell scaffold 在切換到 Orders tab 時遞增，
  /// OrderListScreen 透過 ref.listen 偵測變化後重新拉取最新訂單。
  OrdersTabRefreshProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'ordersTabRefreshProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$ordersTabRefreshHash();

  @$internal
  @override
  OrdersTabRefresh create() => OrdersTabRefresh();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(int value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<int>(value),
    );
  }
}

String _$ordersTabRefreshHash() => r'b01cf1ea6cc342153c1440fde5f75ee6a71c5112';

/// Counter provider — Shell scaffold 在切換到 Orders tab 時遞增，
/// OrderListScreen 透過 ref.listen 偵測變化後重新拉取最新訂單。

abstract class _$OrdersTabRefresh extends $Notifier<int> {
  int build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<int, int>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<int, int>,
              int,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}

/// 用於從 Dashboard / 通知導航到訂單 tab 時指定初始 tab index。
/// 設定後由 OrderListScreen 消費並重置為 null。

@ProviderFor(OrdersInitialTab)
final ordersInitialTabProvider = OrdersInitialTabProvider._();

/// 用於從 Dashboard / 通知導航到訂單 tab 時指定初始 tab index。
/// 設定後由 OrderListScreen 消費並重置為 null。
final class OrdersInitialTabProvider
    extends $NotifierProvider<OrdersInitialTab, int?> {
  /// 用於從 Dashboard / 通知導航到訂單 tab 時指定初始 tab index。
  /// 設定後由 OrderListScreen 消費並重置為 null。
  OrdersInitialTabProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'ordersInitialTabProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$ordersInitialTabHash();

  @$internal
  @override
  OrdersInitialTab create() => OrdersInitialTab();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(int? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<int?>(value),
    );
  }
}

String _$ordersInitialTabHash() => r'c1d03fbdd046dd3457188d1e408e66537cf7ba95';

/// 用於從 Dashboard / 通知導航到訂單 tab 時指定初始 tab index。
/// 設定後由 OrderListScreen 消費並重置為 null。

abstract class _$OrdersInitialTab extends $Notifier<int?> {
  int? build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<int?, int?>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<int?, int?>,
              int?,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
