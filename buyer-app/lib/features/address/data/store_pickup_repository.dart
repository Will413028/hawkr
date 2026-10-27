import 'package:dio/dio.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/exceptions/app_exception.dart';
import '../../../core/networking/dio_provider.dart';
import '../../../shared/utils/hk_log.dart';
import 'store_pickup_model.dart';

final storePickupRepositoryProvider = Provider<StorePickupRepository>(
  (ref) => StorePickupRepository(ref.watch(dioProvider)),
);

class StorePickupRepository {
  StorePickupRepository(this._dio);

  final Dio _dio;

  Future<List<StorePickup>> getAll() async {
    try {
      final response = await _dio.get<Map<String, dynamic>>(
        '/app/store_pickups',
      );
      final body = response.data ?? const <String, dynamic>{};
      _ensureSuccess(body, '載入取貨門市失敗');
      final data = body['data'];
      final records = data is List
          ? data
          : data is Map<String, dynamic> && data['records'] is List
          ? data['records'] as List
          : const <dynamic>[];
      return records
          .whereType<Map>()
          .map(
            (record) => StorePickup.fromJson(Map<String, dynamic>.from(record)),
          )
          .where((pickup) => pickup.id.isNotEmpty)
          .toList(growable: false);
    } on DioException catch (error) {
      _throwNetworkError(error, '載入取貨門市失敗');
    }
  }

  Future<void> create(StorePickup pickup) async {
    try {
      final response = await _dio.post<Map<String, dynamic>>(
        '/app/store_pickups',
        data: pickup.toJson()..remove('id'),
      );
      _ensureSuccess(response.data, '新增取貨門市失敗');
    } on DioException catch (error) {
      _throwNetworkError(error, '新增取貨門市失敗');
    }
  }

  Future<void> update(StorePickup pickup) async {
    try {
      final response = await _dio.put<Map<String, dynamic>>(
        '/app/store_pickups/${Uri.encodeComponent(pickup.id)}',
        data: pickup.toJson()..remove('id'),
      );
      _ensureSuccess(response.data, '更新取貨門市失敗');
    } on DioException catch (error) {
      _throwNetworkError(error, '更新取貨門市失敗');
    }
  }

  Future<void> delete(String id) async {
    try {
      final response = await _dio.delete<Map<String, dynamic>>(
        '/app/store_pickups/${Uri.encodeComponent(id)}',
      );
      _ensureSuccess(response.data, '刪除取貨門市失敗');
    } on DioException catch (error) {
      _throwNetworkError(error, '刪除取貨門市失敗');
    }
  }

  Future<String> createMapSession() async {
    try {
      final response = await _dio.post<Map<String, dynamic>>(
        '/app/store_map_sessions',
        data: const {'appType': 'buyer'},
      );
      final body = response.data;
      _ensureSuccess(body, '無法開啟門市地圖');
      final data = body?['data'];
      final redirectUrl = data is Map
          ? data['redirectUrl']?.toString().trim() ?? ''
          : '';
      if (redirectUrl.isEmpty) {
        throw const ServerException('無法取得門市地圖連結');
      }
      return redirectUrl;
    } on DioException catch (error) {
      _throwNetworkError(error, '無法開啟門市地圖');
    }
  }

  void _ensureSuccess(Map<String, dynamic>? body, String fallback) {
    if (body == null || body['code'] == 0) return;
    throw ServerException((body['message'] ?? fallback).toString());
  }

  Never _throwNetworkError(DioException error, String fallback) {
    HKLog.logE(
      '[StorePickupRepo] ${error.requestOptions.method} '
      '${error.requestOptions.path} status=${error.response?.statusCode} '
      'body=${error.response?.data}',
    );
    if (error.response?.statusCode == 401) {
      throw const UnauthorizedException();
    }
    final data = error.response?.data;
    if (data is Map) {
      throw ServerException((data['message'] ?? fallback).toString());
    }
    throw const NetworkException();
  }
}
