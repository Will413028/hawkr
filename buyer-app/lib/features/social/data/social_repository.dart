import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import '../../../core/networking/dio_provider.dart';
import '../../../core/exceptions/app_exception.dart';
import '../../moderation/data/report_record_model.dart';
import '../../seller/data/seller_repository.dart';
import '../../seller/data/store_card_display.dart';
import 'social_model.dart';

part 'social_repository.g.dart';

@riverpod
SocialRepository socialRepository(Ref ref) => SocialRepository(
  ref.watch(dioProvider),
  sellers: ref.watch(sellerRepositoryProvider),
);

/// 貼文不存在或此刻不公開時詳情頁顯示的說明。
const _postNotFound = '找不到這則貼文';

class SocialRepository {
  final Dio _dio;

  /// 補貼文的店家名稱與頭像。
  final SellerRepository _sellers;
  SocialRepository(this._dio, {required SellerRepository sellers})
    : _sellers = sellers;

  Future<List<Post>> getPosts({
    int? merchantId,
    int page = 1,
    int pageSize = 20,
  }) async {
    try {
      final String path = merchantId != null
          ? '/public/sellers/$merchantId/posts'
          : '/public/posts';
      debugPrint('[SocialRepo] getPosts path=$path page=$page size=$pageSize');
      final resp = await _dio.get<dynamic>(
        path,
        queryParameters: {'page': page, 'page_size': pageSize},
      );
      final data = resp.data;
      debugPrint('[SocialRepo] resp type=${data.runtimeType}');
      List rawList;
      if (data is List) {
        rawList = data;
      } else if (data is Map) {
        final code = data['code'];
        if (code != null && code != 0) {
          throw ServerException((data['message'] ?? '請求失敗').toString());
        }
        final inner = data['data'];
        debugPrint('[SocialRepo] inner type=${inner.runtimeType}');
        if (inner is List) {
          rawList = inner;
        } else if (inner is Map) {
          final records =
              inner['records'] ??
              inner['rows'] ??
              inner['list'] ??
              inner['items'];
          debugPrint('[SocialRepo] records type=${records.runtimeType}');
          rawList = (records is List) ? records : [];
        } else {
          rawList = [];
        }
      } else {
        rawList = [];
      }
      debugPrint('[SocialRepo] rawList length=${rawList.length}');
      for (final raw in rawList) {
        if (raw is Map) _normalizeRelatedProduct(raw);
      }
      // Hydrate missing data in parallel
      await Future.wait([_hydrateProducts(rawList), _hydrateSellers(rawList)]);
      return rawList
          .whereType<Map<String, dynamic>>()
          .map((e) => Post.fromJson(e))
          .toList();
    } catch (e, st) {
      debugPrint('[SocialRepo] ERROR: $e');
      debugPrint('[SocialRepo] STACK: $st');
      if (e is DioException) {
        if (e.response?.statusCode == 401) throw const UnauthorizedException();
        throw const NetworkException();
      }
      rethrow;
    }
  }

  /// Workaround: posts API returns product with only id but empty name/image/price.
  /// Fetch product details in parallel to fill in missing data.
  Future<void> _hydrateProducts(List rawList) async {
    final futures = <Future>[];
    for (int i = 0; i < rawList.length; i++) {
      if (rawList[i] is! Map) continue;
      final post = rawList[i] as Map;
      final product = post['product'];
      if (product is! Map) continue;
      final pid = product['id'];
      final name = (product['name'] ?? '').toString().trim();
      if (pid == null || pid == 0 || name.isNotEmpty) continue;
      futures.add(
        _fetchProductInfo(pid.toString()).then((info) {
          if (info != null) {
            post['product'] = {
              ...Map<String, dynamic>.from(product),
              'name': info['name'] ?? '',
              'image': info['image'] ?? '',
              'price': info['price'] ?? '',
            };
          }
        }),
      );
    }
    if (futures.isNotEmpty) {
      await Future.wait(futures);
    }
  }

