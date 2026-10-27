import 'package:hawkr_api/gen/hawkr/content/v1alpha1/content.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/content/v1alpha1/content.pb.dart' as api;
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart';

/// ContentPublicService 的 fake transport，給覆寫 `connectTransportProvider` 用。沒有註冊的 procedure
/// 直接失敗，測試不會連到真的 gateway。[calls] 記下每次呼叫的 procedure。
Transport fakeContentTransport({
  List<api.Banner> banners = const [],
  List<String>? calls,
}) {
  return FakeTransportBuilder().unary(ContentPublicService.listLiveBanners, (
    _,
    __,
  ) {
    calls?.add(ContentPublicService.listLiveBanners.procedure);
    return api.ListLiveBannersResponse(banners: banners);
  }).build();
}
