import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_seller/shared/utils/seller_avatar_helper.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/images.dart';

import '../../../support/my_store_fixtures.dart';

void main() {
  test('統計沒有設的欄位是 null，不是 0', () {
    final unset = myStore();
    expect(unset.followerCount, isNull);
    expect(unset.reviewCount, isNull);
    expect(unset.ratingAverage, isNull);

    final partial = myStore(stats: StoreStats(followerCount: 0));
    expect(partial.followerCount, 0);
    expect(partial.reviewCount, isNull);
    expect(partial.ratingAverage, isNull);

    final full = myStore(
      stats: StoreStats(followerCount: 12, reviewCount: 3, ratingAverage: 4.5),
    );
    expect(full.followerCount, 12);
    expect(full.reviewCount, 3);
    expect(full.ratingAverage, 4.5);
  });

  test('logo 與封面網址由 uri_template 換 preset', () {
    final store = myStore(
      logoTemplate: 'https://media.example/i/{preset}/logo.png',
      coverTemplate: 'https://media.example/i/{preset}/cover.png',
    );
    expect(store.logoUrl, 'https://media.example/i/avatar/logo.png');
    expect(store.coverUrl, 'https://media.example/i/banner/cover.png');
    expect(myStore().logoUrl, isNull);
    expect(myStore().coverUrl, isNull);
  });

  test('SellerAvatarHelper 吃 Image：空樣板或沒有圖是 null', () {
    expect(SellerAvatarHelper.avatarUrl(null), isNull);
    expect(SellerAvatarHelper.avatarUrl(Image()), isNull);
    expect(
      SellerAvatarHelper.avatarUrl(Image(uriTemplate: 'https://x/{preset}/a')),
      'https://x/avatar/a',
    );
    expect(
      SellerAvatarHelper.coverUrl(Image(uriTemplate: 'https://x/{preset}/a')),
      'https://x/banner/a',
    );
  });

  test('店面位置是縣市加鄉鎮市區，不含明細', () {
    final store = myStore(townCode: '63000010', addressDetail: '路一號');
    expect(store.locationText, '臺北市松山區');
    expect(myStore().locationText, isNull);
  });
}
