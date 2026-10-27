import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart' as seller;
import 'package:hawkr_api/images.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/connect_provider.dart';

part 'subscriber_remote_service.g.dart';

/// Transport DTO for one follower: the `BuyerCard` already carries everything
/// the list shows, so no per-row metadata request is made.
final class SubscriberFollowerDto {
  const SubscriberFollowerDto({
    required this.buyerId,
    required this.subscribedAt,
    this.displayName = '',
    this.avatarUrl = '',
    this.deleted = false,
  });

  final String buyerId;
  final String subscribedAt;
  final String displayName;
  final String avatarUrl;

  /// `BuyerCard.deleted`: the buyer deleted their account; only the id is set.
  final bool deleted;
}

/// Subscriber list uses the seller Connect contract (`ListMyFollowers`).
class SubscriberRemoteService {
  const SubscriberRemoteService(this._seller);

  final SellerAppServiceClient _seller;

  Future<({List<SubscriberFollowerDto> followers, String nextPageToken})>
  fetchFollowers({String pageToken = '', int size = 20}) async {
    if (size < 1 || size > 100) {
      throw const FormatException('invalid follower page size');
    }
    final response = await _seller.listMyFollowers(
      seller.ListMyFollowersRequest(pageSize: size, pageToken: pageToken),
    );
    return (
      followers: List<SubscriberFollowerDto>.unmodifiable(
        response.followers
            .where(
              (follower) => follower.hasBuyer() && follower.buyer.id.isNotEmpty,
            )
            .map(
              (follower) => SubscriberFollowerDto(
                buyerId: follower.buyer.id,
                subscribedAt: follower.hasCreateTime()
                    ? follower.createTime.toDateTime().toUtc().toIso8601String()
                    : '',
                displayName: follower.buyer.displayName,
                avatarUrl: follower.buyer.hasAvatar()
                    ? follower.buyer.avatar.uri(ImagePreset.avatar)
                    : '',
                deleted: follower.buyer.deleted,
              ),
            ),
      ),
      nextPageToken: response.nextPageToken,
    );
  }
}

@riverpod
SubscriberRemoteService subscriberRemoteService(Ref ref) {
  return SubscriberRemoteService(
    SellerAppServiceClient(ref.watch(connectAuthTransportProvider)),
  );
}
