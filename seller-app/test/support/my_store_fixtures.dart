import 'package:hawkr_api/gen/hawkr/common/v1alpha1/image.pb.dart';
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/postal_address.pb.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_seller/core/session/my_store.dart';

const _all = SellerCapability.values;

/// 後端 `CapabilitiesOf(active, suspended=false)` 啟用的動作類能力：全部能力，不含店家狀態 `PUBLIC_LISTING`
/// （它的缺項由 [publicListingMissing] 另外給）。
final activeCapabilities = [
  for (final c in _all)
    if (c != SellerCapability.SELLER_CAPABILITY_UNSPECIFIED &&
        c != SellerCapability.SELLER_CAPABILITY_PUBLIC_LISTING)
      c,
];

/// 停權時只剩白名單（`grantedWhenSuspended`）。
final suspendedCapabilities = [
  SellerCapability.SELLER_CAPABILITY_ACCOUNT_SELF_READ,
  SellerCapability.SELLER_CAPABILITY_ORDER_FULFILL,
  SellerCapability.SELLER_CAPABILITY_ORDER_CONTRACT,
  SellerCapability.SELLER_CAPABILITY_CHAT_SEND,
  SellerCapability.SELLER_CAPABILITY_CHAT_READ,
  SellerCapability.SELLER_CAPABILITY_NOTIFICATION,
  SellerCapability.SELLER_CAPABILITY_CONTENT_CONTRACT,
  SellerCapability.SELLER_CAPABILITY_ACCOUNT_CLOSE,
  SellerCapability.SELLER_CAPABILITY_INVOICE_TAX_REMEDY,
];

/// 等待刪除時再去掉經營類與關店。
final pendingDeletionCapabilities = [
  SellerCapability.SELLER_CAPABILITY_ACCOUNT_SELF_READ,
  SellerCapability.SELLER_CAPABILITY_ORDER_FULFILL,
  SellerCapability.SELLER_CAPABILITY_ORDER_CONTRACT,
  SellerCapability.SELLER_CAPABILITY_CHAT_SEND,
  SellerCapability.SELLER_CAPABILITY_CHAT_READ,
  SellerCapability.SELLER_CAPABILITY_NOTIFICATION,
  SellerCapability.SELLER_CAPABILITY_CONTENT_CONTRACT,
  SellerCapability.SELLER_CAPABILITY_INVOICE_TAX_REMEDY,
];

/// 伺服器回的每個動作類能力的狀態：[active] 的啟用，其餘沒有啟用並缺 [stateMissing]（停權或等待刪除造成的缺項）。
List<CapabilityStatus> actionStatuses(
  Iterable<SellerCapability> active, {
  List<SellerRequirement> stateMissing = const [],
}) => [
  for (final c in activeCapabilities)
    if (active.contains(c))
      CapabilityStatus(capability: c, active: true)
    else
      CapabilityStatus(capability: c, active: false, missing: stateMissing),
];

/// 物流設定（`SHIPPING_SETUP`）缺 [missing] 這些條件的能力狀態（空清單就是啟用）。
CapabilityStatus shippingSetupMissing([
  List<SellerRequirement> missing = const [],
]) => CapabilityStatus(
  capability: SellerCapability.SELLER_CAPABILITY_SHIPPING_SETUP,
  active: missing.isEmpty,
  missing: missing,
);

