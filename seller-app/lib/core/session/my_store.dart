import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/images.dart' show Image;
import 'package:hawkr_twaddr/hawkr_twaddr.dart' as twaddr;

import '../../shared/utils/seller_avatar_helper.dart';

/// 物流設定（`SHIPPING_SETUP`）的閘門，只由伺服器回報的啟用與缺的條件決定，優先序固定：等待刪除 > 停權 > 認證。
/// 啟用就開放；缺 `KYC_APPROVED` 是認證閘門；店家還沒讀到、伺服器沒回報這個能力、缺 `STORE_ACTIVE` 或缺項不認得，
/// 都是 [unknown]（不退回認證閘門）。
enum ShippingGate { open, suspended, pendingDeletion, certification, unknown }

/// 賣家讀到的店家（`SellerAppService.GetMyStore`）：唯讀的判斷放在這裡，和 website 的 `lib/seller/store.ts` 同一套語意。
///
/// proto 沒設的欄位不是 0 或空字串，判斷一律從「有沒有設」開始；`*_UNSPECIFIED` 的列舉值不當成有效值。
final class MyStore {
  const MyStore._(this._response);

  /// 伺服器回了店家（含停權、等待刪除）。
  factory MyStore(GetMyStoreResponse response) => MyStore._(response);

  /// 伺服器回 not_found：帳號還沒有店家。
  const MyStore.absent() : _response = null;

  final GetMyStoreResponse? _response;

  /// 同一份讀取結果，店家換成寫入後伺服器回的版本。沒有店家時回自己。
  MyStore replaceStore(Store written) {
    final response = _response;
    if (response == null) return this;
    return MyStore._(response.deepCopy()..store = written);
  }

  Store? get store {
    final response = _response;
    return response != null && response.hasStore() ? response.store : null;
  }

  bool get hasStore => store != null;

  /// 店家 ID；沒有店家（或沒有 ID）時是 null，不是空字串。
  String? get storeId {
    final id = store?.id ?? '';
    return id.isEmpty ? null : id;
  }

  String? get storeName => _nonEmpty(store?.storeName);

  /// 店家網址的路徑段（對外顯示碼）；沒有時是 null。
  String? get storeCode => _nonEmpty(store?.storeCode);

  /// 寫入店家的版本；沒有店家時是 null。
  String? get etag {
    final value = store?.etag ?? '';
    return value.isEmpty ? null : value;
  }

  Image? get logo {
    final current = store;
    return current != null && current.hasLogo() ? current.logo : null;
  }

  Image? get cover {
    final current = store;
    return current != null && current.hasCoverImage()
        ? current.coverImage
        : null;
  }

  /// 頭像網址（`Image.uri_template` 加 avatar preset）；沒有 logo 時是 null。
  String? get logoUrl => SellerAvatarHelper.avatarUrl(logo);

  /// 封面網址（banner preset）；沒有封面時是 null。
  String? get coverUrl => SellerAvatarHelper.coverUrl(cover);

  /// 粉絲數；讀不到（伺服器沒有設）時是 null，不是 0。
  int? get followerCount {
    final stats = _stats;
    return stats != null && stats.hasFollowerCount()
        ? stats.followerCount
        : null;
  }

  /// 評價數；讀不到時是 null。
  int? get reviewCount {
    final stats = _stats;
    return stats != null && stats.hasReviewCount() ? stats.reviewCount : null;
  }

  /// 平均評分（1 到 5）；沒有評價或讀不到時是 null。
  double? get ratingAverage {
    final stats = _stats;
    return stats != null && stats.hasRatingAverage()
        ? stats.ratingAverage
        : null;
  }

  StoreStats? get _stats {
    final response = _response;
    return response != null && response.hasStats() ? response.stats : null;
  }

  /// 商店電話（`store.contact_phone`）。
  String? get contactPhone => _nonEmpty(store?.contactPhone);

  /// 聯絡人姓名（`store.contact_name`）；沒有填時是 null。
  String? get contactName => _nonEmpty(store?.contactName);

  /// 聯絡人手機（`store.contact_mobile`）；沒有填時是 null。
  String? get contactMobile => _nonEmpty(store?.contactMobile);

  /// 店家地址的鄉鎮市區代碼；沒有填地址時是 null。
  String? get addressTownCode {
    final current = store;
    if (current == null || !current.hasAddress()) return null;
    return _nonEmpty(current.address.townCode);
  }

  /// 店家地址的明細（路名與門牌）；沒有填時是 null。
  String? get addressDetail {
    final current = store;
    if (current == null || !current.hasAddress()) return null;
    return _nonEmpty(current.address.addressDetail);
  }

  /// 店家地址的顯示文字（縣市＋鄉鎮市區＋明細）；沒有填地址時是 null。
  String? get addressText {
    final current = store;
    if (current == null || !current.hasAddress()) return null;
    return _nonEmpty(
      twaddr.formatAddress(
        current.address.townCode,
        current.address.addressDetail,
      ),
    );
  }

  /// 縣市加鄉鎮市區（不含地址明細），店面頁顯示用；沒有地址或代碼不認得時是 null。
  String? get locationText {
    final current = store;
    if (current == null || !current.hasAddress()) return null;
    final town = twaddr.townByCode(current.address.townCode);
    if (town == null) return null;
    final county = twaddr.countyByCode(town.countyCode);
    return '${county?.name ?? ''}${town.name}';
  }

