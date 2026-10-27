import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/api_request_helper.dart';
import '../../../core/networking/dio_provider.dart';
import '../../../shared/utils/hk_log.dart';
import '../../../shared/utils/product_image_resolver.dart';
import '../../../shared/widgets/share_bottom_sheet.dart';
import '../../media/data/media_repository.dart';
import 'post_model.dart';

part 'post_repository.g.dart';

@Riverpod(keepAlive: true)
PostRepository postRepository(Ref ref) {
  return PostRepository(ref.watch(dioProvider), ref);
}

/// 貼文分頁查詢結果：這頁的貼文 + 是否還有下一頁（供 caller 決定要不要
/// 繼續觸發下一次載入，不下載超過目前需要顯示的量）。
class PostPageResult {
  const PostPageResult({required this.posts, required this.hasMore});

  final List<Map<String, dynamic>> posts;
  final bool hasMore;
}

class PostRepository with ApiRequestHelper {
  PostRepository(this._dio, this._ref);

  final Dio _dio;
  final Ref _ref;

  @override
  Dio get dio => _dio;

  // ── Path prefixes ──────────────────────────────────────
  static const _pub = '/api/v1/public';
  static const _app = '/api/v1/app';

  int? _parsePostId(dynamic rawId) {
    if (rawId is int) return rawId;
    if (rawId is num) return rawId.toInt();
    if (rawId is String) return int.tryParse(rawId);
    if (rawId == null) return null;
    return int.tryParse(rawId.toString());
  }

  static String _str(dynamic v) => (v ?? '').toString().trim();

  /// 取第一個非空值（後端「欄位在、值為空字串」的形狀用 `??` 補不到，見
  /// `_enrichSellerPostJson` 的說明）。
  static String _firstNonEmpty(List<dynamic> candidates) {
    for (final c in candidates) {
      final s = _str(c);
      if (s.isNotEmpty) return s;
    }
    return '';
  }

  /// 後端關聯商品有**兩種欄位形狀**（2026-08-05 實打 dev 確認，見 `Post._readLinkedProduct`）：
  /// 列表端點（`/app/sellers/me/posts`）回單數物件
  /// `product`；詳情 / 建立 / 更新回陣列 `products`。只認 `products` 的話，
  /// 賣家自己頁面的貼文列表永遠拿不到關聯商品，卡片整個不渲染。
  List<dynamic>? _rawLinkedProducts(Map<String, dynamic> json) {
    final products = json['products'];
    if (products is List && products.isNotEmpty) return products;
    final single = json['product'];
    if (single is Map) return <dynamic>[single];
    final direct = json['linkedProduct'];
    if (direct is Map) return <dynamic>[direct];
    return null;
  }

  Map<String, dynamic> _attachLinkedProduct(Map<String, dynamic> json) {
    final products = _rawLinkedProducts(json);
    if (products == null || products.first is! Map) return json;
    final direct = json['linkedProduct'];
    return <String, dynamic>{
      ...json,
      'linkedProduct': <String, dynamic>{
        if (direct is Map) ...Map<String, dynamic>.from(direct),
        ...Map<String, dynamic>.from(products.first as Map),
      },
    };
  }

