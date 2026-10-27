import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/exceptions/app_exception.dart';
import '../../../core/networking/api_request_helper.dart';
import '../../../core/networking/dio_provider.dart';
import 'report_model.dart';

part 'report_repository.g.dart';

@riverpod
ReportRepository reportRepository(Ref ref) {
  return ReportRepository(ref.watch(dioProvider));
}

/// 檢舉 Repository（backend-payuni#645）。
class ReportRepository with ApiRequestHelper {
  ReportRepository(this._dio);
  final Dio _dio;

  @override
  Dio get dio => _dio;

  static const _base = '/api/v1/app/reports';

  /// POST /api/v1/app/reports
  ///
  /// [targetType] 合法值見 [ReportTargetType]，[reasonCode] 見
  /// [ReportReasonCode]——兩者的值域都是生成的 CodeMeta。
  ///
  /// [evidenceKeys] 只收裸 objectKey 且必須是 `public/` 前綴（10 檔 §25）。
  /// Gateway 會剝殼本平台顯示 URL，但外來域名與非 `public/` 前綴一律回 400
  /// （backend#2216 C3；欄位名從 `evidence_urls` 收斂成 `evidenceKeys`，
  /// 因為值一直都是 key，舊名在說謊）。
  Future<({String reportId, String status})> submitReport({
    required String targetType,
    required String targetId,
    required String reasonCode,
    String? description,
    List<String>? evidenceKeys,
  }) async {
    final body = <String, dynamic>{
      'targetType': targetType,
      'targetId': targetId,
      'reasonCode': reasonCode,
    };
    if (description != null && description.trim().isNotEmpty) {
      body['description'] = description.trim();
    }
    if (evidenceKeys != null && evidenceKeys.isNotEmpty) {
      body['evidenceKeys'] = evidenceKeys;
    }

    final result = await apiPost(_base, body: body);
    if (result['code'] == 0) {
      final data = result['data'];
      if (data is Map<String, dynamic>) {
        return (
          reportId: (data['reportId'] ?? '').toString(),
          status: (data['status'] ?? '').toString(),
        );
      }
      return (reportId: '', status: '');
    }
    throw AppException.fromApiError(
      result['code'] as int,
      (result['message'] ?? '檢舉送出失敗').toString(),
    );
  }

  /// GET /api/v1/app/reports/me?page=&page_size=
  Future<List<ReportRecord>> listMyReports({
    int page = 1,
    int pageSize = 20,
  }) async {
    final result = await apiGet(
      '$_base/me',
      queryParams: {'page': '$page', 'page_size': '$pageSize'},
    );
    if (result['code'] == 0) {
      final data = result['data'];
      final records = data is Map<String, dynamic> ? data['records'] : null;
      if (records is List) {
        return records
            .whereType<Map<String, dynamic>>()
            .map(ReportRecord.fromJson)
            .toList();
      }
      return [];
    }
    throw AppException.fromApiError(
      result['code'] as int,
      (result['message'] ?? '取得檢舉紀錄失敗').toString(),
    );
  }
}
