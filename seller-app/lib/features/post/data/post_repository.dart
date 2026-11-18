import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:hawkr_api/field_mask.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pb.dart' as pb;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/api_request_helper.dart';
import '../../../core/networking/connect_provider.dart';
import '../../../core/networking/dio_provider.dart';
import '../../../shared/utils/hk_log.dart';
import '../../../shared/widgets/share_bottom_sheet.dart';
import '../../media/data/media_repository.dart';
import 'post_comment.dart';
import 'post_model.dart';

part 'post_repository.g.dart';

@Riverpod(keepAlive: true)
PostRepository postRepository(Ref ref) {
  return PostRepository(
    ref.watch(dioProvider),
    ref,
    ref.watch(socialClientProvider),
    ref.watch(socialPublicClientProvider),
  );
}

/// 貼文分頁查詢結果：這頁的貼文與下一頁的 token（空字串是沒有下一頁）。
class PostPageResult {
  const PostPageResult({
    required this.posts,
    required this.nextPageToken,
    this.schedules = const {},
  });

  final List<Post> posts;
  final String nextPageToken;

  /// 這次載入用到的待發布排程；翻下一頁時原樣帶回 `listMyPostsPage(schedules:)`，不再重讀。
  final Map<int, DateTime> schedules;

  bool get hasMore => nextPageToken.isNotEmpty;
}

class PostRepository with ApiRequestHelper {
  PostRepository(this._dio, this._ref, this._social, this._socialPublic);

  final Dio _dio;
  final Ref _ref;
  final SocialAppServiceClient _social;
  final SocialPublicServiceClient _socialPublic;

  @override
  Dio get dio => _dio;

  // ── Path prefixes ──────────────────────────────────────
  static const _app = '/api/v1/app';

  static const _maxPageSize = 50;

  /// 全螢幕貼文 feed 用：逐頁讀到沒有下一頁，回傳登入賣家**全部**貼文（含草稿、下架與被平台下架的）。
  Future<List<Post>> listMyPosts() async {
    final all = <Post>[];
    var token = '';
    Map<int, DateTime>? schedules;
    do {
      final page = await listMyPostsPage(
        pageToken: token,
        pageSize: _maxPageSize,
        schedules: schedules,
      );
      schedules = page.schedules;
      all.addAll(page.posts);
      token = page.nextPageToken;
    } while (token.isNotEmpty);
    return all;
  }

  /// 自己頁面用：一頁貼文，排程時間已併入。[schedules] 是同一次載入前一頁拿到的排程
  /// （[PostPageResult.schedules]）；沒給時與這頁貼文同時讀一次。
  Future<PostPageResult> listMyPostsPage({
    String pageToken = '',
    int pageSize = 20,
    Map<int, DateTime>? schedules,
  }) async {
    final responseFuture = _social.listMyPosts(
      pb.ListMyPostsRequest(pageSize: pageSize, pageToken: pageToken),
    );
    final timesFuture = schedules != null
        ? Future.value(schedules)
        : _pendingScheduleTimes();
    final results = await Future.wait<Object>([
      responseFuture,
      timesFuture,
    ], eagerError: true);
    final response = results[0] as pb.ListMyPostsResponse;
    final times = results[1] as Map<int, DateTime>;
    return PostPageResult(
      posts: [
        for (final post in response.posts)
          _withSchedule(postFromProto(post), times),
      ],
      nextPageToken: response.nextPageToken,
      schedules: times,
    );
  }

  /// 讀一篇自己的貼文（列表沒有這篇時取得目前的 etag）；排程時間不併入。
  Future<Post> getMyPost(int postId) async {
    final response = await _social.getMyPost(
      pb.GetMyPostRequest(postId: postId.toString()),
    );
    return postFromProto(response.post);
  }

  Post _withSchedule(Post post, Map<int, DateTime> schedules) =>
      post.copyWith(scheduledPublishAt: schedules[post.id]);

