/// Echo 留言節點資料模型，用於建立樹狀留言結構。
class EchoCommentNode {
  EchoCommentNode({
    required this.id,
    required this.isMine,
    required this.userName,
    required this.userAvatar,
    required this.content,
    required this.createdAt,
    this.likeCount = 0,
    this.replyCount = 0,
    this.depth = 0,
    this.parentId = '',
    this.rootId = '',
    this.replyToUserName = '',
    List<EchoCommentNode>? replies,
  }) : replies = replies ?? [];

  final String id;
  final bool isMine;
  final String userName;
  final String userAvatar;
  final String content;
  final DateTime createdAt;
  final int likeCount;
  final int replyCount;
  final int depth;
  final String parentId;
  final String rootId;
  final String replyToUserName;
  final List<EchoCommentNode> replies;
}
