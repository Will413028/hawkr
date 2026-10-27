import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/dio_provider.dart';

part 'wallet_remote_service.g.dart';

@riverpod
WalletRemoteService walletRemoteService(Ref ref) {
  return DioWalletRemoteService(ref.watch(dioProvider));
}

abstract interface class WalletRemoteService {
  Future<dynamic> fetchCoupons({required int status});

  Future<Map<String, dynamic>> receiveCoupon(String templateId);

  Future<Map<String, dynamic>> fetchCouponTemplates({
    required int page,
    required int pageSize,
  });
}

final class DioWalletRemoteService implements WalletRemoteService {
  const DioWalletRemoteService(this._dio);

  final Dio _dio;

  @override
  Future<dynamic> fetchCoupons({required int status}) async {
    final response = await _dio.get<dynamic>(
      '/app/coupons',
      queryParameters: {'status': status},
    );
    return response.data;
  }

  @override
  Future<Map<String, dynamic>> receiveCoupon(String templateId) async {
    // Gateway 路由是 /claim（`routes_coupon.go` CouponsClaim = "/:template_id/claim"）
    final response = await _dio.post<Map<String, dynamic>>(
      '/app/coupons/$templateId/claim',
    );
    return response.data ?? const {};
  }

  @override
  Future<Map<String, dynamic>> fetchCouponTemplates({
    required int page,
    required int pageSize,
  }) async {
    final response = await _dio.get<Map<String, dynamic>>(
      '/app/coupon_templates',
      queryParameters: {
        'page': page.toString(),
        'page_size': pageSize.toString(),
      },
    );
    return response.data ?? const {};
  }
}
