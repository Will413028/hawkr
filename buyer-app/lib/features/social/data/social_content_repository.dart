import 'package:hawkr_api/field_mask.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pb.dart' as pb;
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/exceptions/app_exception.dart';
import '../../../core/networking/connect_provider.dart';
import 'social_model.dart';

part 'social_content_repository.g.dart';

@riverpod
SocialContentRepository socialContentRepository(Ref ref) =>
    SocialContentRepository(
      ref.watch(socialPublicClientProvider),
      ref.watch(socialAppClientProvider),
    );

/// 貼文不存在或此刻不公開時詳情頁顯示的說明。
const _postNotFound = '找不到這則貼文';

/// 一頁貼文；[nextPageToken] 空字串表示沒有下一頁。
typedef PostPage = ({List<Post> posts, String nextPageToken});

/// 貼文、留言與討論區（`SocialPublicService`、`SocialAppService`）。留言與討論區的項目直接用生成的訊息，
/// 能不能刪除、回覆、編輯只讀伺服器給的 `viewer_can_*`。
class SocialContentRepository {
  SocialContentRepository(this._public, this._app);

  /// 貼文與留言每頁筆數（API 預設 20，上限 50）。
  static const pageSize = 20;

  final SocialPublicServiceClient _public;
  final SocialAppServiceClient _app;

  /// 貼文的一頁；[storeId] 不給是全站動態牆。
  Future<PostPage> listPosts({
    String? storeId,
    String pageToken = '',
    int size = pageSize,
  }) async {
    final response = await _public.listPosts(
      pb.ListPostsRequest(
        pageSize: size,
        pageToken: pageToken,
        storeId: storeId,
      ),
    );
    return (
      posts: response.posts.map(Post.fromProto).toList(),
      nextPageToken: response.nextPageToken,
    );
  }

  Future<Post> getPost(int postId) async {
    try {
      final response = await _public.getPost(
        pb.GetPostRequest(postId: postId.toString()),
      );
      return Post.fromProto(response.post);
    } on ConnectException catch (e) {
      if (e.code == Code.notFound) throw const NotFoundException(_postNotFound);
      rethrow;
    }
  }

  /// 開貼文詳情時記一次瀏覽。背景呼叫：失敗（含停權）一律吞掉。
  Future<void> recordPostView(int postId) async {
    try {
      await _public.recordPostView(
        pb.RecordPostViewRequest(postId: postId.toString()),
      );
    } catch (_) {
      // 瀏覽計數失敗不影響畫面。
    }
  }

  Future<void> setPostLiked(int postId, {required bool liked}) async {
    final id = postId.toString();
    try {
      if (liked) {
        await _app.likePost(pb.LikePostRequest(postId: id));
      } else {
        await _app.unlikePost(pb.UnlikePostRequest(postId: id));
      }
    } on ConnectException catch (e) {
      if (e.code == Code.notFound) throw const NotFoundException(_postNotFound);
      rethrow;
    }
  }

  Future<void> setCommentLiked(String commentId, {required bool liked}) async {
    if (liked) {
      await _app.likeComment(pb.LikeCommentRequest(commentId: commentId));
    } else {
      await _app.unlikeComment(pb.UnlikeCommentRequest(commentId: commentId));
    }
  }

  /// 貼文的頂層留言；[parentId] 給了是該留言的回覆。
  Future<pb.ListCommentsResponse> listComments(
    String postId, {
    String? parentId,
    String pageToken = '',
  }) => _public.listComments(
    pb.ListCommentsRequest(
      postId: postId,
      parentId: parentId,
      pageToken: pageToken,
      pageSize: 50,
    ),
  );

  Future<pb.Comment> createComment(
    String postId,
    String body, {
    String? parentId,
    String? replyToId,
    required String requestId,
  }) async {
    final response = await _app.createComment(
      pb.CreateCommentRequest(
        postId: postId,
        body: body,
        parentId: parentId,
        replyToId: replyToId,
        requestId: requestId,
      ),
    );
    return response.comment;
  }

  Future<void> deleteComment(pb.Comment comment) async {
    await _app.deleteComment(
      pb.DeleteCommentRequest(commentId: comment.id, etag: comment.etag),
    );
  }

  /// 討論區的一頁；[parentId] 給了是該項目的回覆。回應帶觀看者的身分與能不能發言。
  Future<pb.ListForumEntriesResponse> listForumEntries(
    String storeId, {
    String? parentId,
    String pageToken = '',
  }) => _public.listForumEntries(
    pb.ListForumEntriesRequest(
      storeId: storeId,
      parentId: parentId,
      pageToken: pageToken,
      pageSize: 50,
    ),
  );

  Future<pb.ForumEntry> createForumEntry(
    String storeId,
    String body, {
    String? parentId,
    String? replyToId,
    required String requestId,
  }) async {
    final response = await _app.createForumEntry(
      pb.CreateForumEntryRequest(
        storeId: storeId,
        body: body,
        parentId: parentId,
        replyToId: replyToId,
        requestId: requestId,
      ),
    );
    return response.entry;
  }

  Future<pb.ForumEntry> updateForumEntry(
    pb.ForumEntry entry,
    String body,
  ) async {
    final edited = pb.ForumEntry(id: entry.id, etag: entry.etag, body: body);
    final response = await _app.updateForumEntry(
      pb.UpdateForumEntryRequest(
        entry: edited,
        updateMask: fieldMask(edited, ['body']),
      ),
    );
    return response.entry;
  }

  Future<void> deleteForumEntry(pb.ForumEntry entry) async {
    await _app.deleteForumEntry(
      pb.DeleteForumEntryRequest(entryId: entry.id, etag: entry.etag),
    );
  }
}
