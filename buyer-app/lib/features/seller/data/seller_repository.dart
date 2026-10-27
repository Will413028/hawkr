import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import '../../../core/networking/dio_provider.dart';
import '../../../core/exceptions/app_exception.dart';
import '../../../features/follow/data/follow_repository.dart';
import '../../../features/product/data/product_model.dart';
import 'seller_model.dart';

part 'seller_repository.g.dart';

@riverpod
SellerRepository sellerRepository(Ref ref) => SellerRepository(
  ref.watch(dioProvider),
  ref.watch(followRepositoryProvider),
);

class SellerRepository {
  final Dio _dio;
  final FollowRepository _follows;
  SellerRepository(this._dio, this._follows);

  Future<List<Product>> getSellerProducts(int sellerId) async {
    try {
      final resp = await _dio.get<Map<String, dynamic>>(
        '/public/sellers/$sellerId/products',
      );
      final body = resp.data!;
      if (body['code'] != 0) {
        throw ServerException((body['message'] ?? '請求失敗').toString());
      }
      final data = body['data'];
      final list =
          (data is Map
                  ? (data['records'] as List?) ??
                        (data['items'] as List?) ??
                        (data['list'] as List?) ??
                        (data['rows'] as List?)
                  : data)
              as List? ??
          [];
      return list
          .map((e) => Product.fromJson(e as Map<String, dynamic>))
          .toList();
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      throw const NetworkException();
    }
  }

  /// Get merchant info by string ID (public endpoint, no auth wrapper needed).
  Future<Map<String, dynamic>?> getMerchantInfo(String merchantId) async {
    try {
      final resp = await _dio.get<Map<String, dynamic>>(
        '/public/sellers/$merchantId',
      );
      return resp.data;
    } on DioException catch (e) {
      debugPrint(
        '[SellerRepository] getMerchantInfo($merchantId) failed: ${e.response?.statusCode} ${e.message}',
      );
      return null;
    }
  }

  /// Unfollow a merchant (toggle follow status).
  Future<bool> unfollowMerchant(String merchantId) async {
    return _follows.unfollowMerchant(merchantId);
  }

  Future<RatingStats> getRatingStats(int sellerId) async {
    try {
      final resp = await _dio.get<Map<String, dynamic>>(
        '/public/sellers/$sellerId/rating',
      );
      final body = resp.data!;
      if (body['code'] != 0) {
        throw ServerException((body['message'] ?? '請求失敗').toString());
      }
      return RatingStats.fromJson(body['data'] as Map<String, dynamic>);
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      throw const NetworkException();
    }
  }
}