  Future<Map<int, DateTime>> _pendingScheduleTimes() async {
    final times = <int, DateTime>{};
    var token = '';
    do {
      final response = await _social.listPendingSchedules(
        pb.ListPendingSchedulesRequest(
          pageSize: _maxPageSize,
          pageToken: token,
        ),
      );
      for (final schedule in response.schedules) {
        final postId = int.tryParse(schedule.postId);
        if (postId == null || !schedule.hasPublishTime()) continue;
        times[postId] = schedule.publishTime.toDateTime();
      }
      token = response.nextPageToken;
    } while (token.isNotEmpty);
    return times;
  }

  /// 發布（`CAP_MARKETING`）。[etag] 是讀到該貼文時的版本；失敗丟 `ConnectException`。
  Future<void> publishPostOrThrow(int postId, {required String etag}) async {
    await _social.publishPost(
      pb.PublishPostRequest(postId: postId.toString(), etag: etag),
    );
  }

  /// 下架（`CAP_CONTENT_CONTRACT`，店家停權時仍可）。失敗丟 `ConnectException`。
  Future<void> unpublishPostOrThrow(int postId, {required String etag}) async {
    await _social.unpublishPost(
      pb.UnpublishPostRequest(postId: postId.toString(), etag: etag),
    );
  }

  Future<void> deletePost(int postId) async {
    await _social.deletePost(pb.DeletePostRequest(postId: postId.toString()));
  }

  /// 建立草稿並發布。[imageStagedToken] 是 media 上傳完成後拿到的 token。
  Future<Post> createPost({
    required String content,
    String? imageStagedToken,
    String? productId,
    List<String> hashtags = const [],
  }) async {
    final response = await _social.createPost(
      pb.CreatePostRequest(
        body: content,
        imageStagedToken: imageStagedToken,
        productId: productId,
        hashtagInputs: hashtags,
        publish: true,
      ),
    );
    return postFromProto(response.post);
  }

  Future<String?> uploadMedia({
    required String sellerId,
    required Uint8List bytes,
    required String filename,
    required String mimeType,
  }) async {
    try {
      HKLog.logD('[PostRepo] UPLOAD filename=$filename size=${bytes.length}');
      final normalizedMimeType = mimeType.split(';').first.trim().toLowerCase();
      final result = await _ref
          .read(mediaRepositoryProvider)
          .uploadWithSession(
            bytes: bytes,
            filename: filename,
            compress: normalizedMimeType != 'image/gif',
          );
      return result.stagedToken;
    } catch (e) {
      HKLog.logD('[PostRepo] UPLOAD error: $e');
      return null;
    }
  }

  // --- 貼文留言（SocialPublicService 讀、SocialAppService 寫） ---

  /// 讀貼文的頂層留言，或 [parentId] 那則頂層留言的回覆；逐頁讀到沒有下一頁。
  /// 店主讀自己的貼文不受公開狀態限制，所以草稿、下架與店家不公開時也讀得到。
  Future<List<PostComment>> listComments(int postId, {String? parentId}) async {
    final all = <PostComment>[];
    var token = '';
    do {
      final response = await _socialPublic.listComments(
        pb.ListCommentsRequest(
          postId: postId.toString(),
          pageSize: _maxPageSize,
          pageToken: token,
          parentId: parentId,
        ),
      );
      all.addAll(response.comments.map(PostComment.fromProto));
      token = response.nextPageToken;
    } while (token.isNotEmpty);
    return all;
  }

  /// 留言；[parentId] 設了是回覆（只限貼文所屬的店），[replyToId] 是回覆針對的那一則。
  Future<PostComment> createComment(
    int postId, {
    required String body,
    String? parentId,
    String? replyToId,
  }) async {
    final response = await _social.createComment(
      pb.CreateCommentRequest(
        postId: postId.toString(),
        body: body,
        parentId: parentId,
        replyToId: replyToId,
      ),
    );
    return PostComment.fromProto(response.comment);
  }

  Future<void> likeComment(String commentId) async {
    await _social.likeComment(pb.LikeCommentRequest(commentId: commentId));
  }

  Future<void> unlikeComment(String commentId) async {
    await _social.unlikeComment(pb.UnlikeCommentRequest(commentId: commentId));
  }

  /// 刪除留言；[etag] 與目前版本不符丟 `ConnectException`（aborted）。
  Future<void> deleteComment(String commentId, {required String etag}) async {
    await _social.deleteComment(
      pb.DeleteCommentRequest(commentId: commentId, etag: etag),
    );
  }

