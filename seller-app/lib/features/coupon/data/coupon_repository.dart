import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/exceptions/app_exception.dart';
import '../../../core/networking/dio_provider.dart';
import '../../../shared/utils/hk_log.dart';
import 'coupon_model.dart';

part 'coupon_repository.g.dart';

/// Repository for coupon template CRUD operations.
///
/// Endpoints based on OpenAPI spec `/app/coupon_templates`.
class CouponRepository {
  CouponRepository(this._dio);

  final Dio _dio;

  static const _basePath = '/api/v1/app/coupon_templates';

  // ---------------------------------------------------------------------------
  // List
  // ---------------------------------------------------------------------------

  /// GET /app/coupon_templates — 取得賣家優惠券模板列表
  Future<List<Coupon>> getSellerCoupons() async {
    try {
      final resp = await _dio.get(_basePath);
      final data = resp.data;
      if (data is Map<String, dynamic>) {
        final inner = data['data'];
        // Handle both formats:
        // 1. data: [...]  (direct array)
        // 2. data: { records: [...] }  (paginated)
        List? list;
        if (inner is List) {
          list = inner;
        } else if (inner is Map<String, dynamic>) {
          list = inner['records'] as List?;
        }
        if (list != null) {
          return list
              .whereType<Map<String, dynamic>>()
              .map(_parseCoupon)
              .toList();
        }
      }
      return [];
    } on DioException catch (e) {
      throw AppException.fromDioException(e);
    }
  }

  // ---------------------------------------------------------------------------
  // Detail
  // ---------------------------------------------------------------------------

  /// GET /app/coupon_templates/{template_id} — 取得優惠券模板詳情
  ///
  /// 回傳券本體加上已領取 / 已使用統計。優惠券模板無「更新」端點，
  /// 此端點僅供唯讀詳情頁使用。
  Future<CouponTemplateDetail> getCouponDetail(String templateId) async {
    try {
      final resp = await _dio.get('$_basePath/$templateId');
      final data = resp.data;
      if (data is Map<String, dynamic>) {
        final inner = data['data'];
        if (inner is Map<String, dynamic>) {
          final tmpl = inner['template'];
          final coupon = tmpl is Map<String, dynamic>
              ? _parseCoupon(tmpl)
              : _parseCoupon(inner);
          return CouponTemplateDetail(
            template: coupon,
            reservedCount:
                int.tryParse(inner['reservedCount']?.toString() ?? '') ?? 0,
            usedCount: int.tryParse(inner['usedCount']?.toString() ?? '') ?? 0,
          );
        }
      }
      throw const UnknownException(message: '載入優惠券詳情失敗');
    } on DioException catch (e) {
      HKLog.logD(
        '[Coupon] getCouponDetail error: '
        '${e.response?.statusCode} ${e.response?.data}',
      );
      throw AppException.fromDioException(e);
    }
  }

  // ---------------------------------------------------------------------------
  // Create
  // ---------------------------------------------------------------------------

  /// POST /app/coupon_templates — 建立並發行優惠券
  ///
  /// Returns a [Coupon] with templateId populated.
  Future<Coupon> createCoupon(Map<String, dynamic> body) async {
    try {
      final resp = await _dio.post(_basePath, data: body);
      final data = resp.data;
      if (data is Map<String, dynamic> && data['code'] == 0) {
        final inner = data['data'];
        if (inner is Map<String, dynamic>) {
          return _parseCoupon(inner);
        }
      }
      final msg = (data is Map<String, dynamic>)
          ? data['message']?.toString()
          : null;
      throw UnknownException(message: msg ?? '建立優惠券失敗');
    } on DioException catch (e) {
      throw _handleCreateError(e);
    }
  }

  /// 建立優惠券的錯誤處理：提取 API 回傳的 message
  AppException _handleCreateError(DioException e) {
    final statusCode = e.response?.statusCode;
    final body = e.response?.data;
    final apiMsg = (body is Map<String, dynamic>)
        ? body['message']?.toString()
        : null;

    if (statusCode == 400) {
      return UnknownException(message: apiMsg ?? '請求參數錯誤');
    }
    if (statusCode == 403) {
      return UnknownException(message: apiMsg ?? '無權限執行此操作');
    }
    if (statusCode == 409) {
      return UnknownException(message: apiMsg ?? '無權限執行此操作');
    }
    return AppException.fromDioException(e);
  }

  // ---------------------------------------------------------------------------
  // Revoke
  // ---------------------------------------------------------------------------

  /// POST /app/coupon_templates/{id}/revoke — 撤銷優惠券模板
  ///
  /// [reason] 為可選參數，不傳則不附帶 reason 欄位。
  Future<bool> deleteCoupon(String templateId, {String? reason}) async {
    if (templateId.isEmpty) return false;
    try {
      final Map<String, dynamic> body = {};
      if (reason != null && reason.isNotEmpty) body['reason'] = reason;

      final resp = await _dio.post(
        '$_basePath/$templateId/revoke',
        data: body.isEmpty ? null : body,
        options: Options(validateStatus: (_) => true),
      );
      final data = resp.data;
      if (data is Map<String, dynamic>) {
        final code = data['code'];
        if (code == 0 || code == '0') return true;
        final msg = data['message']?.toString();
        throw UnknownException(message: msg ?? '撤銷優惠券失敗');
      }
      return resp.statusCode == 200;
    } on DioException catch (e) {
      HKLog.logD(
        '[Coupon] deleteCoupon error: ${e.response?.statusCode} ${e.response?.data}',
      );
      throw AppException.fromDioException(e);
    }
  }

