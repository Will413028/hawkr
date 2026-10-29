import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pb.dart' as buyer;
import 'package:riverpod_annotation/riverpod_annotation.dart';
import '../../../core/networking/connect_provider.dart';

part 'follow_repository.g.dart';

@riverpod
FollowRepository followRepository(Ref ref) => FollowRepository(
  BuyerAppServiceClient(ref.watch(connectAuthTransportProvider)),
);

class FollowRepository {
  /// 列表每頁筆數（API 預設 20，上限 100）。
  static const listPageSize = 20;

  final BuyerAppServiceClient _buyer;
  FollowRepository(this._buyer);

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
}
