import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/exceptions/app_exception.dart';
import '../../../core/networking/dio_provider.dart';

part 'chat_picker_remote_service.g.dart';

/// Stateless HTTP adapter for the order picker catalog.
class ChatPickerRemoteService {
  const ChatPickerRemoteService(this._dio);

  final Dio _dio;

  Future<List<Map<String, dynamic>>> fetchBuyerOrders(String buyerAccountId) {
    return _fetchRows(
      '/api/v1/app/orders',
      queryParameters: {
        'view': 'seller',
        'buyer_id': buyerAccountId,
        'page_size': '20',
      },
    );
  }

  Future<List<Map<String, dynamic>>> _fetchRows(
    String path, {
    required Map<String, String> queryParameters,
  }) async {
    try {
      final response = await _dio.get<dynamic>(
        path,
        queryParameters: queryParameters,
        options: Options(validateStatus: (_) => true),
      );
      final statusCode = response.statusCode ?? -1;
      final raw = response.data;
      final payload = raw is Map
          ? Map<String, dynamic>.from(raw)
          : <String, dynamic>{'data': raw};

      if (statusCode < 200 || statusCode >= 300) {
        throw AppException.fromHttpResponse(
          statusCode,
          _message(payload, fallback: '請求失敗 ($statusCode)'),
        );
      }
      final rawCode = payload['code'];
      if (rawCode != null && rawCode.toString() != '0') {
        throw AppException.fromApiError(
          int.tryParse(rawCode.toString()) ?? -1,
          _message(payload, fallback: '載入失敗'),
        );
      }

      final data = payload['data'];
      final rows = data is Map && data['records'] is List
          ? data['records'] as List<dynamic>
          : data is List
          ? data
          : const <dynamic>[];
      return List<Map<String, dynamic>>.unmodifiable(
        rows.whereType<Map>().map(Map<String, dynamic>.from),
      );
    } on AppException {
      rethrow;
    } on DioException catch (error) {
      throw AppException.fromDioException(error);
    } catch (error) {
      throw UnknownException(message: '無法載入選擇清單，請稍後再試', originalError: error);
    }
  }

  String _message(Map<String, dynamic> payload, {required String fallback}) {
    return (payload['messageZh'] ?? payload['message'] ?? fallback).toString();
  }
}

@riverpod
ChatPickerRemoteService chatPickerRemoteService(Ref ref) {
  return ChatPickerRemoteService(ref.watch(dioProvider));
}