  // ---------------------------------------------------------------------------
  // Customer Lookup
  // ---------------------------------------------------------------------------

  static const _customerLookupPath = '/api/v1/app/sellers/me/customers/lookup';

  /// 依手機號碼查詢客戶
  Future<CustomerLookupResult> lookupByPhone(String phone) async {
    try {
      final resp = await _dio.get(
        _customerLookupPath,
        queryParameters: {'phone': phone},
      );
      return _parseCustomerLookup(resp.data);
    } on DioException catch (e) {
      HKLog.logD(
        '[Coupon] lookupByPhone error: ${e.response?.statusCode} ${e.response?.data}',
      );
      throw _handleLookupError(e);
    }
  }

  /// 從 DioException 提取 API 錯誤訊息，回傳具描述性的 AppException
  AppException _handleLookupError(DioException e) {
    final statusCode = e.response?.statusCode;

    if (statusCode == 404) {
      return const UnknownException(message: '未找到該用戶');
    }
    if (statusCode == 400) {
      return const UnknownException(message: '查詢參數錯誤，請確認輸入');
    }
    return AppException.fromDioException(e);
  }

  CustomerLookupResult _parseCustomerLookup(dynamic data) {
    if (data is Map<String, dynamic> && data['code'] == 0) {
      final inner = data['data'];
      if (inner is Map<String, dynamic>) {
        return CustomerLookupResult.fromJson(inner);
      }
    }
    final msg = (data is Map<String, dynamic>)
        ? data['message']?.toString()
        : null;
    throw UnknownException(message: msg ?? '查無此用戶');
  }

  // ---------------------------------------------------------------------------
  // Helpers
  // ---------------------------------------------------------------------------

  /// Parse API response (CouponTemplateResponse) into [Coupon].
  ///
  /// type 統一對應 [CouponType.code]：
  ///   數字 1 / 字串 '1' / 'CASH'            → 'CASH'
  ///   數字 2 / 字串 '2' / 'PERCENT'          → 'PERCENT'
  ///   舊版字串 'DISCOUNT' → 'PERCENT'
  Coupon _parseCoupon(Map<String, dynamic> m) {
    final rawType = m['type'];
    final String type = _normalizeType(rawType);

    final templateId = (m['templateId'] ?? m['template_id'] ?? m['id'] ?? '')
        .toString();
    return Coupon(
      templateId: templateId,
      instanceId: (m['instanceId'] ?? m['instance_id'] ?? '').toString(),
      title: (m['title'] ?? m['name'] ?? '').toString(),
      type: type,
      status: _normalizeStatus((m['status'] ?? '').toString()),
      discountAmount:
          m['discountAmount']?.toString() ?? m['discount_amount']?.toString(),
      discountPercent:
          m['discountPercent']?.toString() ?? m['discount_percent']?.toString(),
      thresholdAmount:
          m['thresholdAmount']?.toString() ??
          m['threshold_amount']?.toString() ??
          m['min_amount']?.toString(),
      maxDiscountAmount: m['maxDiscountAmount']?.toString(),
      startAt: m['startAt']?.toString() ?? m['start_at']?.toString(),
      endAt:
          m['endAt']?.toString() ??
          m['end_at']?.toString() ??
          m['end_time']?.toString(),
      issuedCount: m['issuedCount']?.toString(),
      totalIssueLimit: m['totalIssueLimit']?.toString(),
      usageLimitPerAccount: m['usageLimitPerAccount']?.toString(),
      issuerAccountId: m['issuerAccountId']?.toString(),
      issuerType: m['issuerType']?.toString(),
      applicableStoreId: m['applicableStoreId']?.toString(),
      sellerName: m['sellerName']?.toString(),
      createdAt: m['createdAt']?.toString(),
    );
  }

  /// 將 API 回傳的 type 值統一轉為 [CouponType.code]
  static String _normalizeType(dynamic raw) {
    if (raw == null) return 'CASH';
    final s = raw.toString().toUpperCase();
    return switch (s) {
      '1' || 'CASH' => 'CASH',
      '2' || 'PERCENT' => 'PERCENT',
      // 舊版相容
      'DISCOUNT' => 'PERCENT',
      _ => s,
    };
  }

  /// 將 status 數字值統一轉為大寫 code
  ///
  /// 實例狀態：0=未使用, 1=已鎖定, 2=已使用, 3=已過期, 4=已收回, 5=已作廢
  static String _normalizeStatus(String raw) {
    final upper = raw.toUpperCase();
    if (upper == 'UNUSED' ||
        upper == 'LOCKED' ||
        upper == 'USED' ||
        upper == 'EXPIRED' ||
        upper == 'REVOKED' ||
        upper == 'ANNUL') {
      return upper;
    }
    return switch (raw) {
      '0' => 'UNUSED',
      '1' => 'LOCKED',
      '2' => 'USED',
      '3' => 'EXPIRED',
      '4' => 'REVOKED',
      '5' => 'ANNUL',
      _ => raw.toUpperCase(),
    };
  }
}

@riverpod
CouponRepository couponRepository(Ref ref) {
  return CouponRepository(ref.watch(dioProvider));
}
