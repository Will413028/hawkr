import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/store_card.pb.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart' as seller;
import 'package:riverpod_annotation/riverpod_annotation.dart';
import '../../../core/networking/connect_provider.dart';
import '../../../core/networking/dio_provider.dart';
import '../../../features/follow/data/follow_repository.dart';

part 'seller_repository.g.dart';

@riverpod
SellerRepository sellerRepository(Ref ref) => SellerRepository(
  ref.watch(dioProvider),
  ref.watch(followRepositoryProvider),
  ref.watch(sellerPublicClientProvider),
  ref.watch(sellerPublicOptionalAuthClientProvider),
);

class SellerRepository {
  /// BatchGetStores 一次最多的店家數。
  static const batchGetStoresLimit = 100;

  final Dio _dio;
  final FollowRepository _follows;
  final SellerPublicServiceClient _public;
  final SellerPublicServiceClient _publicWithSession;
  SellerRepository(
    this._dio,
    this._follows,
    this._public,
    this._publicWithSession,
  );

  /// 店面頁的店家與統計；不存在或不公開時丟 `ConnectException`（not_found）。
  Future<seller.GetStoreResponse> getStore(String storeCode) =>
      _public.getStore(seller.GetStoreRequest(storeCode: storeCode));

  /// 依店家 id 批次讀卡片，回傳以 id 為鍵的表；沒有卡片的店家（從沒公開過、不存在）不在表裡。
  Future<Map<String, StoreCard>> batchGetStores(Iterable<String> ids) async {
    final unique = <String>{
      for (final id in ids)
        if (id.trim().isNotEmpty) id.trim(),
    }.toList();
    final chunks = <List<String>>[
      for (var i = 0; i < unique.length; i += batchGetStoresLimit)
        unique.sublist(
          i,
          i + batchGetStoresLimit > unique.length
              ? unique.length
              : i + batchGetStoresLimit,
        ),
    ];
    final responses = await Future.wait([
      for (final chunk in chunks)
        _public.batchGetStores(seller.BatchGetStoresRequest(ids: chunk)),
    ]);
    return {
      for (final response in responses)
        for (final card in response.stores) card.id: card,
    };
  }

  /// 商品頁的瀏覽 beacon。登入與否由伺服器判斷，失敗不影響頁面。
  Future<void> recordProductView(String productId) async {
    if (productId.trim().isEmpty) return;
    try {
      await _publicWithSession.recordProductView(
        seller.RecordProductViewRequest(productId: productId),
      );
    } catch (_) {}
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
}
