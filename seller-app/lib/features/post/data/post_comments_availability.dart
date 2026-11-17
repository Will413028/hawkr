import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pbenum.dart';

import '../../seller/data/current_store.dart';
import 'post_model.dart';

/// 這則貼文此刻是否公開，也就是買家看得到它：店家能公開、已發布、沒有被平台下架。
/// 不公開時，公開的留言、回覆與即時串流都讀不到它，留言區不呼叫它們，改顯示原因。
enum PostCommentsAvailability {
  available,
  storeNotPublic,
  notPublished,
  platformHidden;

  /// 不可用時留言區顯示的說明；可用時是 null。
  String? get message => switch (this) {
    available => null,
    storeNotPublic => '店家公開後，買家的留言會顯示在這裡。',
    notPublished => '貼文發布後，買家的留言會顯示在這裡。',
    platformHidden => '這則貼文已被平台下架，買家看不到留言。',
  };
}

/// 多個原因同時成立時依序是：平台下架、店家不能公開、未發布（平台下架與發布狀態無關，
/// 且賣家自己的動作解不開）。
///
/// [share] 還沒讀到店家（`StoreShareUnknown`）時不下結論；[status] 未設是「這條路徑的上游沒給」，
/// 同樣不下結論，只有明確不是已發布才算還沒發布。
PostCommentsAvailability postCommentsAvailability({
  required StoreShare share,
  required PostStatus status,
  required bool hidden,
}) {
  if (hidden) return PostCommentsAvailability.platformHidden;
  if (share is StoreShareDisabled) {
    return PostCommentsAvailability.storeNotPublic;
  }
  if (status != PostStatus.POST_STATUS_UNSPECIFIED &&
      status != PostStatus.POST_STATUS_PUBLISHED) {
    return PostCommentsAvailability.notPublished;
  }
  return PostCommentsAvailability.available;
}

/// 貼文本身的狀態是否讓買家看得到：已發布且沒被平台下架，不看店家能不能公開。
/// 分享入口以此決定出不出現（店家還不能公開時入口仍在，只是停用）。
/// [Post.status] 未設時同 [postCommentsAvailability]，不下結論。
bool postIsPublishedAndVisible(Post post) =>
    !post.isHidden &&
    (post.status == PostStatus.POST_STATUS_UNSPECIFIED ||
        post.status == PostStatus.POST_STATUS_PUBLISHED);

/// [Post] 模型版本。
PostCommentsAvailability postCommentsAvailabilityOfPost(
  StoreShare share,
  Post post,
) => postCommentsAvailability(
  share: share,
  status: post.status,
  hidden: post.isHidden,
);