  /// Hydrate seller name/logo when posts API only returns sellerId.
  Future<void> _hydrateSellers(List rawList) async {
    // Collect unique seller IDs that need hydration
    final Map<String, List<Map>> sellerPosts = {};
    for (final raw in rawList) {
      if (raw is! Map) continue;
      final name = (raw['sellerName'] ?? '').toString().trim();
      if (name.isNotEmpty) continue;
      final sid = raw['sellerId'];
      if (sid == null || sid == 0) continue;
      final key = sid.toString();
      sellerPosts.putIfAbsent(key, () => []).add(raw);
    }
    if (sellerPosts.isEmpty) return;
    try {
      // 店家名稱與頭像一律走店家卡片（BatchGetStores，上限內合批）。
      final cards = await _sellers.batchGetStores(sellerPosts.keys);
      for (final MapEntry(:key, :value) in sellerPosts.entries) {
        final card = cards[key];
        if (card == null) continue;
        final sellerName = card.hasName() ? card.name.trim() : '';
        final logo = card.logoUri;
        for (final post in value) {
          if (sellerName.isNotEmpty) post['sellerName'] = sellerName;
          if (logo.isNotEmpty) post['logoUrl'] = logo;
        }
      }
    } catch (e) {
      debugPrint('[SocialRepo] _hydrateSellers error: $e');
    }
  }

  Future<Map<String, dynamic>?> _fetchProductInfo(String productId) async {
    try {
      final resp = await _dio.get<dynamic>('/public/products/$productId');
      final data = resp.data;
      if (data is Map) {
        final inner = data['data'] ?? data;
        if (inner is Map) {
          final name = (inner['name'] ?? '').toString();
          // 49 §6.1 成對契約：商品詳情吐 imageKey（objectKey）＋ imageUrl。
          final image = (inner['imageKey'] ?? inner['imageUrl'] ?? '')
              .toString();
          final priceMin = inner['priceMin'];
          return {
            'name': name,
            'image': image,
            'price': priceMin?.toString() ?? '',
          };
        }
      }
    } catch (e) {
      debugPrint('[SocialRepo] _fetchProductInfo($productId) error: $e');
    }
    return null;
  }

  Future<Post> getPostDetail(int postId) async {
    try {
      final resp = await _dio.get<Map<String, dynamic>>(
        '/public/posts/$postId',
      );
      final body = resp.data!;
      if (body['code'] == 404) throw const NotFoundException(_postNotFound);
      if (body['code'] != 0) {
        throw ServerException((body['message'] ?? '請求失敗').toString());
      }
      // 列表與詳情端點的貼文圖形狀已一致：都只有 imageKeys / imageUrls（複數，
      // 49 §6.1）。Post.imageUrl 由 model 端的 getter 從複數欄導出，這裡不再需要
      // 把單數欄補回去。
      final data = Map<String, dynamic>.from(body['data'] as Map);
      // 詳細端點目前回 `products: [...]`，列表端點則回 `product`。
      // Post model 以單一關聯商品呈現，統一取第一筆，避免詳細頁漏掉商品 chip。
      if (data['product'] is! Map) {
        final products = data['products'];
        if (products is List && products.isNotEmpty && products.first is Map) {
          data['product'] = Map<String, dynamic>.from(products.first as Map);
        }
      }
      _normalizeRelatedProduct(data);
      return Post.fromJson(data);
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      if (e.response?.statusCode == 404) {
        throw const NotFoundException(_postNotFound);
      }
      throw const NetworkException();
    }
  }

  /// 貼文關聯商品（PostProductData）的圖片欄是成對的 imageKey ＋ imageUrl
  /// （49 §6.1）；展示層讀的是單一 `image`，這裡把它折成 key 優先的單值。
  /// `image` 也可能已由 [_hydrateProducts] 補上（走 /public/products/{id}）。
  void _normalizeRelatedProduct(Map post) {
    final rawProduct = post['product'];
    if (rawProduct is! Map) return;

    final product = Map<String, dynamic>.from(rawProduct);
    final image = [product['image'], product['imageKey'], product['imageUrl']]
        .map((value) => value?.toString().trim() ?? '')
        .firstWhere((value) => value.isNotEmpty, orElse: () => '');
    if (image.isNotEmpty) product['image'] = image;
    post['product'] = product;
  }

