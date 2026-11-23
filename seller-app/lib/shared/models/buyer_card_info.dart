import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart' as pb;
import 'package:hawkr_api/images.dart';

/// 賣家契約的 `BuyerCard`（追蹤者名單與封鎖名單共用）：已刪除的買家只有 [id]。
final class BuyerCardInfo {
  const BuyerCardInfo({
    required this.id,
    required this.deleted,
    this.displayName = '',
    this.avatarUrl = '',
  });

  factory BuyerCardInfo.fromProto(pb.BuyerCard b) => BuyerCardInfo(
    id: b.id,
    deleted: b.deleted,
    displayName: b.displayName,
    avatarUrl: b.hasAvatar() && b.avatar.uriTemplate.isNotEmpty
        ? b.avatar.uri(ImagePreset.avatar)
        : '',
  );

  final String id;
  final bool deleted;
  final String displayName;
  final String avatarUrl;
}