/// 伺服器對 `GetMyStore` 的回應。[suspended] 為 true 是帳號停權；[capabilities] 是啟用的能力，沒給時依狀態取後端推導的集合；
/// [reportCapabilities] 為 false 是伺服器沒有回報任何能力狀態（`missingFor` 回 null）；[shippingSetup] 取代預設啟用的 `SHIPPING_SETUP`；[publicListing] 是店家狀態 `PUBLIC_LISTING`（[publicListingMissing]），不給就是伺服器沒有回報它。
GetMyStoreResponse myStoreResponse({
  String id = 'store-1',
  String name = '阿明果園',
  String contactPhone = '',
  String contactName = '',
  String contactMobile = '',
  String? townCode,
  String addressDetail = '',
  bool onboardingCompleted = true,
  StoreStatus status = StoreStatus.STORE_STATUS_ACTIVE,
  bool suspended = false,
  List<SellerCapability>? capabilities,
  String etag = 'etag-1',
  StoreStats? stats,
  String? logoTemplate,
  String? coverTemplate,
  String storeCode = '',
  CapabilityStatus? publicListing,
  CapabilityStatus? shippingSetup,
  bool reportCapabilities = true,
  String description = '',
}) {
  final store = Store(
    description: description,
    id: id,
    storeCode: storeCode,
    storeName: name,
    contactPhone: contactPhone,
    contactName: contactName,
    contactMobile: contactMobile,
    onboardingCompleted: onboardingCompleted,
    status: status,
    etag: etag,
  );
  if (logoTemplate != null) store.logo = Image(uriTemplate: logoTemplate);
  if (coverTemplate != null) {
    store.coverImage = Image(uriTemplate: coverTemplate);
  }
  if (townCode != null) {
    store.address = PostalAddress(
      townCode: townCode,
      addressDetail: addressDetail,
    );
  }
  final stateMissing = suspended
      ? [SellerRequirement.SELLER_REQUIREMENT_NOT_SUSPENDED]
      : status == StoreStatus.STORE_STATUS_PENDING_DELETION
      ? [SellerRequirement.SELLER_REQUIREMENT_NOT_PENDING_DELETION]
      : <SellerRequirement>[];
  return GetMyStoreResponse(
    store: store,
    stats: stats,
    suspended: suspended,
    capabilities: [
      if (reportCapabilities)
        for (final s in actionStatuses(
          capabilities ??
              (suspended
                  ? suspendedCapabilities
                  : status == StoreStatus.STORE_STATUS_PENDING_DELETION
                  ? pendingDeletionCapabilities
                  : activeCapabilities),
          stateMissing: stateMissing,
        ))
          if (s.capability ==
                  SellerCapability.SELLER_CAPABILITY_SHIPPING_SETUP &&
              shippingSetup != null)
            shippingSetup
          else
            s,
      if (reportCapabilities) ?publicListing,
    ],
  );
}

MyStore myStore({
  bool onboardingCompleted = true,
  StoreStatus status = StoreStatus.STORE_STATUS_ACTIVE,
  bool suspended = false,
  List<SellerCapability>? capabilities,
  String name = '阿明果園',
  String contactPhone = '',
  String? townCode,
  String addressDetail = '',
  String etag = 'etag-1',
  StoreStats? stats,
  String? logoTemplate,
  String? coverTemplate,
  CapabilityStatus? publicListing,
  CapabilityStatus? shippingSetup,
  bool reportCapabilities = true,
  String description = '',
}) => MyStore(
  myStoreResponse(
    publicListing: publicListing,
    shippingSetup: shippingSetup,
    reportCapabilities: reportCapabilities,
    description: description,
    etag: etag,
    stats: stats,
    logoTemplate: logoTemplate,
    coverTemplate: coverTemplate,
    name: name,
    contactPhone: contactPhone,
    townCode: townCode,
    addressDetail: addressDetail,
    onboardingCompleted: onboardingCompleted,
    status: status,
    suspended: suspended,
    capabilities: capabilities,
  ),
);

MyStore suspendedStore() => myStore(suspended: true);

MyStore pendingDeletionStore() =>
    myStore(status: StoreStatus.STORE_STATUS_PENDING_DELETION);

/// 缺 [missing] 這些公開條件的店家狀態 `PUBLIC_LISTING`（空清單就是全部滿足）。
CapabilityStatus publicListingMissing([
  List<SellerRequirement> missing = const [],
]) => CapabilityStatus(
  capability: SellerCapability.SELLER_CAPABILITY_PUBLIC_LISTING,
  active: missing.isEmpty,
  missing: missing,
);
