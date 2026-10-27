import '../../../../core/codemeta/code_meta.dart';
import '../../../../shared/widgets/hk_status_badge.dart';
import '../../data/post_hidden.dart';
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
  if (post.publishStatus == PostPublishStatus.draft.value) {
    return (
      label: PostPublishStatus.draft.label,
      tone: HkStatusTone.neutral,
      scheduledAt: post.scheduledPublishAt,
    );
  }
  if (post.publishStatus == PostPublishStatus.unpublished.value) {
    return (
      label: PostPublishStatus.unpublished.label,
      tone: HkStatusTone.warning,
      scheduledAt: null,
    );
  }
  return (label: null, tone: HkStatusTone.neutral, scheduledAt: null);
}
