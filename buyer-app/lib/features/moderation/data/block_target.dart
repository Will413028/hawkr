import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pb.dart' as pb;

/// 被封鎖的一方是店家還是買家；決定確認說明用哪一句（C7a、C7b）。
enum BlockedPartyKind { store, buyer }

/// C7a：封鎖店家的確認說明。
const blockStoreConfirmText = '封鎖後，您將看不到這家店的貼文與討論區，雙方也無法互傳訊息。可隨時在「封鎖名單」解除。';

/// C7b：封鎖買家的確認說明。
const blockBuyerConfirmText = '封鎖後，您與對方將看不到彼此的留言與評價。可隨時在「封鎖名單」解除。';

/// 封鎖的對象，一律送 `BuyerAppService`：店面、留言與討論區留言的作者、聊天訊息所屬對話的店家。
sealed class BlockTarget {
  const BlockTarget();

  /// 確認彈窗的說明文字。
  String get confirmText;

  /// 封鎖成功後要移除貼文的店家 ID；封鎖的是買家時為 null。
  String? get blockedStoreId;

  /// 封鎖入口所在的那篇貼文；該篇沒有店家資料時也一併移除。
  int? get originPostId => null;
}

/// 從貼文封鎖發文的店家；[storeId] 是不透明字串。
final class StoreBlockTarget extends BlockTarget {
  const StoreBlockTarget(this.storeId, {this.originPostId});

  final String storeId;

  @override
  final int? originPostId;

  @override
  String? get blockedStoreId => storeId;

  @override
  String get confirmText => blockStoreConfirmText;
}

/// 封鎖一則留言的作者（買家或店家）。
final class CommentAuthorBlockTarget extends BlockTarget {
  const CommentAuthorBlockTarget(
    this.commentId,
    this.party, {
    this.authorStoreId,
  });

  final String commentId;
  final BlockedPartyKind party;

  /// 作者是店家時的店家 ID。
  final String? authorStoreId;

  @override
  String? get blockedStoreId =>
      party == BlockedPartyKind.store ? authorStoreId : null;

  @override
  String get confirmText => party == BlockedPartyKind.store
      ? blockStoreConfirmText
      : blockBuyerConfirmText;
}

/// 封鎖一則討論區留言的作者（買家或店家）。
final class ForumAuthorBlockTarget extends BlockTarget {
  const ForumAuthorBlockTarget(this.entryId, this.party, {this.authorStoreId});

  final String entryId;
  final BlockedPartyKind party;

  /// 作者是店家時的店家 ID。
  final String? authorStoreId;

  @override
  String? get blockedStoreId =>
      party == BlockedPartyKind.store ? authorStoreId : null;

  @override
  String get confirmText => party == BlockedPartyKind.store
      ? blockStoreConfirmText
      : blockBuyerConfirmText;
}

/// 封鎖一則聊天訊息所屬對話的店家（送 `BlockAuthor.message_id`；伺服器由訊息推導店家，client 不送對方的帳號）。
/// 訊息已隱藏或抹除仍可當參照。
final class MessageBlockTarget extends BlockTarget {
  const MessageBlockTarget(this.messageId, {this.storeId});

  final String messageId;

  /// 對話的店家 ID；封鎖成功後據以移除已載入的貼文。
  final String? storeId;

  @override
  String? get blockedStoreId => storeId;

  @override
  String get confirmText => blockStoreConfirmText;
}

BlockedPartyKind? _partyOf(pb.Actor author) => switch (author.kind) {
  pb.ActorKind.ACTOR_KIND_STORE => BlockedPartyKind.store,
  pb.ActorKind.ACTOR_KIND_BUYER => BlockedPartyKind.buyer,
  _ => null,
};

/// 留言作者可封鎖時的對象；自己的留言、沒有作者（已刪除或隱藏）或作者身分不明回 null。
BlockTarget? blockTargetForComment(pb.Comment comment) {
  if (comment.isMine || !comment.hasAuthor()) return null;
  final party = _partyOf(comment.author);
  return party == null
      ? null
      : CommentAuthorBlockTarget(
          comment.id,
          party,
          authorStoreId: comment.author.id,
        );
}

/// 討論區留言作者可封鎖時的對象；規則同 [blockTargetForComment]。
BlockTarget? blockTargetForForumEntry(pb.ForumEntry entry) {
  if (entry.isMine || !entry.hasAuthor()) return null;
  final party = _partyOf(entry.author);
  return party == null
      ? null
      : ForumAuthorBlockTarget(entry.id, party, authorStoreId: entry.author.id);
}
