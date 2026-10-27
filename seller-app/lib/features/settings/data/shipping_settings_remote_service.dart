import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/exceptions/app_exception.dart';
import '../../../core/networking/dio_provider.dart';

part 'shipping_settings_remote_service.g.dart';

class ShippingSettingsRemoteService {
  const ShippingSettingsRemoteService(this._dio);

  final Dio _dio;

  Future<List<Map<String, dynamic>>> fetchOnboardings() async {
    final payload = await _getJson('/api/v1/app/shipping_onboardings');
    final data = payload['data'];
    if (data is! List) return const [];
    return _freezeList(data);
  }

  Future<Map<String, dynamic>> sendMutation(
    String path,
    Map<String, dynamic> body,
  ) {
    return _postJson(path, body);
  }

  Future<Map<String, dynamic>> _getJson(String path) async {
    try {
      final response = await _dio.get<dynamic>(
        path,
        options: Options(validateStatus: (_) => true),
      );
      return _validatedPayload(response);
    } on AppException {
      rethrow;
    } on DioException catch (error) {
      throw AppException.fromDioException(error);
    }
  }

  Future<Map<String, dynamic>> _postJson(
    String path,
    Map<String, dynamic> body,
  ) async {
    try {
      final response = await _dio.post<dynamic>(
        path,
        data: body,
        options: Options(validateStatus: (_) => true),
      );
      return _validatedPayload(response);
    } on AppException {
      rethrow;
    } on DioException catch (error) {
      throw AppException.fromDioException(error);
    }
  }

  Map<String, dynamic> _validatedPayload(Response<dynamic> response) {
    final statusCode = response.statusCode ?? -1;
    final raw = response.data;
    final payload = raw is Map
        ? Map<String, dynamic>.from(raw)
        : <String, dynamic>{'data': raw};
    final message = (payload['messageZh'] ?? payload['message'] ?? '物流設定請求失敗')
        .toString();

    if (statusCode < 200 || statusCode >= 300) {
      throw AppException.fromHttpResponse(statusCode, message);
    }
    final rawCode = payload['code'];
    if (rawCode != null && rawCode.toString() != '0') {
      throw AppException.fromApiError(
        int.tryParse(rawCode.toString()) ?? -1,
        message,
      );
    }
    return Map<String, dynamic>.unmodifiable(payload);
  }

  List<Map<String, dynamic>> _freezeList(List<dynamic> source) {
    return List<Map<String, dynamic>>.unmodifiable(
      source.whereType<Map>().map(
        (entry) =>
            Map<String, dynamic>.unmodifiable(Map<String, dynamic>.from(entry)),
      ),
    );
  }
}

@riverpod
ShippingSettingsRemoteService shippingSettingsRemoteService(Ref ref) {
  return ShippingSettingsRemoteService(ref.watch(dioProvider));
}
