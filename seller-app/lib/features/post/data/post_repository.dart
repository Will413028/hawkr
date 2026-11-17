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
import 'post_model.dart';

part 'post_repository.g.dart';

@Riverpod(keepAlive: true)
PostRepository postRepository(Ref ref) {
  return PostRepository(
    ref.watch(dioProvider),
    ref,
    ref.watch(socialClientProvider),
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
  PostRepository(this._dio, this._ref, this._social);

  final Dio _dio;
  final Ref _ref;
  final SocialAppServiceClient _social;

  @override
  Dio get dio => _dio;

  // ── Path prefixes ──────────────────────────────────────
  static const _pub = '/api/v1/public';
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

  Future<List<Map<String, dynamic>>> listCommentsRaw(int postId) {
    return _listCommentsRaw(postId);
  }

  Future<Map<String, dynamic>?> commentOnPostRaw(
    int postId, {
    required String content,
  }) {
    return _commentOnPostRaw(postId, content: content);
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

  // --- Comment APIs ---

  Future<List<Comment>> listComments(int postId) async {
    try {
      final list = await _listCommentsRaw(postId);
      return list.map((json) => Comment.fromJson(json)).toList();
    } catch (e) {
      HKLog.logE('PostRepository.listComments error', e);
      rethrow;
    }
  }

  Future<List<Map<String, dynamic>>> _listCommentsRaw(int postId) async {
    try {
      final result = await apiGet('$_pub/posts/$postId/comments');
      if (result['code'] == 0) {
        final data = result['data'];
        // API returns paginated format: {records: [...], total, page, pageSize}
        if (data is Map<String, dynamic>) {
          final records = data['records'];
          if (records is List) return List<Map<String, dynamic>>.from(records);
        }
        // Fallback: direct list
        if (data is List) return List<Map<String, dynamic>>.from(data);
      }
    } catch (_) {}
    return [];
  }

  Future<Comment?> commentOnPost(int postId, {required String content}) async {
    try {
      final result = await _commentOnPostRaw(postId, content: content);
      if (result != null) {
        return Comment.fromJson(result);
      }
      return null;
    } catch (e) {
      HKLog.logE('PostRepository.commentOnPost error', e);
      rethrow;
    }
  }

  Future<Map<String, dynamic>?> _commentOnPostRaw(
    int postId, {
    required String content,
  }) async {
    final result = await apiPost(
      '$_app/posts/$postId/comments',
      body: {'content': content},
    );
    if (result['code'] == 0) {
      return result['data'] as Map<String, dynamic>?;
    }
    return null;
  }

  // ── 評論按讚 ───────────────────────────────────────────

  /// Toggle 留言按讚 — POST /app/posts/comments/{id}/like
  Future<bool> likeComment(int commentId) async {
    try {
      final result = await apiPost('$_app/posts/comments/$commentId/like');
      if (result['code'] == 0) {
        return result['data']?['liked'] == true;
      }
    } catch (_) {}
    return false;
  }

  // ── 評論回覆 ───────────────────────────────────────────

  Future<Map<String, dynamic>?> replyToComment(
    int commentId, {
    required int postId,
    required String content,
  }) async {
    final result = await apiPost(
      '$_app/posts/comments/$commentId/replies',
      body: {
        'content': content,
        'postId': postId.toString(),
        'replyToId': commentId.toString(),
        'role': 2,
      },
    );
    if (result['code'] == 0) {
      return result['data'] as Map<String, dynamic>?;
    }
    return null;
  }

  Future<List<Map<String, dynamic>>> listCommentReplies(int commentId) async {
    final result = await apiGet('$_pub/posts/comments/$commentId/replies');
    final data = result['data'];
    // API returns paginated: {records: [...], total, page, pageSize}
    if (data is Map<String, dynamic>) {
      final records = data['records'];
      if (records is List) return List<Map<String, dynamic>>.from(records);
      // fallback: {replies: [...]}
      final replies = data['replies'];
      if (replies is List) return List<Map<String, dynamic>>.from(replies);
    }
    if (data is List) return List<Map<String, dynamic>>.from(data);
    return [];
  }

  Future<bool> deleteComment(int commentId) async {
    final result = await apiDelete('$_app/posts/comments/$commentId');
    return result['code'] == 0;
  }

  // ── 商家訊息 ───────────────────────────────────────────

  Future<List<Map<String, dynamic>>> listSellerMessages(String sellerId) async {
    final result = await apiGet(
      '/api/v1/public/sellers/$sellerId/forum',
      queryParams: {'page': '1', 'page_size': '50'},
    );
    final data = result['data'];
    if (data is Map<String, dynamic>) {
      final records = data['records'];
      if (records is List) return records.cast<Map<String, dynamic>>();
    }
    if (data is List) return data.cast<Map<String, dynamic>>();
    return [];
  }

  Future<List<Map<String, dynamic>>> listMessageReplies(int messageId) async {
    final result = await apiGet(
      '/api/v1/public/sellers/forum/$messageId/replies',
      queryParams: {'page': '1', 'page_size': '50'},
    );
    final data = result['data'];
    if (data is Map<String, dynamic>) {
      final records = data['records'];
      if (records is List) return records.cast<Map<String, dynamic>>();
    }
    if (data is List) return data.cast<Map<String, dynamic>>();
    return [];
  }

  Future<bool> replySellerMessage(int messageId, String content) async {
    final result = await apiPost(
      '$_app/sellers/forum/$messageId/reply',
      body: {'content': content},
    );
    return result['code'] == 0;
  }

  Future<bool> updateSellerReply({
    required int replyId,
    required String content,
  }) async {
    final result = await apiPut(
      '$_app/sellers/forum/$replyId',
      body: {'content': content},
    );
    return result['code'] == 0;
  }

  Future<bool> deleteSellerMessage(int messageId) async {
    final result = await apiDelete('$_app/sellers/forum/$messageId');
    return result['code'] == 0;
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
