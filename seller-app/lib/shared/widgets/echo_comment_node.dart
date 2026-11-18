/// Echo 留言節點資料模型，用於建立樹狀留言結構。權限取伺服器的 `viewer_can_*`。
class EchoCommentNode {
  EchoCommentNode({
    required this.id,
    required this.etag,
    required this.isMine,
    required this.userName,
    required this.userAvatar,
    required this.content,
    required this.createdAt,
    this.placeholderText,
    this.authorIsBuyer = false,
    this.viewerCanEdit = false,
    this.viewerCanDelete = false,
    this.viewerCanReply = false,
    List<EchoCommentNode>? replies,
  }) : replies = replies ?? [];

  final String id;
  final String etag;
  final bool isMine;
  final String userName;
  final String userAvatar;
  final String content;
  final DateTime? createdAt;

  /// 被刪除或被隱藏的頂層項目顯示的佔位文字；一般項目是 null。
  final String? placeholderText;
  final bool authorIsBuyer;
  final bool viewerCanEdit;
  final bool viewerCanDelete;
  final bool viewerCanReply;
  final List<EchoCommentNode> replies;
}
