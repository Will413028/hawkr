import 'package:hawkr_seller/features/customer/data/subscriber_remote_service.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart' as seller;
import 'package:hawkr_api/testing.dart';
import 'package:flutter_test/flutter_test.dart';

/// 粉絲清單走 Connect（`SellerAppService.ListMyFollowers`）：分頁游標與空清單的行為。
void main() {
  group('粉絲清單分頁', () {
    test('粉絲清單使用 SellerAppService 的 generated client', () async {
      final service = SubscriberRemoteService(
        SellerAppServiceClient(
          FakeTransportBuilder()
              .unary(
                SellerAppService.listMyFollowers,
                (_, _) => seller.ListMyFollowersResponse(
                  followers: [
                    seller.Follower(buyer: seller.BuyerCard(id: '2636840881')),
                  ],
                ),
              )
              .build(),
        ),
      );

      final followers = await service.fetchFollowers();

      expect(followers.followers, hasLength(1), reason: '讀錯 key 會靜默回空清單');
      expect(followers.followers.single.buyerId, '2636840881');
      expect(followers.followers.single.subscribedAt, '');
    });

    test('沒有追蹤者時不讀 REST envelope', () async {
      final service = SubscriberRemoteService(
        SellerAppServiceClient(
          FakeTransportBuilder()
              .unary(
                SellerAppService.listMyFollowers,
                (_, _) => seller.ListMyFollowersResponse(),
              )
              .build(),
        ),
      );
      final followers = await service.fetchFollowers();
      expect(followers.followers, isEmpty);
    });

    test('粉絲下一頁直接使用上一頁的 cursor', () async {
      final tokens = <String>[];
      final service = SubscriberRemoteService(
        SellerAppServiceClient(
          FakeTransportBuilder().unary(SellerAppService.listMyFollowers, (
            request,
            _,
          ) {
            tokens.add(request.pageToken);
            return seller.ListMyFollowersResponse(
              nextPageToken: request.pageToken.isEmpty ? 'cursor-2' : '',
              followers: [
                seller.Follower(
                  buyer: seller.BuyerCard(
                    id: request.pageToken.isEmpty ? '1' : '2',
                  ),
                ),
              ],
            );
          }).build(),
        ),
      );
      final first = await service.fetchFollowers(size: 1);
      final second = await service.fetchFollowers(
        pageToken: first.nextPageToken,
        size: 1,
      );
      expect(tokens, ['', 'cursor-2']);
      expect(second.followers.single.buyerId, '2');
    });
  });
}