  /// 帳號此刻停權（`GetMyStore.suspended`，與能力的 `NOT_SUSPENDED` 同一次推導）；停權的內容讀 `ListMyMeasures`。
  bool get isSuspended => _response?.suspended ?? false;

  /// 隨帳號刪除、等待未結訂單完成。
  bool get isPendingDeletion =>
      store?.status == StoreStatus.STORE_STATUS_PENDING_DELETION;

  /// 開店流程還沒完成：沒有店家，或店家沒有標記完成新手引導。
  bool get needsOnboarding => !(store?.onboardingCompleted ?? false);

  /// 伺服器回報的一個能力的狀態；沒有回報（不是「沒有啟用」）時是 null。
  CapabilityStatus? capabilityStatus(SellerCapability capability) {
    final response = _response;
    if (response == null) return null;
    for (final status in response.capabilities) {
      if (status.capability == capability) return status;
    }
    return null;
  }

  /// 目前啟用的能力（`GetMyStore.capabilities` 中 `active` 的那些，去掉 `SELLER_CAPABILITY_UNSPECIFIED`）。
  List<SellerCapability> get capabilities {
    final response = _response;
    if (response == null) return const [];
    return List.unmodifiable([
      for (final status in response.capabilities)
        if (status.active &&
            status.capability != SellerCapability.SELLER_CAPABILITY_UNSPECIFIED)
          status.capability,
    ]);
  }

  /// 這個能力現在啟用；伺服器沒有回報它時是 false。
  bool can(SellerCapability capability) =>
      capability != SellerCapability.SELLER_CAPABILITY_UNSPECIFIED &&
      (capabilityStatus(capability)?.active ?? false);

  /// 這個能力還缺的條件（去掉 `SELLER_REQUIREMENT_UNSPECIFIED`）；伺服器沒有回報這個能力時是 null，不是「什麼都不缺」。
  List<SellerRequirement>? missingFor(SellerCapability capability) {
    final status = capabilityStatus(capability);
    if (status == null) return null;
    return List.unmodifiable(
      status.missing.where(
        (r) => r != SellerRequirement.SELLER_REQUIREMENT_UNSPECIFIED,
      ),
    );
  }

  ShippingGate get shippingGate {
    final status = capabilityStatus(
      SellerCapability.SELLER_CAPABILITY_SHIPPING_SETUP,
    );
    if (status == null) return ShippingGate.unknown;
    if (status.active) return ShippingGate.open;
    final missing = status.missing;
    if (missing.contains(
      SellerRequirement.SELLER_REQUIREMENT_NOT_PENDING_DELETION,
    )) {
      return ShippingGate.pendingDeletion;
    }
    if (missing.contains(SellerRequirement.SELLER_REQUIREMENT_NOT_SUSPENDED)) {
      return ShippingGate.suspended;
    }
    if (missing.contains(SellerRequirement.SELLER_REQUIREMENT_KYC_APPROVED) &&
        !missing.contains(SellerRequirement.SELLER_REQUIREMENT_STORE_ACTIVE)) {
      return ShippingGate.certification;
    }
    return ShippingGate.unknown;
  }

  /// 商家認證（含 PayUni 審核）已核准：`PUBLIC_LISTING` 缺的條件不含 `KYC_APPROVED`。沒有回報 `PUBLIC_LISTING` 時不算已認證。
  bool get isCertified {
    final missing = missingFor(
      SellerCapability.SELLER_CAPABILITY_PUBLIC_LISTING,
    );
    return missing != null &&
        !missing.contains(SellerRequirement.SELLER_REQUIREMENT_KYC_APPROVED);
  }

  /// 商店介紹；沒有填時是 null。
  String? get description => _nonEmpty(store?.description);

  /// 平台核准的食品業者登錄字號；沒有核准的登錄時是 null。
  String? get foodRegistrationNumber {
    final current = store;
    if (current == null || !current.hasApprovedFoodRegistration()) return null;
    return _nonEmpty(current.approvedFoodRegistration.registrationNumber);
  }

  /// 待審的登錄（有值時可以撤回）；沒有時是 null。
  FoodRegistration? get pendingFoodRegistration {
    final current = store;
    if (current == null || !current.hasPendingFoodRegistration()) return null;
    return current.pendingFoodRegistration;
  }

  /// 被退回、之後沒有再送出新的登錄；沒有時是 null。
  FoodRegistration? get rejectedFoodRegistration {
    final current = store;
    if (current == null || !current.hasRejectedFoodRegistration()) return null;
    return current.rejectedFoodRegistration;
  }

  /// 最近一次被平台撤銷的登錄（字號、撤銷時間與理由）；伺服器只在目前沒有核准、撤銷之後也沒有更新的待審或核准時才帶，沒有時是 null。
  FoodRegistration? get revokedFoodRegistration {
    final current = store;
    if (current == null || !current.hasRevokedFoodRegistration()) return null;
    return current.revokedFoodRegistration;
  }

  /// 停權的通用說明：停權的內容（依據與期間）在措施列表。
  static const genericSuspensionLabel = '帳號已被平台停權';

  @override
  bool operator ==(Object other) =>
      other is MyStore && other._response == _response;

  @override
  int get hashCode => _response.hashCode;
}

String? _nonEmpty(String? value) {
  final trimmed = value?.trim();
  return trimmed == null || trimmed.isEmpty ? null : trimmed;
}
