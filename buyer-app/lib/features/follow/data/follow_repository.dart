import 'package:dio/dio.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pb.dart' as buyer;
import 'package:riverpod_annotation/riverpod_annotation.dart';
import '../../../core/networking/dio_provider.dart';
import '../../../core/networking/connect_provider.dart';
import '../../../core/exceptions/app_exception.dart';
import 'follow_model.dart';

part 'follow_repository.g.dart';

@riverpod
FollowRepository followRepository(Ref ref) => FollowRepository(
  ref.watch(dioProvider),
  BuyerAppServiceClient(ref.watch(connectAuthTransportProvider)),
);

class FollowRepository {
  /// 列表每頁筆數（API 預設 20，上限 100）。
  static const listPageSize = 20;

  final Dio _dio;
  final BuyerAppServiceClient _buyer;
  FollowRepository(this._dio, this._buyer);

  /// 追蹤店家的一頁；[pageToken] 空字串是第一頁，回應的 `nextPageToken` 空字串表示沒有下一頁。
  Future<buyer.ListFollowsResponse> listFollows({
    String pageToken = '',
    int pageSize = listPageSize,
  }) => _buyer.listFollows(
    buyer.ListFollowsRequest(pageSize: pageSize, pageToken: pageToken),
  );

  Future<void> follow(int merchantId) async {
    await _buyer.followStore(
      buyer.FollowStoreRequest(storeId: merchantId.toString()),
    );
  }

  Future<void> unfollow(int merchantId) async {
    await _buyer.unfollowStore(
      buyer.UnfollowStoreRequest(storeId: merchantId.toString()),
    );
  }

  Future<bool> checkFollow(String storeId) async {
    final response = await _buyer.checkFollow(
      buyer.CheckFollowRequest(storeId: storeId),
    );
    return response.following;
  }

  /// Unfollow a merchant by string ID (for legacy screens).
  Future<bool> unfollowMerchant(String merchantId) async {
    try {
      final mid = int.tryParse(merchantId);
      if (mid == null) return false;
      await unfollow(mid);
      return true;
    } catch (_) {
      return false;
    }
  }

  Future<FansCount> getFansCount(int merchantId) async {
    try {
      final resp = await _dio.get<Map<String, dynamic>>(
        '/public/sellers/$merchantId/stats',
      );
      final body = resp.data!;
      if (body['code'] != 0) {
        throw ServerException((body['message'] ?? '請求失敗').toString());
      }
      final data = body['data'] as Map<String, dynamic>? ?? {};
      return FansCount(
        merchantId: merchantId,
        count: (data['followers'] as num?)?.toInt() ?? 0,
      );
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      throw const NetworkException();
    }
  }
}
