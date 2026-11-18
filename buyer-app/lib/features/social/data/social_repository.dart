import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import '../../../core/networking/dio_provider.dart';
import '../../../core/exceptions/app_exception.dart';
import '../../moderation/data/report_record_model.dart';

part 'social_repository.g.dart';

@riverpod
SocialRepository socialRepository(Ref ref) =>
    SocialRepository(ref.watch(dioProvider));

/// 社群檢舉（`/app/reports`）。貼文、留言與討論區在 `SocialContentRepository`；檢舉改用 moderation 在 4c-3。
class SocialRepository {
  final Dio _dio;
  SocialRepository(this._dio);

  /// 統一檢舉端點 — POST /app/reports
  /// 取代舊版 per-post 端點 `/app/sellers/posts/{postId}/reports`（B-122/#610）。
  /// targetType: REVIEW / POST / POST_COMMENT / SELLER_FORUM / CHAT_MESSAGE / USER
  /// reasonCode: SPAM / SEXUAL / FRAUD / HARASSMENT / FOOD_SAFETY / POLITICAL / FAKE_NEWS / OTHER
  /// evidenceKeys: 裸 objectKey（如 `public/report/{buyerId}/1.png`）。Gateway 會
  /// 剝殼本平台顯示 URL，**外來域名與非 `public/` 前綴一律 400**（10 檔 §25）。
  /// 回傳 reportId。
  Future<String> submitReport({
    required String targetType,
    required String targetId,
    required String reasonCode,
    String? description,
    List<String>? evidenceKeys,
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
          if (evidenceKeys != null && evidenceKeys.isNotEmpty)
            'evidenceKeys': evidenceKeys,
        },
      );
      final body = resp.data;
      if (body == null) throw const ServerException('舉報失敗');
      final code = body['code'];
      if (code != null && code != 0) {
        throw ServerException((body['message'] ?? '舉報失敗').toString());
      }
      final data = body['data'] as Map<String, dynamic>? ?? {};
      return (data['reportId'] ?? '').toString();
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      if (e.response?.statusCode == 409) {
        throw const ServerException('您已檢舉過此內容，請勿重複送出');
      }
      final bizMsg = _dioBizErrorMessage(e);
      if (bizMsg != null) throw ServerException(bizMsg);
      throw const NetworkException();
    }
  }

  /// GET /app/reports/me — 我的檢舉紀錄
  Future<List<ReportRecord>> getMyReports({
    int page = 1,
    int pageSize = 20,
  }) async {
    try {
      final resp = await _dio.get<Map<String, dynamic>>(
        '/app/reports/me',
        queryParameters: {'page': page, 'page_size': pageSize},
      );
      final body = resp.data;
      if (body == null) return [];
      final code = body['code'];
      if (code != null && code != 0) {
        throw ServerException((body['message'] ?? '查詢檢舉紀錄失敗').toString());
      }
      final data = body['data'] as Map<String, dynamic>? ?? {};
      final records = data['records'] as List? ?? [];
      return records
          .whereType<Map>()
          .map((e) => ReportRecord.fromJson(Map<String, dynamic>.from(e)))
          .toList();
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      final bizMsg = _dioBizErrorMessage(e);
      if (bizMsg != null) throw ServerException(bizMsg);
      throw const NetworkException();
    }
  }
}

/// 從 Dio 錯誤回應體讀取後端業務訊息（gateway envelope 統一欄位為 `message`，
/// `msg` 僅作少數舊路徑相容 fallback）。讀不到時回傳 null，
/// 呼叫端 fallback 到通用 [NetworkException]。
String? _dioBizErrorMessage(DioException e) {
  final data = e.response?.data;
  if (data is! Map) return null;
  final msg = data['message']?.toString().trim();
  return (msg != null && msg.isNotEmpty) ? msg : null;
}
