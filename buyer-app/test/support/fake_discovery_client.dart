import 'package:hawkr_api/gen/hawkr/discovery/v1alpha1/discovery.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/discovery/v1alpha1/discovery.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/discovery/v1alpha1/discovery.pb.dart'
    as discovery;
import 'package:hawkr_api/testing.dart';

/// DiscoveryPublicService 的 fake client。沒有註冊的 procedure 直接失敗。[requests] 記下每次的請求。
DiscoveryPublicServiceClient fakeDiscoveryClient({
  List<discovery.RecommendedSeller> sellers = const [],
  List<discovery.SuggestRecommendedSellersRequest>? requests,
}) {
  return DiscoveryPublicServiceClient(
    FakeTransportBuilder().unary(
      DiscoveryPublicService.suggestRecommendedSellers,
      (request, _) {
        requests?.add(request);
        return discovery.SuggestRecommendedSellersResponse(sellers: sellers);
      },
    ).build(),
  );
}
