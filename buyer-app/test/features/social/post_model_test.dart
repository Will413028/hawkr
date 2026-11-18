import 'package:fixnum/fixnum.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/image.pb.dart' as common;
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/store_card.pb.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pb.dart' as pb;
import 'package:hawkr_buyer/features/social/data/social_model.dart';

import 'social_fakes.dart';

void main() {
  test('Post.fromProto 取店家、圖片、商品與觀看者的欄位', () {
    final proto = pb.Post(
      id: '101',
      store: StoreCard(
        id: '7',
        storeCode: 'nanmu',
        name: '南崁王',
        logo: common.Image(uriTemplate: 'https://img.test/{preset}/logo.jpg'),
      ),
      body: '內文',
      image: common.Image(uriTemplate: 'https://img.test/{preset}/p.jpg'),
      likeCount: Int64(5),
      commentCount: Int64(3),
      shareCount: Int64(2),
      viewCount: Int64(9),
      viewerLiked: true,
      viewerCanComment: true,
      viewerActor: storeActor('南崁王'),
      url: 'https://hawkr.test/sellers/nanmu/posts/101',
      product: pb.PostProduct(
        id: '55',
        name: '紅豆',
        minPrice: Int64(195),
        image: common.Image(uriTemplate: 'https://img.test/{preset}/r.jpg'),
      ),
    );
    proto.publishTime = proto.publishTime.deepCopy()
      ..seconds = Int64(1783000000);

    final post = Post.fromProto(proto);

    expect(post.id, 101);
    expect(post.content, '內文');
    expect(post.merchantId, 7);
    expect(post.storeCode, 'nanmu');
    expect(post.merchantName, '南崁王');
    expect(post.merchantAvatar, 'https://img.test/avatar/logo.jpg');
    expect(post.imageUrl, 'https://img.test/detail/p.jpg');
    expect(post.likeCount, 5);
    expect(post.commentCount, 3);
    expect(post.shareCount, 2);
    expect(post.viewCount, 9);
    expect(post.liked, isTrue);
    expect(post.createdAt, 1783000000);
    expect(post.viewerCanComment, isTrue);
    expect(post.viewerActor?.name, '南崁王');
    expect(post.shareUrl, 'https://hawkr.test/sellers/nanmu/posts/101');
    expect(
      post.product,
      const PostProduct(
        id: '55',
        name: '紅豆',
        image: 'https://img.test/card/r.jpg',
        price: 195,
      ),
    );
  });

  test('沒有店家、圖片、商品與觀看者時不補假值', () {
    final post = Post.fromProto(pb.Post(id: '9', body: 'x'));

    expect(post.merchantId, isNull);
    expect(post.merchantName, isNull);
    expect(post.merchantAvatar, isNull);
    expect(post.storeCode, '');
    expect(post.imageUrl, '');
    expect(post.createdAt, isNull);
    expect(post.product, isNull);
    expect(post.viewerActor, isNull);
    expect(post.viewerCanComment, isFalse);
    expect(post.liked, isFalse);
  });
}
