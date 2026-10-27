import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pb.dart' as buyer;
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/store_card.pb.dart'
    as common;
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/image.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_buyer/features/follow/data/follow_repository.dart';
import 'package:hawkr_buyer/features/member_card/data/member_card_repository.dart';
import 'package:hawkr_buyer/features/visit/data/visit_repository.dart';
import 'package:dio/dio.dart';
import 'package:fixnum/fixnum.dart' show Int64;
import 'package:flutter_test/flutter_test.dart';

void main() {
  final store = common.StoreCard(
    id: '9',
    name: '農場',
    logo: Image(uriTemplate: 'https://media.test/i/{preset}/logo'),
  );
  final product = buyer.ProductCard(
    id: '866542675787809994',
    name: '水果',
    storeId: store.id,
    store: store,
  );
  final favoriteRequests = <buyer.ListFavoritesRequest>[];
  final visitRequests = <buyer.ListVisitsRequest>[];
  final followRequests = <buyer.ListFollowsRequest>[];
  setUp(() {
    favoriteRequests.clear();
    visitRequests.clear();
    followRequests.clear();
  });
  final transport = FakeTransportBuilder()
      .unary(
        BuyerAppService.listMemberCards,
        (_, _) => buyer.ListMemberCardsResponse(
          memberCards: [
            buyer.MemberCard(
              storeId: store.id,
              store: store,
              totalSpent: Int64(1190),
            ),
          ],
        ),
      )
      .unary(
        BuyerAppService.listSpendingRecords,
        (_, _) => buyer.ListSpendingRecordsResponse(
          records: [
            buyer.SpendingRecord(
              orderNumber: 'ORDER-1',
              amount: Int64(1190),
              refundedAmount: Int64(0),
              couponApplied: true,
            ),
          ],
          totalSpent: Int64(1190),
          totalRefunded: Int64(0),
        ),
      )
      .unary(BuyerAppService.listFavorites, (request, _) {
        favoriteRequests.add(request);
        return buyer.ListFavoritesResponse(
          favorites: [buyer.Favorite(product: product)],
          nextPageToken: request.pageToken.isEmpty ? 'fav-p2' : '',
          totalSize: 2,
        );
      })
      .unary(BuyerAppService.listVisits, (request, _) {
        visitRequests.add(request);
        return buyer.ListVisitsResponse(
          visits: [buyer.Visit(product: product)],
          nextPageToken: request.pageToken.isEmpty ? 'visit-p2' : '',
        );
      })
      .unary(
        BuyerAppService.batchCheckFavorites,
        (request, _) => buyer.BatchCheckFavoritesResponse(
          favoritedProductIds: request.productIds.where(
            (id) => id == '866542675787809994',
          ),
        ),
      )
      .unary(BuyerAppService.listFollows, (request, _) {
        followRequests.add(request);
        return buyer.ListFollowsResponse(
          follows: [buyer.Follow(storeId: store.id, store: store)],
          nextPageToken: request.pageToken.isEmpty ? 'follow-p2' : '',
          totalSize: 2,
        );
      })
      .build();
  final client = BuyerAppServiceClient(transport);

  test(
    'member cards retain store identity, spend, and generated image URI',
    () async {
      final result = await MemberCardRepository(client).getMemberCards();
      expect(result.records.single.sellerId, '9');
      expect(result.records.single.totalSpending, '1190');
      expect(
        result.records.single.sellerLogoKey,
        'https://media.test/i/avatar/logo',
      );
    },
  );

  test(
    'spending records retain totals, order number, refund, and coupon flag',
    () async {
      final result = await MemberCardRepository(
        client,
      ).getSpendingRecords(sellerId: '9');
      expect(result.totalSpending, 1190);
      expect(result.totalRefund, 0);
      expect(result.records.single.orderSn, 'ORDER-1');
      expect(result.records.single.refund, '0');
      expect(result.records.single.hasCoupon, isTrue);
    },
  );

  test(
    'favorites load one cursor page at a time and keep the exact product ID',
    () async {
      final repository = VisitRepository(client);
      final first = await repository.listFavorites();
      expect(favoriteRequests, hasLength(1));
      expect(favoriteRequests.single.pageToken, isEmpty);
      expect(favoriteRequests.single.pageSize, VisitRepository.listPageSize);
      expect(first.favorites.single.product.id, '866542675787809994');
      expect(first.nextPageToken, 'fav-p2');
      expect(first.totalSize, 2);

      final second = await repository.listFavorites(
        pageToken: first.nextPageToken,
      );
      expect(favoriteRequests, hasLength(2));
      expect(favoriteRequests.last.pageToken, 'fav-p2');
      expect(second.nextPageToken, isEmpty);
    },
  );

  test('visits load one cursor page at a time', () async {
    final first = await VisitRepository(client).listVisits();
    expect(visitRequests.single.pageToken, isEmpty);
    expect(visitRequests.single.pageSize, VisitRepository.listPageSize);
    expect(first.visits.single.product.id, '866542675787809994');
    expect(first.nextPageToken, 'visit-p2');
  });

  test('visible product membership uses the batch RPC', () async {
    final ids = await VisitRepository(
      client,
    ).batchCheckFavorites([866542675787809994, 123]);
    expect(ids, {866542675787809994});
  });

  test('follows load one cursor page at a time', () async {
    final dio = Dio();
    addTearDown(dio.close);
    final first = await FollowRepository(dio, client).listFollows();
    expect(followRequests.single.pageToken, isEmpty);
    expect(followRequests.single.pageSize, FollowRepository.listPageSize);
    expect(first.follows.single.store.id, '9');
    expect(first.nextPageToken, 'follow-p2');
    expect(first.totalSize, 2);
  });
}
