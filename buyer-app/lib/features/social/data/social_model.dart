import 'package:freezed_annotation/freezed_annotation.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pb.dart' as pb;
import 'package:hawkr_api/images.dart' show ImagePreset, ImageUri;

import '../../seller/data/store_card_display.dart';

part 'social_model.freezed.dart';

/// 貼文關聯商品的卡片欄位：id、名稱、圖片（卡片尺寸）、最低價（新台幣整數，0 代表沒有價格）。
@freezed
abstract class PostProduct with _$PostProduct {
  const factory PostProduct({
    required String id,
    required String name,
    required String image,
    required int price,
  }) = _PostProduct;
}

/// 貼文的呈現模型，由 `hawkr.social.v1alpha1.Post` 轉來（[Post.fromProto]）。
///
/// 轉換時丟掉不用的欄位：標籤、更新時間、狀態與 etag（店主才有）、商品的最高價與可購買旗標、`viewer_*` 以外的店家卡片欄位。
@freezed
abstract class Post with _$Post {
  const factory Post({
    required int id,
    @Default('') String content,
    @Default('') String storeId,
    @Default('') String storeCode,
    String? merchantName,
    String? merchantAvatar,
    @Default('') String imageUrl,
    @Default(0) int likeCount,
    @Default(0) int commentCount,
    @Default(0) int shareCount,
    @Default(0) int viewCount,
    @Default(false) bool liked,
    int? createdAt,
    PostProduct? product,
    @Default(false) bool viewerCanComment,
    pb.Actor? viewerActor,
    @Default('') String shareUrl,
  }) = _Post;

  factory Post.fromProto(pb.Post post) {
    final store = post.hasStore() ? post.store : null;
    final product = post.hasProduct() ? post.product : null;
    return Post(
      id: int.parse(post.id),
      content: post.body,
      storeId: store?.id ?? '',
      storeCode: store?.storeCode ?? '',
      merchantName: store != null && store.hasName() ? store.name : null,
      merchantAvatar: store == null || store.logoUri.isEmpty
          ? null
          : store.logoUri,
      imageUrl: post.hasImage() ? post.image.uri(ImagePreset.detail) : '',
      likeCount: post.likeCount.toInt(),
      commentCount: post.commentCount.toInt(),
      shareCount: post.shareCount.toInt(),
      viewCount: post.viewCount.toInt(),
      liked: post.viewerLiked,
      createdAt: post.hasPublishTime()
          ? post.publishTime.seconds.toInt()
          : null,
      product: product == null
          ? null
          : PostProduct(
              id: product.id,
              name: product.name,
              image: product.hasImage()
                  ? product.image.uri(ImagePreset.card)
                  : '',
              price: product.minPrice.toInt(),
            ),
      viewerCanComment: post.viewerCanComment,
      viewerActor: post.hasViewerActor() ? post.viewerActor : null,
      shareUrl: post.url,
    );
  }
}
