import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/exceptions/app_exception.dart';
import '../../../core/networking/dio_provider.dart';
import 'shipment_anomaly_model.dart';

part 'shipment_anomaly_repository.g.dart';

@riverpod
ShipmentAnomalyRepository shipmentAnomalyRepository(Ref ref) {
  return ShipmentAnomalyRepository(ref.watch(dioProvider));
}

/// 配送異常 Repository
///
/// 對應後端 shipping-service §5.21 的 4 個 RPC（透過 Gateway HTTP 暴露）。
class ShipmentAnomalyRepository {
  final Dio _dio;

  ShipmentAnomalyRepository(this._dio);

  static const _base = '/api/v1/app/shipments/anomalies';

  /// GET /api/v1/app/shipments/anomalies
  Future<ShipmentAnomalyListResult> list({
    String? anomalyType,
    String? severity,
    bool showAcknowledged = false,
  }) async {
    try {
      // Gateway 期望 snake_case query params（gateway/internal/handler/shipment_anomaly.go）
      final params = <String, dynamic>{'show_acknowledged': showAcknowledged};
      if (anomalyType != null && anomalyType.isNotEmpty) {
        params['anomaly_type'] = anomalyType;
      }
      if (severity != null && severity.isNotEmpty) {
        params['severity'] = severity;
      }
      final response = await _dio.get(_base, queryParameters: params);
      final data = response.data;
      if (data['code'] == 0) {
        final raw = data['data'];
        if (raw is Map<String, dynamic>) {
          return ShipmentAnomalyListResult.fromJson(raw);
        }
        return const ShipmentAnomalyListResult();
      }
      throw AppException.fromApiError(
        data['code'] as int,
        data['message'] as String? ?? '取得配送異常列表失敗',
      );
    } on DioException catch (e) {
      throw AppException.fromDioException(e);
    }
  }

  /// GET /api/v1/app/shipments/anomalies/stats
  Future<ShipmentAnomalyStats> stats() async {
    try {
      final response = await _dio.get('$_base/stats');
      final data = response.data;
      if (data['code'] == 0) {
        final raw = data['data'];
        if (raw is Map<String, dynamic>) {
          return ShipmentAnomalyStats.fromJson(raw);
        }
        return const ShipmentAnomalyStats();
      }
      throw AppException.fromApiError(
        data['code'] as int,
        data['message'] as String? ?? '取得配送異常統計失敗',
      );
    } on DioException catch (e) {
      throw AppException.fromDioException(e);
    }
  }

  /// POST /api/v1/app/shipments/anomalies/{shipment_id}/{anomaly_type}/acknowledge
  Future<void> acknowledge(String shipmentId, String anomalyType) async {
    await _act(shipmentId, anomalyType, 'acknowledge');
  }

  /// POST /api/v1/app/shipments/anomalies/{shipment_id}/{anomaly_type}/ignore
  Future<void> ignore(String shipmentId, String anomalyType) async {
    await _act(shipmentId, anomalyType, 'ignore');
  }

  Future<void> _act(
    String shipmentId,
    String anomalyType,
    String action,
  ) async {
    try {
      final response = await _dio.post(
        '$_base/$shipmentId/$anomalyType/$action',
      );
      final data = response.data;
      if (data['code'] != 0) {
        throw AppException.fromApiError(
          data['code'] as int,
          data['message'] as String? ?? '操作失敗',
        );
      }
    } on DioException catch (e) {
      throw AppException.fromDioException(e);
    }
  }
}
