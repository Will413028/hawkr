import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import '../../../core/networking/dio_provider.dart';
import '../../../core/exceptions/app_exception.dart';
import 'after_sale_model.dart';
import 'appeal_data.dart';

part 'after_sale_repository.g.dart';

@riverpod
AfterSaleRepository afterSaleRepository(Ref ref) =>
    AfterSaleRepository(ref.watch(dioProvider));

class AfterSaleRepository {
  final Dio _dio;
  AfterSaleRepository(this._dio);

  /// POST /app/after_sales — 申請售後
  Future<AfterSaleCase> createAfterSale({
    required String orderNo,
    required String reason,
    String? description,
    List<String>? photoTokens,
  }) async {
    try {
      final data = <String, dynamic>{'orderNo': orderNo, 'reason': reason};
      if (description != null && description.isNotEmpty) {
        data['description'] = description;
      }
      if (photoTokens != null && photoTokens.isNotEmpty) {
        data['photoTokens'] = photoTokens;
      }
      final resp = await _dio.post<Map<String, dynamic>>(
        '/app/after_sales',
        data: data,
      );
      final body = resp.data!;
      if (body['code'] != 0) {
        throw ServerException((body['message'] ?? '申請售後失敗').toString());
      }
      return AfterSaleCase.fromJson(
        body['data'] as Map<String, dynamic>? ?? {},
      );
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      if (e.response?.statusCode == 404) {
        throw const ServerException('售後服務功能即將上線，敬請期待');
      }
      final code = e.response?.data?['code'];
      if (code == 9002) throw const ServerException('該訂單已有進行中的售後單');
      if (code == 9003) throw const ServerException('已超過申請時限');
      throw const NetworkException();
    }
  }

  /// GET /app/after_sales — 我的售後列表
  Future<({List<AfterSaleCase> records, int total})> getAfterSaleList({
    int page = 1,
    int pageSize = 20,
    String? status,
  }) async {
    try {
      final params = <String, dynamic>{'page': page, 'page_size': pageSize};
      if (status != null) params['status'] = status;

      final resp = await _dio.get<Map<String, dynamic>>(
        '/app/after_sales',
        queryParameters: params,
      );
      final body = resp.data!;
      if (body['code'] != 0) {
        throw ServerException((body['message'] ?? '查詢售後列表失敗').toString());
      }
      final data = body['data'] as Map<String, dynamic>? ?? {};
      final rawList = data['records'] as List? ?? [];
      final records = rawList
          .whereType<Map>()
          .map((e) => AfterSaleCase.fromJson(Map<String, dynamic>.from(e)))
          .toList();
      final total =
          int.tryParse(data['total']?.toString() ?? '') ?? records.length;
      return (records: records, total: total);
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      // 404 = API 尚未部署，回傳空列表
      if (e.response?.statusCode == 404) {
        return (records: <AfterSaleCase>[], total: 0);
      }
      throw const NetworkException();
    }
  }

  /// GET /app/after_sales/:case_no — 售後詳情
  Future<AfterSaleCase> getAfterSaleDetail(String caseNo) async {
    try {
      final resp = await _dio.get<Map<String, dynamic>>(
        '/app/after_sales/$caseNo',
      );
      final body = resp.data!;
      if (body['code'] != 0) {
        throw ServerException((body['message'] ?? '查詢售後詳情失敗').toString());
      }
      return AfterSaleCase.fromJson(
        body['data'] as Map<String, dynamic>? ?? {},
      );
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      throw const NetworkException();
    }
  }

  /// POST /app/after_sales/:case_no/supplement — 補件
  Future<void> supplementAfterSale(
    String caseNo, {
    List<String>? photoTokens,
    String? additionalDescription,
  }) async {
    try {
      final data = <String, dynamic>{};
      if (photoTokens != null && photoTokens.isNotEmpty) {
        data['photoTokens'] = photoTokens;
      }
      if (additionalDescription != null && additionalDescription.isNotEmpty) {
        data['additionalDescription'] = additionalDescription;
      }
      final resp = await _dio.post<Map<String, dynamic>>(
        '/app/after_sales/$caseNo/supplement',
        data: data,
      );
      final body = resp.data!;
      if (body['code'] != 0) {
        throw ServerException((body['message'] ?? '補件失敗').toString());
      }
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      throw const NetworkException();
    }
  }

  /// POST /app/after_sales/:case_no/cancel — 取消售後
  Future<void> cancelAfterSale(String caseNo) async {
    try {
      final resp = await _dio.post<Map<String, dynamic>>(
        '/app/after_sales/$caseNo/cancel',
      );
      final body = resp.data!;
      if (body['code'] != 0) {
        throw ServerException((body['message'] ?? '取消售後失敗').toString());
      }
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      throw const NetworkException();
    }
  }

