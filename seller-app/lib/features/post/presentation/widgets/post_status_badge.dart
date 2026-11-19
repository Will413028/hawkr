import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pbenum.dart';

import '../../../../shared/widgets/hk_status_badge.dart';
import '../../data/post_model.dart';
import '../../data/post_publish.dart';

/// 貼文頁面上的狀態徽章：[label] 為 null 表示不畫（已發布是常態，缺值是上游沒給）。
///
/// 平台下架與發布狀態互斥，只顯示平台下架（照 website `SellerPostsClient`）；
/// [scheduledAt] 只在「未發布」徽章旁有值。
typedef PostStatusBadgeData = ({
  String? label,
  HkStatusTone tone,
  DateTime? scheduledAt,
});

PostStatusBadgeData postStatusBadgeOf(Post post) {
  if (post.isHidden) {
    return (
      label: kPostPlatformHiddenLabel,
      tone: HkStatusTone.danger,
      scheduledAt: null,
    );
  }
  if (post.status == PostStatus.POST_STATUS_DRAFT) {
    return (
      label: kPostDraftLabel,
      tone: HkStatusTone.neutral,
      scheduledAt: post.scheduledPublishAt,
    );
  }
  if (post.status == PostStatus.POST_STATUS_UNPUBLISHED) {
    return (
      label: kPostUnpublishedLabel,
      tone: HkStatusTone.warning,
      scheduledAt: null,
    );
  }
  return (label: null, tone: HkStatusTone.neutral, scheduledAt: null);
}
