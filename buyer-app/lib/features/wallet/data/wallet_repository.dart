import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:hawkr_buyer/shared/utils/hk_log.dart';
import '../../../core/exceptions/app_exception.dart';
import 'wallet_model.dart';
import 'wallet_remote_service.dart';

part 'wallet_repository.g.dart';

@riverpod
WalletRepository walletRepository(Ref ref) =>
    WalletRepository(ref.watch(walletRemoteServiceProvider));

class WalletRepository {
  final WalletRemoteService _service;
  WalletRepository(this._service);

  /// GET /app/coupons
  Future<List<Coupon>> getMyCoupons({int status = 0}) async {
    try {
      final body = await _service.fetchCoupons(status: status);
      HKLog.logD('[getMyCoupons] body: $body');
      if (body is Map && body['code'] != null && body['code'] != 0) {
        throw ServerException((body['message'] ?? '請求失敗').toString());
      }
      final list = _extractCouponList(body);
      return list
          .whereType<Map>()
          .map((e) => Coupon.fromApi(Map<String, dynamic>.from(e)))
          .toList();
    } on AppException {
      rethrow;
    } on Object catch (error) {
      HKLog.logE('[WalletRepository] getMyCoupons failed: $error', error);
      throw const NetworkException();
    }
  }

  /// POST /app/coupons/{template_id}/receive — 領取優惠券
  Future<Map<String, dynamic>> receiveCoupon(String templateId) async {
    try {
      final body = await _service.receiveCoupon(templateId);
      if (body['code'] != 0) {
        throw ServerException((body['message'] ?? '領取失敗').toString());
      }
      return body['data'] as Map<String, dynamic>? ?? {};
    } on AppException {
      rethrow;
    } on Object {
      throw const NetworkException();
    }
  }

  /// GET /app/coupon_templates — 優惠券模板列表
  Future<List<Map<String, dynamic>>> getCouponTemplates({
    int page = 1,
    int pageSize = 20,
  }) async {
    try {
      final body = await _service.fetchCouponTemplates(
        page: page,
        pageSize: pageSize,
      );
      if (body['code'] != 0) {
        throw ServerException((body['message'] ?? '請求失敗').toString());
      }
      final data = body['data'];
      final list = (data is Map ? data['records'] : data) as List? ?? [];
      return list
          .whereType<Map>()
          .map((e) => Map<String, dynamic>.from(e))
          .toList();
    } on AppException {
      rethrow;
    } on Object {
      throw const NetworkException();
    }
  }

  List _extractCouponList(dynamic body) {
    if (body is List) return body;
    if (body is! Map) return const [];
    final data = body['data'];
    if (data is List) return data;
    if (data is Map) {
      final nested = data['records'] ?? data['rows'] ?? data['list'];
      if (nested is List) return nested;
    }
    final fallback = body['rows'] ?? body['list'] ?? body['coupons'];
    return fallback is List ? fallback : const [];
  }
}