  /// POST /app/after_sales/:case_no/appeal — 提交申訴（PR #983）
  ///
  /// 適用於 status=33（待寄回逾時自動關案，72h 內可申訴）
  /// 與 status=60（賣家拒絕）。後端錯誤碼處理：
  /// - `AFTERSALE_APPEAL_NOT_APPEALABLE` → 當前案件狀態不允許申訴
  /// - `AFTERSALE_APPEAL_ALREADY_SUBMITTED` → 您已提交過申訴
  /// - `AFTERSALE_APPEAL_DEADLINE_EXCEEDED` → 申訴期限已過
  /// - `AFTERSALE_CASE_UNDER_APPEAL` → 案件正在處理中，請稍後再試
  Future<void> submitAppeal({
    required String caseNo,
    required String reason,
    String? description,
    List<String>? evidencePhotoTokens,
  }) async {
    try {
      final data = <String, dynamic>{'reason': reason};
      if (description != null && description.isNotEmpty) {
        data['description'] = description;
      }
      if (evidencePhotoTokens != null && evidencePhotoTokens.isNotEmpty) {
        data['evidencePhotoTokens'] = evidencePhotoTokens;
      }
      final resp = await _dio.post<Map<String, dynamic>>(
        '/app/after_sales/$caseNo/appeal',
        data: data,
      );
      final body = resp.data!;
      if (body['code'] != 0) {
        throw ServerException(
          _appealErrorMessage(body['code'], body['message']),
        );
      }
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      final code = e.response?.data?['code'];
      final msg = e.response?.data?['message'];
      if (code != null) {
        throw ServerException(_appealErrorMessage(code, msg));
      }
      throw const NetworkException();
    }
  }

  /// GET /app/after_sales/:case_no/arbitration — 查詢仲裁進度（PR #983）
  Future<ArbitrationData> getArbitration(String caseNo) async {
    try {
      final resp = await _dio.get<Map<String, dynamic>>(
        '/app/after_sales/$caseNo/arbitration',
      );
      final body = resp.data!;
      if (body['code'] != 0) {
        throw ServerException((body['message'] ?? '查詢仲裁進度失敗').toString());
      }
      return ArbitrationData.fromJson(
        body['data'] as Map<String, dynamic>? ?? {},
      );
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      if (e.response?.statusCode == 404) {
        throw const ServerException('查無申訴紀錄');
      }
      throw const NetworkException();
    }
  }

  String _appealErrorMessage(dynamic code, dynamic fallbackMsg) {
    final codeStr = code?.toString() ?? '';
    // 後端錯誤碼可能是純字串 code（透過 envelope message），亦可能是數字 code。
    // 統一以字串比對；若不在已知列表則回傳 fallback。
    if (codeStr.contains('AFTERSALE_APPEAL_NOT_APPEALABLE')) {
      return '當前案件狀態不允許申訴';
    }
    if (codeStr.contains('AFTERSALE_APPEAL_ALREADY_SUBMITTED')) {
      return '您已提交過申訴';
    }
    if (codeStr.contains('AFTERSALE_APPEAL_DEADLINE_EXCEEDED')) {
      return '申訴期限已過';
    }
    if (codeStr.contains('AFTERSALE_CASE_UNDER_APPEAL')) {
      return '案件正在處理中，請稍後再試';
    }
    // 後端可能將錯誤訊息塞在 msg 欄位，含對應的中文錯誤碼
    final msgStr = fallbackMsg?.toString() ?? '';
    if (msgStr.contains('AFTERSALE_APPEAL_NOT_APPEALABLE')) {
      return '當前案件狀態不允許申訴';
    }
    if (msgStr.contains('AFTERSALE_APPEAL_ALREADY_SUBMITTED')) {
      return '您已提交過申訴';
    }
    if (msgStr.contains('AFTERSALE_APPEAL_DEADLINE_EXCEEDED')) {
      return '申訴期限已過';
    }
    if (msgStr.contains('AFTERSALE_CASE_UNDER_APPEAL')) {
      return '案件正在處理中，請稍後再試';
    }
    return msgStr.isNotEmpty ? msgStr : '提交申訴失敗';
  }

  /// POST /app/orders/:order_no/refund_page — 取得 ATM 退款頁面 URL
  Future<String> getRefundPageUrl(String orderNo) async {
    try {
      final resp = await _dio.post<Map<String, dynamic>>(
        '/app/orders/$orderNo/refund_page',
      );
      final body = resp.data!;
      if (body['code'] != 0) {
        throw ServerException((body['message'] ?? '取得退款頁面失敗').toString());
      }
      final data = body['data'] as Map<String, dynamic>? ?? {};
      // 後端 naming 不確定（OpenAPI 寫 redirect_url，實作可能用 redirectUrl）
      final url = (data['redirectUrl'] ?? data['redirect_url'] ?? '')
          .toString();
      if (url.isEmpty) {
        throw const ServerException('後端未回傳 redirect_url');
      }
      return url;
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      throw const NetworkException();
    }
  }
}
