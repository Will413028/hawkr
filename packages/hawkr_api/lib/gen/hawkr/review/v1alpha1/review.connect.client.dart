//
//  Generated code. Do not modify.
//  source: hawkr/review/v1alpha1/review.proto
//

import "package:connectrpc/connect.dart" as connect;
import "review.pb.dart" as hawkrreviewv1alpha1review;
import "review.connect.spec.dart" as specs;

/// ReviewPublicService 是任何人都能讀的評價與統計，不需要登入。
extension type ReviewPublicServiceClient (connect.Transport _transport) {
  /// ListProductReviews 列出商品的公開評價，新到舊（建立時間，相同時以 ID 排序）。公開可讀：沒有未還原的平台隱藏、作者仍在營運中。
  /// 帶登入憑證時套用觀看者的封鎖，過濾在分頁之前，所以一頁可以少於 page_size，結束只看 next_page_token。可以用 rating 與 with_images 篩選；
  /// page_token 只能用在同一組 product_id 與篩選。商品沒有評價是空的一頁。
  Future<hawkrreviewv1alpha1review.ListProductReviewsResponse> listProductReviews(
    hawkrreviewv1alpha1review.ListProductReviewsRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.ReviewPublicService.listProductReviews,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// ListStoreReviews 列出店家的公開評價，規則同 ListProductReviews。店家自己讀自己店的評價用 ReviewAppService.ListOwnStoreReviews（不套觀看者）。
  Future<hawkrreviewv1alpha1review.ListStoreReviewsResponse> listStoreReviews(
    hawkrreviewv1alpha1review.ListStoreReviewsRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.ReviewPublicService.listStoreReviews,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// GetStoreReviewStats 是店家的評價統計：公開可讀的評價數、平均與各星等的數量。統計不依觀看者過濾，公開與店家自己看到的數字相同。
  Future<hawkrreviewv1alpha1review.GetStoreReviewStatsResponse> getStoreReviewStats(
    hawkrreviewv1alpha1review.GetStoreReviewStatsRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.ReviewPublicService.getStoreReviewStats,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }
}
/// ReviewAppService 是登入者的評價。評價是買家與店家共用的資源（買家寫、店家回）：CreateReview 與 ListMyReviews 是買家；
/// ListOwnStoreReviews 的店家身分由伺服器從 session 推導（呼叫者此刻的店），ReplyReview 的店家由評價所屬的店推導，請求都不帶 store_id。
extension type ReviewAppServiceClient (connect.Transport _transport) {
  /// GetReview 讀一則公開可讀的評價，套用觀看者的封鎖；其他一律回 not_found，不區分原因。
  Future<hawkrreviewv1alpha1review.GetReviewResponse> getReview(
    hawkrreviewv1alpha1review.GetReviewRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.ReviewAppService.getReview,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// ReplyReview 回覆一則評價，已有回覆時覆蓋；店家由評價所屬的店推導，請求不帶 store_id。需要賣家能力 CAP_MARKETING。評價不存在或呼叫者讀不到回 not_found，
  /// 評價公開可讀，所以呼叫者不是那間店的店主（包含沒有店）回 permission_denied，評價此刻不能回覆（被隱藏、作者已離開、回覆被下架）回 failed_precondition 與 REVIEW_NOT_REPLYABLE。
  Future<hawkrreviewv1alpha1review.ReplyReviewResponse> replyReview(
    hawkrreviewv1alpha1review.ReplyReviewRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.ReviewAppService.replyReview,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// CreateReview 以呼叫者的買家身分對一個已完成的訂單項目寫評價。商品、規格、價格與商品圖取自訂單項目。訂單不是呼叫者的或不存在回 not_found；
  /// 同一個訂單項目已有評價（可能已被平台隱藏而不在 ListMyReviews 的結果中）回 already_exists 與 REVIEW_ALREADY_EXISTS，client 顯示「已評價」；訂單還不能評價回 failed_precondition 與 ORDER_NOT_REVIEWABLE；
  /// 超過評價期限回 failed_precondition 與 REVIEW_WINDOW_EXPIRED。
  Future<hawkrreviewv1alpha1review.CreateReviewResponse> createReview(
    hawkrreviewv1alpha1review.CreateReviewRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.ReviewAppService.createReview,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// ListMyReviews 列出呼叫者自己寫的評價，新到舊；被平台隱藏的不列（作者看不到被隱藏的評價）。order_item_id 只列那個訂單項目的可見評價（至多一則）；沒有結果不代表沒寫過，被隱藏的評價仍占著該訂單項目（CreateReview 會回 REVIEW_ALREADY_EXISTS）。
  /// page_token 只能用在同一個 order_item_id。
  Future<hawkrreviewv1alpha1review.ListMyReviewsResponse> listMyReviews(
    hawkrreviewv1alpha1review.ListMyReviewsRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.ReviewAppService.listMyReviews,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// ListOwnStoreReviews 列出呼叫者此刻的店的公開可讀評價，新到舊，不套觀看者的封鎖（店家看得到被它封鎖的買家的評價）。呼叫者沒有店回 not_found。
  /// 不檢查賣家能力：停權的店家仍可讀自己店的評價。
  Future<hawkrreviewv1alpha1review.ListOwnStoreReviewsResponse> listOwnStoreReviews(
    hawkrreviewv1alpha1review.ListOwnStoreReviewsRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.ReviewAppService.listOwnStoreReviews,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }
}
