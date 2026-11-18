import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pb.dart' as pb;

/// 被封鎖的一方是店家還是買家；決定確認說明用哪一句（C7a、C7b）。
enum BlockedPartyKind { store, buyer }

/// C7a：封鎖店家的確認說明。
const blockStoreConfirmText = '封鎖後，您將看不到這家店的貼文與討論區，雙方也無法互傳訊息。可隨時在「封鎖名單」解除。';

/// C7b：封鎖買家的確認說明。
const blockBuyerConfirmText = '封鎖後，您與對方將看不到彼此的留言與評價。可隨時在「封鎖名單」解除。';

/// 聊天與評價入口的封鎖說明（舊 REST，A6 前保留）。
const legacyBlockConfirmText = '封鎖後對方將無法傳送訊息給您，您也無法傳送訊息給對方。可隨時在「封鎖名單」解除。';

/// 封鎖的對象。社群的對象（[SocialBlockTarget]）走 `BuyerAppService`；聊天與評價的對象（[LegacyBlockTarget]）
/// 仍走舊 REST（A6 前的允許殘留）。兩者以型別分開，呼叫端不用字串判斷走哪條。
sealed class BlockTarget {
  const BlockTarget();

  /// 確認彈窗的說明文字。
  String get confirmText;
}

sealed class SocialBlockTarget extends BlockTarget {
  const SocialBlockTarget();
}

/// 從貼文封鎖發文的店家；[storeId] 是不透明字串。
final class StoreBlockTarget extends SocialBlockTarget {
  const StoreBlockTarget(this.storeId);

  final String storeId;

  @override
  String get confirmText => blockStoreConfirmText;
}

/// 封鎖一則留言的作者（買家或店家）。
final class CommentAuthorBlockTarget extends SocialBlockTarget {
  const CommentAuthorBlockTarget(this.commentId, this.party);

  final String commentId;
  final BlockedPartyKind party;

  @override
  String get confirmText => party == BlockedPartyKind.store
      ? blockStoreConfirmText
      : blockBuyerConfirmText;
}

/// 封鎖一則討論區留言的作者（買家或店家）。
final class ForumAuthorBlockTarget extends SocialBlockTarget {
  const ForumAuthorBlockTarget(this.entryId, this.party);

  final String entryId;
  final BlockedPartyKind party;

  @override
  String get confirmText => party == BlockedPartyKind.store
      ? blockStoreConfirmText
      : blockBuyerConfirmText;
}

/// 聊天與評價入口：以舊系統的使用者編號封鎖（舊 `POST /app/blocks/{userId}`）。
final class LegacyBlockTarget extends BlockTarget {
  const LegacyBlockTarget(this.userId);

  final String userId;

  @override
  String get confirmText => legacyBlockConfirmText;
}

BlockedPartyKind? _partyOf(pb.Actor author) => switch (author.kind) {
  pb.ActorKind.ACTOR_KIND_STORE => BlockedPartyKind.store,
  pb.ActorKind.ACTOR_KIND_BUYER => BlockedPartyKind.buyer,
  _ => null,
};

/// 留言作者可封鎖時的對象；自己的留言、沒有作者（已刪除或隱藏）或作者身分不明回 null。
SocialBlockTarget? blockTargetForComment(pb.Comment comment) {
  if (comment.isMine || !comment.hasAuthor()) return null;
  final party = _partyOf(comment.author);
  return party == null ? null : CommentAuthorBlockTarget(comment.id, party);
}

/// 討論區留言作者可封鎖時的對象；規則同 [blockTargetForComment]。
SocialBlockTarget? blockTargetForForumEntry(pb.ForumEntry entry) {
  if (entry.isMine || !entry.hasAuthor()) return null;
  final party = _partyOf(entry.author);
  return party == null ? null : ForumAuthorBlockTarget(entry.id, party);
}
