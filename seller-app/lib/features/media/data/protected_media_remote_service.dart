import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/exceptions/app_exception.dart';
import '../../../core/networking/dio_provider.dart';

part 'protected_media_remote_service.g.dart';

/// Stateless adapter for authenticated/public byte downloads.
class ProtectedMediaRemoteService {
  const ProtectedMediaRemoteService(this._dio);

  final Dio _dio;

  Future<List<int>> fetchBytes(String url) async {
    try {
      final response = await _dio.get<List<int>>(
        url,
        options: Options(
          responseType: ResponseType.bytes,
          validateStatus: (_) => true,
        ),
      );
      final statusCode = response.statusCode ?? -1;
      if (statusCode < 200 || statusCode >= 300) {
        throw AppException.fromHttpResponse(statusCode, '媒體下載失敗 ($statusCode)');
      }
      final bytes = response.data;
      if (bytes == null || bytes.isEmpty) {
        throw const UnknownException(message: '媒體內容為空');
      }
      return List<int>.unmodifiable(bytes);
    } on AppException {
      rethrow;
    } on DioException catch (error) {
      throw AppException.fromDioException(error);
    } catch (error) {
      throw UnknownException(message: '媒體下載失敗', originalError: error);
    }
  }
}

@riverpod
ProtectedMediaRemoteService protectedMediaRemoteService(Ref ref) {
  return ProtectedMediaRemoteService(ref.watch(dioProvider));
}
