import 'package:hawkr_api/gen/hawkr/common/v1alpha1/store_card.pb.dart';
import 'package:hawkr_api/images.dart' show ImagePreset, ImageUri;
import 'package:hawkr_twaddr/hawkr_twaddr.dart';

/// 名稱不設時的顯示（個資清除之後的已關閉店家）。
const closedStoreName = '已關閉的店家';

/// 店家卡片在 App 各處的呈現：名稱、營業狀態、所在地與圖片。
extension StoreCardDisplay on StoreCard {
  /// 名稱不設或空白時是 [closedStoreName]。
  String get displayName {
    final value = hasName() ? name.trim() : '';
    return value.isEmpty ? closedStoreName : value;
  }

  /// 營業中且有店面網址：可點、以 `store_code` 導頁。
  bool get isOpen =>
      status == StoreCardStatus.STORE_CARD_STATUS_OPEN && storeCode.isNotEmpty;

  /// 暫停或已關閉時的標籤；營業中與未知的狀態沒有標籤。
  String? get statusLabel => switch (status) {
    StoreCardStatus.STORE_CARD_STATUS_PAUSED => '暫停營業',
    StoreCardStatus.STORE_CARD_STATUS_CLOSED => '已關閉',
    _ => null,
  };

  /// 縣市加鄉鎮市區；沒有代碼或不認得時是空字串。
  String get location => townLocation(townCode);

  String get logoUri => hasLogo() ? logo.uri(ImagePreset.avatar) : '';

  /// 評分與評價數都讀得到時才有值。
  bool get hasRatingStats => hasRatingAverage() && hasReviewCount();
}

/// 內政部 8 碼鄉鎮市區代碼轉成縣市加鄉鎮市區；空字串或不認得的代碼回空字串。
String townLocation(String townCode) {
  if (townByCode(townCode) == null) return '';
  return formatAddress(townCode, '');
}
