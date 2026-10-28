//
//  Generated code. Do not modify.
//  source: hawkr/content/v1alpha1/content.proto
//

import "package:connectrpc/connect.dart" as connect;
import "content.pb.dart" as hawkrcontentv1alpha1content;

/// ContentPublicService 是前台讀取的內容，不需要登入。
abstract final class ContentPublicService {
  /// Fully-qualified name of the ContentPublicService service.
  static const name = 'hawkr.content.v1alpha1.ContentPublicService';

  /// ListLiveBanners 列出此刻要在首頁顯示的 banner：啟用中、而且在顯示期間 [開始, 結束) 內，依顯示順序排列。
  /// 啟用中的 banner 最多 20 則（寫入時限制），第一頁（預設 50 則）就是全部，client 不必翻頁。
  static const listLiveBanners = connect.Spec(
    '/$name/ListLiveBanners',
    connect.StreamType.unary,
    hawkrcontentv1alpha1content.ListLiveBannersRequest.new,
    hawkrcontentv1alpha1content.ListLiveBannersResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// CheckAppVersion 判斷 App 是否低於最低版本、必須強制更新。版本比較只在後端做，
  /// App 只看 update_required。App 收到任何錯誤都要放行（fail-open）。
  static const checkAppVersion = connect.Spec(
    '/$name/CheckAppVersion',
    connect.StreamType.unary,
    hawkrcontentv1alpha1content.CheckAppVersionRequest.new,
    hawkrcontentv1alpha1content.CheckAppVersionResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );
}
