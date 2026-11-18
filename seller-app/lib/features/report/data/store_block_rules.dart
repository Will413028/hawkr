/// 店家能不能封鎖這則貼文留言的作者：別人寫的、而伺服器允許觀看者刪（代表它在這家店的內容裡）。
bool canBlockCommentAuthor({
  required bool isMine,
  required bool viewerCanDelete,
}) => !isMine && viewerCanDelete;

/// 討論區發言：同一條規則，再加上作者必須是買家（店家自己的發言不是封鎖對象）。
bool canBlockForumAuthor({
  required bool isMine,
  required bool viewerCanDelete,
  required bool authorIsBuyer,
}) =>
    authorIsBuyer &&
    canBlockCommentAuthor(isMine: isMine, viewerCanDelete: viewerCanDelete);
