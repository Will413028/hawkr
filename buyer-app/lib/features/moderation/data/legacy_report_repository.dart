import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/exceptions/app_exception.dart';
import '../../../core/networking/dio_provider.dart';

part 'legacy_report_repository.g.dart';

@riverpod
LegacyReportRepository legacyReportRepository(Ref ref) =>
    LegacyReportRepository(ref.watch(dioProvider));

/// 聊天訊息與評價的檢舉（舊 `POST /app/reports`）。A6 搬進 moderation 前的允許殘留；
/// 社群內容的檢舉在 `ModerationRepository`。
class LegacyReportRepository {
  final Dio _dio;
  LegacyReportRepository(this._dio);

  /// targetType: REVIEW / CHAT_MESSAGE / USER；reasonCode 見 `ReportReasonCode`。
  Future<void> submitReport({
    required String targetType,
    required String targetId,
    required String reasonCode,
    String? description,
  }) async {
    try {
      final resp = await _dio.post<Map<String, dynamic>>(
        '/app/reports',
        data: {
          'targetType': targetType,
          'targetId': targetId,
          'reasonCode': reasonCode,
          if (description != null && description.isNotEmpty)
            'description': description,
        },
      );
      final body = resp.data;
      if (body == null) throw const ServerException('舉報失敗');
      final code = body['code'];
      if (code != null && code != 0) {
        throw ServerException((body['message'] ?? '舉報失敗').toString());
      }
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      if (e.response?.statusCode == 409) {
        throw const ServerException('您已檢舉過此內容，請勿重複送出');
      }
      final data = e.response?.data;
      final msg = data is Map ? data['message']?.toString().trim() : null;
      if (msg != null && msg.isNotEmpty) throw ServerException(msg);
      throw const NetworkException();
    }
  }
}