  Future<Map<String, dynamic>> _enrichSellerPostJson(
    Map<String, dynamic> source,
  ) async {
    var json = Map<String, dynamic>.from(source);
    final int? postId = _parsePostId(json['id'] ?? json['postId']);
    if (postId == null) {
      HKLog.logI(
        'PostRepository: skip seller post enrichment, id not parseable: ${json['id'] ?? json['postId']}',
      );
      return _attachLinkedProduct(json);
    }

    try {
      final directLinkedProduct = json['linkedProduct'];
      final products = _rawLinkedProducts(json);
      if (products != null && products.isNotEmpty) {
        final enrichedProducts = await Future.wait(
          products.map((rawProduct) async {
            if (rawProduct is! Map) return rawProduct;

            final product = Map<String, dynamic>.from(rawProduct);
            // `PostProductData` 的欄位只有 id / name / price / imageKey / imageUrl
            // （實打 dev 確認）。建立與更新回應會給 id、其餘欄位**回空字串**
            // （不是缺欄位）—— 所以補齊條件一律用「非空」判斷，不能用 `??`
            // （`'' ?? x` 仍是 `''`，補不到東西）。
            final String productId = _str(product['id']);
            final bool hasName = _str(product['name']).isNotEmpty;
            final bool hasPrice = _str(product['price']).isNotEmpty;
            final bool hasImage =
                extractPrimaryProductImageKey(product) != null;

            if (productId.isEmpty || (hasName && hasPrice && hasImage)) {
              return product;
            }

            try {
              final prodDetail = await _getProductDetailRaw(productId);
              if (prodDetail == null) return product;

              return <String, dynamic>{
                ...product,
                'id': productId,
                'name': _firstNonEmpty([product['name'], prodDetail['name']]),
                'price': _firstNonEmpty([
                  product['price'],
                  prodDetail['priceMin'],
                  prodDetail['price'],
                ]),
                // 回填成後端 `PostProductData` 的欄位名與型別（objectKey，不是 URL；
                // 49 §6.1 跨層只帶 key），顯示端因此只需要認得同一個欄位。
                'imageKey': _firstNonEmpty([
                  product['imageKey'],
                  extractPrimaryProductImageKey(prodDetail),
                ]),
              };
            } catch (_) {
              return product;
            }
          }),
        );
        json['products'] = enrichedProducts;
        if (enrichedProducts.isNotEmpty && enrichedProducts.first is Map) {
          json['linkedProduct'] = <String, dynamic>{
            if (directLinkedProduct is Map)
              ...Map<String, dynamic>.from(directLinkedProduct),
            ...Map<String, dynamic>.from(enrichedProducts.first as Map),
          };
        }
      }
    } catch (e, st) {
      HKLog.logE(
        'PostRepository: seller post enrichment failed for $postId',
        e,
        st,
      );
    }

    return _attachLinkedProduct(json);
  }

  Future<List<Map<String, dynamic>>> _enrichSellerPostList(
    List<Map<String, dynamic>> list,
  ) {
    return Future.wait(list.map(_enrichSellerPostJson));
  }

  /// 全螢幕貼文 feed（story-viewer）用：一次回傳登入賣家**全部**貼文（含被平台下架的）
  /// 供左右滑動瀏覽。後端 page_size 上限 50（見 gateway `maxPostPageSize`），因此逐頁
  /// 迴圈到 hasMore=false 累積齊全，單次頁面請求數 = ceil(貼文數/50)，不是每篇貼文一次。
  Future<List<Post>> listMyPosts() async {
    try {
      const pageSize = 50;
      final all = <Map<String, dynamic>>[];
      var page = 1;
      while (true) {
        final raw = await _listMyPostsPage(page: page, pageSize: pageSize);
        all.addAll(raw.posts);
        if (!raw.hasMore || raw.posts.isEmpty) break;
        page++;
      }
      final enriched = await _enrichSellerPostList(all);
      return enriched.map((json) => Post.fromJson(json)).toList();
    } catch (e) {
      HKLog.logE('PostRepository.listMyPosts error', e);
      rethrow;
    }
  }

  Future<Map<String, dynamic>?> _getProductDetailRaw(dynamic productId) async {
    final result = await apiGet('$_app/products/$productId');
    if (result['code'] == 0 && result['data'] is Map<String, dynamic>) {
      return result['data'] as Map<String, dynamic>;
    }
    return null;
  }

  /// 自己頁面用：回傳當前登入賣家的貼文（**含被平台下架的**），走 authed
  /// `/app/sellers/me/posts`（ListMyPosts，預設 status='' → published + hidden）。
  Future<PostPageResult> listMyPostsRaw({
    int page = 1,
    int pageSize = 20,
  }) async {
    final raw = await _listMyPostsPage(page: page, pageSize: pageSize);
    final enriched = await _enrichSellerPostList(raw.posts);
    return PostPageResult(posts: enriched, hasMore: raw.hasMore);
  }

