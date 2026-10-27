// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'shipment_anomaly_providers.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 配送異常統計（首頁 banner / dashboard 用）。
///
/// 失敗時降級為 0 的 stats（避免 banner 阻塞首頁）。

@ProviderFor(shipmentAnomalyStats)
final shipmentAnomalyStatsProvider = ShipmentAnomalyStatsProvider._();

/// 配送異常統計（首頁 banner / dashboard 用）。
///
/// 失敗時降級為 0 的 stats（避免 banner 阻塞首頁）。

final class ShipmentAnomalyStatsProvider
    extends
        $FunctionalProvider<
          AsyncValue<ShipmentAnomalyStats>,
          ShipmentAnomalyStats,
          FutureOr<ShipmentAnomalyStats>
        >
    with
        $FutureModifier<ShipmentAnomalyStats>,
        $FutureProvider<ShipmentAnomalyStats> {
  /// 配送異常統計（首頁 banner / dashboard 用）。
  ///
  /// 失敗時降級為 0 的 stats（避免 banner 阻塞首頁）。
  ShipmentAnomalyStatsProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'shipmentAnomalyStatsProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$shipmentAnomalyStatsHash();

  @$internal
  @override
  $FutureProviderElement<ShipmentAnomalyStats> $createElement(
    $ProviderPointer pointer,
  ) => $FutureProviderElement(pointer);

  @override
  FutureOr<ShipmentAnomalyStats> create(Ref ref) {
    return shipmentAnomalyStats(ref);
  }
}

String _$shipmentAnomalyStatsHash() =>
    r'071e53d18e76be98e4018dd6d6a2a7b22507d027';

/// 賣家訂單列表用：orderNo → 是否有未 ack 的 anomaly。
///
/// 失敗時回空 map（不影響訂單卡顯示）。

@ProviderFor(shipmentAnomalyByOrderNo)
final shipmentAnomalyByOrderNoProvider = ShipmentAnomalyByOrderNoProvider._();

/// 賣家訂單列表用：orderNo → 是否有未 ack 的 anomaly。
///
/// 失敗時回空 map（不影響訂單卡顯示）。

final class ShipmentAnomalyByOrderNoProvider
    extends
        $FunctionalProvider<
          AsyncValue<Map<String, ShipmentAnomaly>>,
          Map<String, ShipmentAnomaly>,
          FutureOr<Map<String, ShipmentAnomaly>>
        >
    with
        $FutureModifier<Map<String, ShipmentAnomaly>>,
        $FutureProvider<Map<String, ShipmentAnomaly>> {
  /// 賣家訂單列表用：orderNo → 是否有未 ack 的 anomaly。
  ///
  /// 失敗時回空 map（不影響訂單卡顯示）。
  ShipmentAnomalyByOrderNoProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'shipmentAnomalyByOrderNoProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$shipmentAnomalyByOrderNoHash();

  @$internal
  @override
  $FutureProviderElement<Map<String, ShipmentAnomaly>> $createElement(
    $ProviderPointer pointer,
  ) => $FutureProviderElement(pointer);

  @override
  FutureOr<Map<String, ShipmentAnomaly>> create(Ref ref) {
    return shipmentAnomalyByOrderNo(ref);
  }
}

String _$shipmentAnomalyByOrderNoHash() =>
    r'aee17e029241cfcb2c0788f9c3ddd0826cb0068c';

@ProviderFor(ShipmentAnomalyDashboardController)
final shipmentAnomalyDashboardControllerProvider =
    ShipmentAnomalyDashboardControllerProvider._();

final class ShipmentAnomalyDashboardControllerProvider
    extends
        $AsyncNotifierProvider<
          ShipmentAnomalyDashboardController,
          ShipmentAnomalyDashboardState
        > {
  ShipmentAnomalyDashboardControllerProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'shipmentAnomalyDashboardControllerProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() =>
      _$shipmentAnomalyDashboardControllerHash();

  @$internal
  @override
  ShipmentAnomalyDashboardController create() =>
      ShipmentAnomalyDashboardController();
}

String _$shipmentAnomalyDashboardControllerHash() =>
    r'446b9e8a3218666f7cafbfab0814aefaecccd761';

abstract class _$ShipmentAnomalyDashboardController
    extends $AsyncNotifier<ShipmentAnomalyDashboardState> {
  FutureOr<ShipmentAnomalyDashboardState> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref =
        this.ref
            as $Ref<
              AsyncValue<ShipmentAnomalyDashboardState>,
              ShipmentAnomalyDashboardState
            >;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<
                AsyncValue<ShipmentAnomalyDashboardState>,
                ShipmentAnomalyDashboardState
              >,
              AsyncValue<ShipmentAnomalyDashboardState>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