  Future<Map<String, dynamic>?> likePost(int postId) async {
    try {
      final resp = await _dio.post<Map<String, dynamic>>(
        '/app/posts/$postId/like',
      );
      final body = resp.data!;
      if (body['code'] != 0) {
        throw ServerException((body['message'] ?? '操作失敗').toString());
      }
      return body;
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      throw const NetworkException();
    }
  }

  Future<List<PostComment>> getComments(int postId) async {
    try {
      final resp = await _dio.get<Map<String, dynamic>>(
        '/public/posts/$postId/comments',
      );
      final body = resp.data!;
      if (body['code'] == 404) throw const NotFoundException(_postNotFound);
      if (body['code'] != 0) {
        throw ServerException((body['message'] ?? '請求失敗').toString());
      }
      // 後端 response: data 可能是 List（舊格式）或 {records: [...], page, total}（分頁格式）
      final data = body['data'];
      final list = data is Map
          ? (data['records'] as List? ?? const [])
          : (data as List? ?? const []);
      return list
          .map((e) => PostComment.fromJson(e as Map<String, dynamic>))
          .toList();
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      if (e.response?.statusCode == 404) {
        throw const NotFoundException(_postNotFound);
      }
      throw const NetworkException();
    }
  }

  Future<PostComment> addComment(int postId, String content) async {
    try {
      final resp = await _dio.post<Map<String, dynamic>>(
        '/app/posts/$postId/comments',
        data: {'content': content},
      );
      final body = resp.data!;
      if (body['code'] != 0) {
        throw ServerException((body['message'] ?? '留言失敗').toString());
      }
      return PostComment.fromJson(body['data'] as Map<String, dynamic>);
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      throw const NetworkException();
    }
  }

  Future<void> likeComment(int commentId) async {
    try {
      final resp = await _dio.post<Map<String, dynamic>>(
        '/app/posts/comments/$commentId/like',
      );
      final body = resp.data!;
      if (body['code'] != 0) {
        throw ServerException((body['message'] ?? '操作失敗').toString());
      }
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      throw const NetworkException();
    }
  }

  /// 刪除留言 — DELETE /app/posts/comments/{id}
  /// 留言本人或貼文賣家可操作（軟刪除）
  Future<void> deleteComment(String commentId) async {
    try {
      final resp = await _dio.delete<Map<String, dynamic>>(
        '/app/posts/comments/$commentId',
      );
      final body = resp.data;
      if (body == null) return;
      final code = body['code'];
      if (code != null && code != 0) {
        throw ServerException((body['message'] ?? '刪除失敗').toString());
      }
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      throw const NetworkException();
    }
  }

  Future<List<Map<String, dynamic>>> getCommentReplies(int commentId) async {
    try {
      final resp = await _dio.get<dynamic>(
        '/public/posts/comments/$commentId/replies',
      );
      final data = resp.data;
      if (data is List) {
        return data
            .whereType<Map>()
            .map((e) => Map<String, dynamic>.from(e))
            .toList();
      }
      if (data is Map) {
        for (final k in ['data', 'rows', 'list', 'items']) {
          final v = data[k];
          if (v is List) {
            return v
                .whereType<Map>()
                .map((e) => Map<String, dynamic>.from(e))
                .toList();
          }
        }
      }
    } on DioException {
      // fall through
    }
    return [];
  }

  Future<Map<String, dynamic>?> replyToComment(
    int commentId,
    String content,
  ) async {
    try {
      final resp = await _dio.post<dynamic>(
        '/app/posts/comments/$commentId/replies',
        data: {'content': content},
      );
      final data = resp.data;
      if (data is Map<String, dynamic>) return data;
      if (data is Map) return Map<String, dynamic>.from(data);
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      throw const NetworkException();
    }
    return null;
  }

  // ---------------------------------------------------------------------------
  // Raw-response variants (return Map/List instead of Freezed models)
  // Used by widgets that work extensively with Map<String, dynamic>.
  // ---------------------------------------------------------------------------

