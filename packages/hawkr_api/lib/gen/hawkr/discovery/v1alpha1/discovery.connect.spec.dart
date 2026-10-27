//
//  Generated code. Do not modify.
//  source: hawkr/discovery/v1alpha1/discovery.proto
//

import "package:connectrpc/connect.dart" as connect;
import "discovery.pb.dart" as hawkrdiscoveryv1alpha1discovery;

/// DiscoveryPublicService 是任何人都能讀的探索資料，不需要登入。
abstract final class DiscoveryPublicService {
  /// Fully-qualified name of the DiscoveryPublicService service.
  static const name = 'hawkr.discovery.v1alpha1.DiscoveryPublicService';

  /// SuggestRecommendedSellers 依推薦分數由高到低回傳前幾家可以公開曝光的店家：沒有分頁（排行只取前面幾筆），數量由
  /// max_results 決定。分數是排程每 15 分鐘重算的快照；曝光資格是 seller 的「可公開曝光」規則（營運中、沒有停權、身分認證與
  /// PayUni 審核通過、店名與 logo 與所在地齊全），每次讀取即時檢查，停權立刻生效。店家卡片上的評價統計（review_count、
  /// rating_average）與其他清單上同一家店的卡片一致，是讀取時批次查詢評價的結果，不是排名用的快照；評價讀取失敗時卡片不設
  /// 評價欄位。沒有任何合格的店家時回空清單；排行上的店家若此刻沒有營業中的卡片（資料不一致，或剛好在兩次讀取之間不再可
  /// 曝光）會被略過，所以回傳的家數可能少於 max_results。粉絲數來自別的模組：讀取失敗時該欄位不設，不會是 0。
  static const suggestRecommendedSellers = connect.Spec(
    '/$name/SuggestRecommendedSellers',
    connect.StreamType.unary,
    hawkrdiscoveryv1alpha1discovery.SuggestRecommendedSellersRequest.new,
    hawkrdiscoveryv1alpha1discovery.SuggestRecommendedSellersResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );
}