  // ── 店家討論區 ─────────────────────────────────────────

  /// 讀店家討論區的頂層發言，或 [parentId] 那則的回覆；逐頁讀到沒有下一頁。
  Future<List<StoreForumEntry>> listForumEntries(
    String storeId, {
    String? parentId,
  }) async {
    final all = <StoreForumEntry>[];
    var token = '';
    do {
      final response = await _socialPublic.listForumEntries(
        pb.ListForumEntriesRequest(
          storeId: storeId,
          pageSize: _maxPageSize,
          pageToken: token,
          parentId: parentId,
        ),
      );
      all.addAll(response.entries.map(StoreForumEntry.fromProto));
      token = response.nextPageToken;
    } while (token.isNotEmpty);
    return all;
  }

  /// 在店家討論區回覆 [parentId]（父項不是頂層時伺服器掛到它所屬的頂層底下）。
  Future<StoreForumEntry> replyForumEntry(
    String storeId, {
    required String parentId,
    required String body,
  }) async {
    final response = await _social.createForumEntry(
      pb.CreateForumEntryRequest(
        storeId: storeId,
        body: body,
        parentId: parentId,
      ),
    );
    return StoreForumEntry.fromProto(response.entry);
  }

  /// 修改自己的發言內文；[etag] 與目前版本不符丟 `ConnectException`（aborted）。
  Future<StoreForumEntry> updateForumEntry(
    String entryId, {
    required String etag,
    required String body,
  }) async {
    final entry = pb.ForumEntry(id: entryId, etag: etag, body: body);
    final response = await _social.updateForumEntry(
      pb.UpdateForumEntryRequest(
        entry: entry,
        updateMask: fieldMask(entry, ['body']),
      ),
    );
    return StoreForumEntry.fromProto(response.entry);
  }

  /// 刪除發言；[etag] 與目前版本不符丟 `ConnectException`（aborted）。
  Future<void> deleteForumEntry(String entryId, {required String etag}) async {
    await _social.deleteForumEntry(
      pb.DeleteForumEntryRequest(entryId: entryId, etag: etag),
    );
  }

  /// 取得分享連結 — GET /api/v1/app/share_url?type={type}&id={id}
  Future<ShareData?> getShareUrl({
    required String type,
    required String id,
  }) async {
    final result = await apiGet(
      '$_app/share_url',
      queryParams: {'type': type, 'id': id},
    );
    if (result['code'] == 0 && result['data'] is Map<String, dynamic>) {
      final d = result['data'] as Map<String, dynamic>;
      return ShareData(
        title: (d['title'] ?? '').toString(),
        description: (d['description'] ?? '').toString(),
        imageUrl: (d['imageUrl'] ?? '').toString(),
        url: (d['url'] ?? '').toString(),
      );
    }
    return null;
  }

  // ── 貼文更新 ───────────────────────────────────────────

  /// 修改貼文內文、商品與標籤；[imageStagedToken] 是 `null` 時保留現有圖片，非 null 時換成該 token
  /// （空字串是清掉圖）。[updateProduct] 為 true 時才改關聯商品，[productId] 是 `null` 即清掉商品；
  /// 為 false 時 [productId] 被忽略、商品不變。[etag] 與目前版本不符丟 `ConnectException`（aborted）。
  Future<Post> updatePost(
    int postId, {
    required String etag,
    required String content,
    String? imageStagedToken,
    String? productId,
    bool updateProduct = false,
    List<String> hashtags = const [],
  }) async {
    final post = pb.Post(
      id: postId.toString(),
      etag: etag,
      body: content,
      hashtagInputs: hashtags,
    );
    final paths = ['body', 'hashtag_inputs'];
    if (updateProduct) {
      if (productId != null) post.productId = productId;
      paths.add('product_id');
    }
    if (imageStagedToken != null) {
      post.imageStagedToken = imageStagedToken;
      paths.add('image_staged_token');
    }
    final response = await _social.updatePost(
      pb.UpdatePostRequest(post: post, updateMask: fieldMask(post, paths)),
    );
    return postFromProto(response.post);
  }
}
