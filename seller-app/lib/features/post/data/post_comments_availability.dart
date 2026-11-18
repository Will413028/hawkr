import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pbenum.dart';

import '../../seller/data/current_store.dart';
import 'post_model.dart';

/// 這則貼文買家此刻看不到的原因（店家不能公開、未發布、被平台下架）。
/// 留言一律讀得到也刪得了，不論貼文是否公開；原因只用來在留言列表上方提醒店主買家看不到這些留言。
enum PostCommentsNotice {
  storeNotPublic,
  notPublished,
  platformHidden;

  String get message => switch (this) {
    storeNotPublic => '店家目前不公開，買家看不到這些留言。',
    notPublished => '這則貼文已下架，買家看不到這些留言。',
    platformHidden => '這則貼文已被平台下架，買家看不到留言。',
  };
}

/// 多個原因同時成立時依序是：平台下架、店家不能公開、未發布（平台下架與發布狀態無關，
/// 且賣家自己的動作解不開）。買家看得到時是 null。
///
/// [share] 還沒讀到店家（`StoreShareUnknown`）時不下結論；[status] 未設是「這條路徑的上游沒給」，
/// 同樣不下結論，只有明確不是已發布才算還沒發布。
PostCommentsNotice? postCommentsNotice({
  required StoreShare share,
  required PostStatus status,
  required bool hidden,
}) {
  if (hidden) return PostCommentsNotice.platformHidden;
  if (share is StoreShareDisabled) return PostCommentsNotice.storeNotPublic;
  if (status != PostStatus.POST_STATUS_UNSPECIFIED &&
      status != PostStatus.POST_STATUS_PUBLISHED) {
    return PostCommentsNotice.notPublished;
  }
  return null;
}

/// 貼文本身的狀態是否讓買家看得到：已發布且沒被平台下架，不看店家能不能公開。
/// 分享入口以此決定出不出現（店家還不能公開時入口仍在，只是停用）。
/// [Post.status] 未設時同 [postCommentsNotice]，不下結論。
bool postIsPublishedAndVisible(Post post) =>
    !post.isHidden &&
    (post.status == PostStatus.POST_STATUS_UNSPECIFIED ||
        post.status == PostStatus.POST_STATUS_PUBLISHED);

/// [Post] 模型版本。
PostCommentsNotice? postCommentsNoticeOfPost(StoreShare share, Post post) =>
    postCommentsNotice(
      share: share,
      status: post.status,
      hidden: post.isHidden,
    );