  Future<List<Map<String, dynamic>>> getCommentsRaw(String postId) async {
    try {
      final resp = await _dio.get<dynamic>('/public/posts/$postId/comments');
      final data = resp.data;
      if (data is List) {
        return data
            .whereType<Map>()
            .map((e) => Map<String, dynamic>.from(e))
            .toList();
      }
      if (data is Map) {
        // Handle { code, data: { records: [...] } }
        final inner = data['data'];
        if (inner is Map) {
          final records = inner['records'];
          if (records is List) {
            return records
                .whereType<Map>()
                .map((e) => Map<String, dynamic>.from(e))
                .toList();
          }
        }
        if (inner is List) {
          return inner
              .whereType<Map>()
              .map((e) => Map<String, dynamic>.from(e))
              .toList();
        }
        for (final k in ['records', 'rows', 'list', 'items']) {
          final v = data[k];
          if (v is List) {
            return v
                .whereType<Map>()
                .map((e) => Map<String, dynamic>.from(e))
                .toList();
          }
        }
      }
    } on DioException {
      // fall through
    }
    return [];
  }

  Future<Map<String, dynamic>?> addCommentRaw(
    String postId,
    String content, {
    String? clientRequestId,
  }) async {
    try {
      final body = <String, dynamic>{
        'content': content,
        if (clientRequestId != null && clientRequestId.isNotEmpty)
          'clientRequestId': clientRequestId,
      };
      final resp = await _dio.post<dynamic>(
        '/app/posts/$postId/comments',
        data: body,
      );
      final data = resp.data;
      if (data is Map<String, dynamic>) {
        final inner = data['data'];
        if (inner is Map<String, dynamic>) return inner;
        return data;
      }
      if (data is Map) return Map<String, dynamic>.from(data);
    } on DioException {
      // fall through
    }
    return null;
  }

  Future<Map<String, dynamic>?> replyToCommentRaw(
    String commentId,
    String content, {
    String? postId,
    String? replyToId,
    String? clientRequestId,
  }) async {
    try {
      final body = <String, dynamic>{'content': content};
      if (postId != null) body['postId'] = postId;
      if (replyToId != null) body['replyToId'] = replyToId;
      if (clientRequestId != null && clientRequestId.isNotEmpty) {
        body['clientRequestId'] = clientRequestId;
      }
      final resp = await _dio.post<dynamic>(
        '/app/posts/comments/$commentId/replies',
        data: body,
      );
      final data = resp.data;
      if (data is Map<String, dynamic>) {
        final inner = data['data'];
        if (inner is Map<String, dynamic>) return inner;
        return data;
      }
      if (data is Map) return Map<String, dynamic>.from(data);
    } on DioException {
      // fall through
    }
    return null;
  }

  Future<Map<String, dynamic>?> likeCommentRaw(String commentId) async {
    try {
      final resp = await _dio.post<dynamic>(
        '/app/posts/comments/$commentId/like',
      );
      final data = resp.data;
      if (data is Map<String, dynamic>) return data;
      if (data is Map) return Map<String, dynamic>.from(data);
    } on DioException {
      // fall through
    }
    return null;
  }

  Future<List<Map<String, dynamic>>> getCommentRepliesRaw(
    String commentId,
  ) async {
    try {
      final resp = await _dio.get<dynamic>(
        '/public/posts/comments/$commentId/replies',
      );
      final data = resp.data;
      if (data is List) {
        return data
            .whereType<Map>()
            .map((e) => Map<String, dynamic>.from(e))
            .toList();
      }
      if (data is Map) {
        final inner = data['data'];
        if (inner is Map) {
          final records = inner['records'];
          if (records is List) {
            return records
                .whereType<Map>()
                .map((e) => Map<String, dynamic>.from(e))
                .toList();
          }
        }
        if (inner is List) {
          return inner
              .whereType<Map>()
              .map((e) => Map<String, dynamic>.from(e))
              .toList();
        }
        for (final k in ['records', 'rows', 'list', 'items']) {
          final v = data[k];
          if (v is List) {
            return v
                .whereType<Map>()
                .map((e) => Map<String, dynamic>.from(e))
                .toList();
          }
        }
      }
    } on DioException {
      // fall through
    }
    return [];
  }

