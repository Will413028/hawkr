import 'package:hawkr_api/gen/hawkr/content/v1alpha1/content.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/content/v1alpha1/content.pb.dart' as api;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/connect_provider.dart';

part 'banner_repository.g.dart';

@Riverpod(keepAlive: true)
BannerRepository bannerRepository(Ref ref) =>
    BannerRepository(ref.watch(contentClientProvider));

/// 首頁的 banner（ContentPublicService 的 ListLiveBanners）。
class BannerRepository {
  BannerRepository(this._client);

  final ContentPublicServiceClient _client;

  /// 上架中、在期間內的 banner，依顯示順序。只讀第一頁：伺服器預設每頁 50 則，上架中的 banner 不會超過
  /// 這個數量。錯誤原樣丟出（transport 已依錯誤模型重試過）。
  Future<List<api.Banner>> liveBanners() async {
    final res = await _client.listLiveBanners(api.ListLiveBannersRequest());
    return res.banners;
  }
}
