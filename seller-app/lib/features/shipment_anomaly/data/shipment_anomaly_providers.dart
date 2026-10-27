import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../shared/utils/hk_log.dart';
import 'shipment_anomaly_model.dart';
import 'shipment_anomaly_repository.dart';

part 'shipment_anomaly_providers.g.dart';

/// 配送異常統計（首頁 banner / dashboard 用）。
///
/// 失敗時降級為 0 的 stats（避免 banner 阻塞首頁）。
@riverpod
Future<ShipmentAnomalyStats> shipmentAnomalyStats(Ref ref) async {
  try {
    return await ref.read(shipmentAnomalyRepositoryProvider).stats();
  } catch (e) {
    HKLog.logW('[shipmentAnomalyStats] 取得統計失敗，降級為 0: $e');
    return const ShipmentAnomalyStats();
  }
}

/// 賣家訂單列表用：orderNo → 是否有未 ack 的 anomaly。
///
/// 失敗時回空 map（不影響訂單卡顯示）。
@riverpod
Future<Map<String, ShipmentAnomaly>> shipmentAnomalyByOrderNo(Ref ref) async {
  try {
    final result = await ref.read(shipmentAnomalyRepositoryProvider).list();
    final map = <String, ShipmentAnomaly>{};
    for (final a in result.records) {
      if (a.orderNo.isEmpty) continue;
      // 同 orderNo 多筆 anomaly 時保留第一筆即可（badge 不需精細）。
      map.putIfAbsent(a.orderNo, () => a);
    }
    return map;
  } catch (e) {
    HKLog.logW('[shipmentAnomalyByOrderNo] 取得 anomaly map 失敗，降級為空: $e');
    return const <String, ShipmentAnomaly>{};
  }
}

// ============================================================
// Dashboard 列表 Controller
// ============================================================

/// Dashboard 列表查詢條件。
class ShipmentAnomalyFilter {
  final String? anomalyType;
  final String? severity;
  final bool showAcknowledged;

  const ShipmentAnomalyFilter({
    this.anomalyType,
    this.severity,
    this.showAcknowledged = false,
  });

  ShipmentAnomalyFilter copyWith({
    String? anomalyType,
    String? severity,
    bool? showAcknowledged,
    bool clearAnomalyType = false,
    bool clearSeverity = false,
  }) {
    return ShipmentAnomalyFilter(
      anomalyType: clearAnomalyType ? null : (anomalyType ?? this.anomalyType),
      severity: clearSeverity ? null : (severity ?? this.severity),
      showAcknowledged: showAcknowledged ?? this.showAcknowledged,
    );
  }
}

/// Dashboard 頁面狀態。
class ShipmentAnomalyDashboardState {
  final List<ShipmentAnomaly> records;
  final ShipmentAnomalyStats stats;
  final ShipmentAnomalyFilter filter;

  const ShipmentAnomalyDashboardState({
    this.records = const [],
    this.stats = const ShipmentAnomalyStats(),
    this.filter = const ShipmentAnomalyFilter(),
  });

  ShipmentAnomalyDashboardState copyWith({
    List<ShipmentAnomaly>? records,
    ShipmentAnomalyStats? stats,
    ShipmentAnomalyFilter? filter,
  }) {
    return ShipmentAnomalyDashboardState(
      records: records ?? this.records,
      stats: stats ?? this.stats,
      filter: filter ?? this.filter,
    );
  }
}

@Riverpod(keepAlive: false)
class ShipmentAnomalyDashboardController
    extends _$ShipmentAnomalyDashboardController {
  ShipmentAnomalyRepository get _repo =>
      ref.read(shipmentAnomalyRepositoryProvider);

  @override
  Future<ShipmentAnomalyDashboardState> build() async {
    return _load(const ShipmentAnomalyFilter());
  }

  Future<ShipmentAnomalyDashboardState> _load(
    ShipmentAnomalyFilter filter,
  ) async {
    final results = await Future.wait([
      _repo.list(
        anomalyType: filter.anomalyType,
        severity: filter.severity,
        showAcknowledged: filter.showAcknowledged,
      ),
      _repo.stats(),
    ]);
    final list = results[0] as ShipmentAnomalyListResult;
    final stats = results[1] as ShipmentAnomalyStats;
    return ShipmentAnomalyDashboardState(
      records: list.records,
      stats: stats,
      filter: filter,
    );
  }

  /// 切換 filter（會重新拉資料）。
  Future<void> updateFilter(ShipmentAnomalyFilter filter) async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() => _load(filter));
  }

  /// 下拉刷新（用當前 filter）。
  Future<void> refresh() async {
    final current = state.value?.filter ?? const ShipmentAnomalyFilter();
    state = await AsyncValue.guard(() => _load(current));
  }

  /// 標記已處理。
  Future<bool> acknowledge(String shipmentId, String anomalyType) async {
    try {
      await _repo.acknowledge(shipmentId, anomalyType);
      // 同步刷新 banner 統計
      ref.invalidate(shipmentAnomalyStatsProvider);
      ref.invalidate(shipmentAnomalyByOrderNoProvider);
      await refresh();
      return true;
    } catch (_) {
      return false;
    }
  }

  /// 標記忽略。
  Future<bool> markIgnored(String shipmentId, String anomalyType) async {
    try {
      await _repo.ignore(shipmentId, anomalyType);
      ref.invalidate(shipmentAnomalyStatsProvider);
      ref.invalidate(shipmentAnomalyByOrderNoProvider);
      await refresh();
      return true;
    } catch (_) {
      return false;
    }
  }
}