  Future<int> commentLikeCountRaw(String commentId) async {
    // like_count 已包含在留言列表回傳資料中，無需額外 API
    return 0;
  }

  Future<bool> commentLikedByMeRaw(String commentId, String userId) async {
    // liked 狀態已包含在留言列表回傳資料中，無需額外 API
    return false;
  }

  /// 統一檢舉端點 — POST /app/reports
  /// 取代舊版 per-post 端點 `/app/sellers/posts/{postId}/reports`（B-122/#610）。
  /// targetType: REVIEW / POST / POST_COMMENT / SELLER_FORUM / CHAT_MESSAGE / USER
  /// reasonCode: SPAM / SEXUAL / FRAUD / HARASSMENT / FOOD_SAFETY / POLITICAL / FAKE_NEWS / OTHER
  /// evidenceKeys: 裸 objectKey（如 `public/report/{buyerId}/1.png`）。Gateway 會
  /// 剝殼本平台顯示 URL，**外來域名與非 `public/` 前綴一律 400**（10 檔 §25）。
  /// 回傳 reportId。
  Future<String> submitReport({
    required String targetType,
    required String targetId,
    required String reasonCode,
    String? description,
    List<String>? evidenceKeys,
  }) async {
    try {
      final resp = await _dio.post<Map<String, dynamic>>(
        '/app/reports',
        data: {
          'targetType': targetType,
          'targetId': targetId,
          'reasonCode': reasonCode,
          if (description != null && description.isNotEmpty)
            'description': description,
          if (evidenceKeys != null && evidenceKeys.isNotEmpty)
            'evidenceKeys': evidenceKeys,
        },
      );
      final body = resp.data;
      if (body == null) throw const ServerException('舉報失敗');
      final code = body['code'];
      if (code != null && code != 0) {
        throw ServerException((body['message'] ?? '舉報失敗').toString());
      }
      final data = body['data'] as Map<String, dynamic>? ?? {};
      return (data['reportId'] ?? '').toString();
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      if (e.response?.statusCode == 409) {
        throw const ServerException('您已檢舉過此內容，請勿重複送出');
      }
      final bizMsg = _dioBizErrorMessage(e);
      if (bizMsg != null) throw ServerException(bizMsg);
      throw const NetworkException();
    }
  }

  /// GET /app/reports/me — 我的檢舉紀錄
  Future<List<ReportRecord>> getMyReports({
    int page = 1,
    int pageSize = 20,
  }) async {
    try {
      final resp = await _dio.get<Map<String, dynamic>>(
        '/app/reports/me',
        queryParameters: {'page': page, 'page_size': pageSize},
      );
      final body = resp.data;
      if (body == null) return [];
      final code = body['code'];
      if (code != null && code != 0) {
        throw ServerException((body['message'] ?? '查詢檢舉紀錄失敗').toString());
      }
      final data = body['data'] as Map<String, dynamic>? ?? {};
      final records = data['records'] as List? ?? [];
      return records
          .whereType<Map>()
          .map((e) => ReportRecord.fromJson(Map<String, dynamic>.from(e)))
          .toList();
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      final bizMsg = _dioBizErrorMessage(e);
      if (bizMsg != null) throw ServerException(bizMsg);
      throw const NetworkException();
    }
  }
}

/// 從 Dio 錯誤回應體讀取後端業務訊息（gateway envelope 統一欄位為 `message`，
/// `msg` 僅作少數舊路徑相容 fallback）。讀不到時回傳 null，
/// 呼叫端 fallback 到通用 [NetworkException]。
String? _dioBizErrorMessage(DioException e) {
  final data = e.response?.data;
  if (data is! Map) return null;
  final msg = data['message']?.toString().trim();
  return (msg != null && msg.isNotEmpty) ? msg : null;
}
