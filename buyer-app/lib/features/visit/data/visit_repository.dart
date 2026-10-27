import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pb.dart' as buyer;
import 'package:riverpod_annotation/riverpod_annotation.dart';
import '../../../core/networking/connect_provider.dart';
import '../../../core/networking/membership_batch.dart';

part 'visit_repository.g.dart';

@riverpod
VisitRepository visitRepository(Ref ref) => VisitRepository(
  BuyerAppServiceClient(ref.watch(connectAuthTransportProvider)),
);

class VisitRepository {
  /// 列表每頁筆數（API 預設 20，上限 100）。
  static const listPageSize = 20;

  final BuyerAppServiceClient _buyer;
  VisitRepository(this._buyer);

  Future<void> addVisit(int productId) async {
    await _buyer.recordVisit(
      buyer.RecordVisitRequest(productId: productId.toString()),
    );
  }

  Future<Set<int>> batchCheckFavorites(Iterable<int> productIds) =>
      checkMembershipInBatches(productIds, (batch) async {
        final response = await _buyer.batchCheckFavorites(
          buyer.BatchCheckFavoritesRequest(
            productIds: batch.map((id) => id.toString()),
          ),
        );
        return response.favoritedProductIds.map(int.parse);
      });

  /// 收藏的一頁；[pageToken] 空字串是第一頁，回應的 `nextPageToken` 空字串表示沒有下一頁。
  Future<buyer.ListFavoritesResponse> listFavorites({
    String pageToken = '',
    int pageSize = listPageSize,
  }) => _buyer.listFavorites(
    buyer.ListFavoritesRequest(pageSize: pageSize, pageToken: pageToken),
  );

  /// 瀏覽紀錄的一頁（新到舊）；分頁語意同 [listFavorites]。
  Future<buyer.ListVisitsResponse> listVisits({
    String pageToken = '',
    int pageSize = listPageSize,
  }) => _buyer.listVisits(
    buyer.ListVisitsRequest(pageSize: pageSize, pageToken: pageToken),
  );

  Future<void> addFavorite(int productId) async {
    await _buyer.addFavorite(
      buyer.AddFavoriteRequest(productId: productId.toString()),
    );
  }

  Future<void> removeFavorite(int productId) async {
    await _buyer.removeFavorite(
      buyer.RemoveFavoriteRequest(productId: productId.toString()),
    );
  }
}