  Future<PostPageResult> _listMyPostsPage({
    required int page,
    required int pageSize,
  }) async {
    final result = await apiGet(
      '$_app/sellers/me/posts',
      queryParams: {'page': '$page', 'page_size': '$pageSize'},
    );
    return _parsePostPage(result, page: page, pageSize: pageSize);
  }

  /// 解析 gateway 分頁 envelope（`data: {records, total, page, pageSize}`），
  /// 算出是否還有下一頁；純資料解析，不做 enrichment。
  PostPageResult _parsePostPage(
    Map<String, dynamic> result, {
    required int page,
    required int pageSize,
  }) {
    if (result['code'] != 0) {
      return const PostPageResult(posts: [], hasMore: false);
    }
    final data = result['data'];
    List<Map<String, dynamic>> posts = const [];
    int? total;
    if (data is List) {
      posts = List<Map<String, dynamic>>.from(data);
    } else if (data is Map) {
      final records = data['records'] ?? data['list'];
      if (records is List) posts = List<Map<String, dynamic>>.from(records);
      total = int.tryParse('${data['total'] ?? ''}');
    }
    final hasMore = total != null
        ? page * pageSize < total
        : posts.length >= pageSize;
    return PostPageResult(posts: posts, hasMore: hasMore);
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

  /// 賣家把自己的貼文轉為已發布（`CapMarketing`）。失敗一律丟 [AppException]。
  Future<void> publishPostOrThrow(int postId) async {
    await apiPostOrThrow('$_app/sellers/posts/$postId/publish');
  }

  /// 賣家把自己已發布的貼文轉為已下架（`CapContentContract`）。失敗一律丟 [AppException]。
  Future<void> unpublishPostOrThrow(int postId) async {
    await apiPostOrThrow('$_app/sellers/posts/$postId/unpublish');
  }

  Future<void> deletePost(int postId) async {
    await apiDeleteOrThrow('$_app/sellers/posts/$postId');
  }

  Future<Post?> createPost({
    required String sellerId,
    required String title,
    String? content,
    String? contentType,
    String? imageUrl,
    dynamic linkedProductId,
    List<String>? tags,
  }) async {
    final productId = _str(linkedProductId);
    final body = <String, dynamic>{
      if (title.isNotEmpty) 'title': title,
      'content': content?.isNotEmpty == true ? content : title,
      if (contentType != null) 'content_type': contentType,
      if (imageUrl != null && imageUrl.isNotEmpty) 'stagedToken': imageUrl,
      if (productId.isNotEmpty) 'productId': productId,
      if (tags != null) 'tags': tags,
    };
    final result = await apiPost('$_app/sellers/posts', body: body);
    final data = result['code'] == 0 ? result['data'] : null;
    if (data is Map<String, dynamic> && data['id'] != null) {
      return Post.fromJson(data);
    }
    return null;
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

  /// [keepAssetId] 非空表示保留現有圖片；[newStagedToken] 非空表示換新圖；兩者皆空則清除圖片
  /// （契約鍵 `keepAssetId` / `newStagedToken`，backend PR #2235 起為 camelCase）
  Future<Post?> updatePost(
    int postId, {
    required String content,
    String keepAssetId = '',
    String newStagedToken = '',
    int? productId,
    List<String>? tags,
  }) async {
    final body = <String, dynamic>{
      'content': content,
      'keepAssetId': keepAssetId,
      'newStagedToken': newStagedToken,
      if (productId != null) 'productId': productId.toString(),
      if (tags != null) 'tags': tags,
    };
    final result = await apiPut('$_app/sellers/posts/$postId', body: body);
    final data = result['code'] == 0 ? result['data'] : null;
    if (data is Map<String, dynamic> && data['id'] != null) {
      return Post.fromJson(data);
    }
    return null;
  }
}
