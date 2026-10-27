import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pbenum.dart';

import '../session/my_store.dart';
import 'app_router.dart';

const _storefrontWrite = SellerCapability.SELLER_CAPABILITY_STOREFRONT_WRITE;
const _productManage = SellerCapability.SELLER_CAPABILITY_PRODUCT_MANAGE;
const _orderFulfill = SellerCapability.SELLER_CAPABILITY_ORDER_FULFILL;
const _marketing = SellerCapability.SELLER_CAPABILITY_MARKETING;
const _chatRead = SellerCapability.SELLER_CAPABILITY_CHAT_READ;
const _notification = SellerCapability.SELLER_CAPABILITY_NOTIFICATION;
const _accountClose = SellerCapability.SELLER_CAPABILITY_ACCOUNT_CLOSE;

/// App 每一個路由需要的賣家能力；`null` 表示宣告過、不需要能力的頁面（公告頁、純本機的工具頁、
/// 不分買賣家的頁面，以及以帳號為範圍的頁面：措施與申覆以帳號為單位，所有店家都已關閉的帳號照樣受處分，
/// 必須還能查措施與申覆，所以不要求店家能力）。
///
/// 每一頁只宣告它主要的讀取或操作所屬的能力（頁內的其他動作依各自能力的缺項在頁內顯示閘門），對照 website 的 `pageCapabilities.ts`、
/// `backend/internal/httpapi/seller/declaration.go` 與 gateway 路由宣告的能力。能不能進一頁由
/// `GetMyStore.capabilities` 決定：伺服器依店家狀態推導能力、逐方法檢查，這裡是主動導頁的 UX 捷徑，
/// 不是安全邊界。營業中的店家擁有全部能力，所以這張表只在能力被收回（停權、等待刪除）時才有作用。
///
/// 新增路由常數時在這裡宣告；`route_capabilities_test.dart` 檢查每個路由常數都有宣告。
final Map<String, SellerCapability?> routeCapabilities = {
  AuthRoutes.welcome: null,
  AuthRoutes.auth: null,
  AuthRoutes.onboard: null,
  SellerRoutes.suspensionNotice: null,
  SellerRoutes.storeUnavailable: null,
  SellerRoutes.forceUpdate: null,

  MainRoutes.dashboard: _marketing,
  MainRoutes.category: _storefrontWrite,
  MainRoutes.wallet: _storefrontWrite,
  MainRoutes.orders: _orderFulfill,
  MainRoutes.settings: _accountClose,

  CustomerRoutes.customers: _marketing,
  CustomerRoutes.subscribers: _marketing,
  CustomerRoutes.detail: _marketing,
  CustomerRoutes.spendingHistory: _marketing,
  CustomerRoutes.orderDetail: _orderFulfill,

  OrderRoutes.details: _orderFulfill,
  OrderRoutes.matched: _orderFulfill,
  OrderRoutes.paymentDetails: _orderFulfill,
  OrderRoutes.receipts: _orderFulfill,
  OrderRoutes.receiptsDetail: _orderFulfill,
  OrderRoutes.dispute: _orderFulfill,
  OrderRoutes.logistics: _orderFulfill,
  OrderRoutes.view: _orderFulfill,
  OrderRoutes.matchedDetail: _orderFulfill,

  ProductRoutes.picker: _productManage,
  ProductRoutes.info: _productManage,
  ProductRoutes.images: _productManage,
  ProductRoutes.tags: _productManage,
  ProductRoutes.specs: _productManage,
  ProductRoutes.mediaPicker: _productManage,
  ProductRoutes.preview: _productManage,
  ProductRoutes.reviews: _marketing,

  PostRoutes.mediaPicker: _marketing,
  PostRoutes.compose: _marketing,
  PostRoutes.edit: _marketing,
  PostRoutes.feed: _marketing,

  SellerRoutes.certificationUpload: _storefrontWrite,
  SellerRoutes.photoTools: null,
  SellerRoutes.measures: null,
  SellerRoutes.measureRequest: null,

  CouponRoutes.editor: _marketing,
  CouponRoutes.add: _marketing,
  CouponRoutes.detail: _marketing,

  SettingsRoutes.profile: _accountClose,
  SettingsRoutes.chat: _chatRead,
  SettingsRoutes.chatAiAssistant: _marketing,
  SettingsRoutes.chatThread: _chatRead,
  SettingsRoutes.shipping: _productManage,
  SettingsRoutes.notifications: _notification,
  SettingsRoutes.notificationSettings: _notification,
  SettingsRoutes.help: null,
  SettingsRoutes.myReports: null,
  SettingsRoutes.blockList: null,

  ToolRoutes.photoEditor: null,
  ToolRoutes.richTextEditor: null,
  ToolRoutes.afterSales: _orderFulfill,
  ToolRoutes.afterSalesDetail: _orderFulfill,

  ShipmentAnomalyRoutes.dashboard: _orderFulfill,
  ProcurementRoutes.sellerPublications: _marketing,
  ProcurementRoutes.sellerQuote: _marketing,
};

/// 沒有店家也能進的頁面（帳號層級）：還沒開店的帳號（`needsOnboarding`）導頁時放行這些，其餘一律導去開店頁。
///
/// 開店流程自己（onboard、welcome 的 splash 自行導頁），以及以帳號為範圍的措施列表與申覆表單。每一項在
/// [routeCapabilities] 必須宣告為不需要能力（`route_capabilities_test.dart` 檢查）：沒有店家就沒有店家能力。
const Set<String> routesAllowedWithoutStore = {
  AuthRoutes.onboard,
  AuthRoutes.welcome,
  SellerRoutes.measures,
  SellerRoutes.measureRequest,
};

/// 依 [capabilities] 判斷這個路由現在開放嗎；沒有宣告的路由不開放。
bool isRouteAvailable(
  String location,
  Iterable<SellerCapability> capabilities,
) {
  if (!routeCapabilities.containsKey(location)) return false;
  final required = routeCapabilities[location];
  return required == null || capabilities.contains(required);
}

const _noticeRequirements = {
  SellerRequirement.SELLER_REQUIREMENT_NOT_SUSPENDED,
  SellerRequirement.SELLER_REQUIREMENT_NOT_PENDING_DELETION,
  SellerRequirement.SELLER_REQUIREMENT_STORE_ACTIVE,
};

/// 這個路由現在該不該把賣家導去公告頁：路由宣告的主要能力沒有啟用，而且它缺的條件含未停權、未進入刪除或營業中。
/// 其他缺項（例如 KYC）不導頁，由頁面依相關能力的缺項顯示閘門。還沒讀到店家、伺服器沒有回報這個能力時不擋，交給伺服器判斷。
/// 沒有宣告、或宣告為不需要能力的路由不擋。
bool isBlockedByCapabilities(MyStore? store, String location) {
  if (store == null) return false;
  final required = routeCapabilities[location];
  if (required == null) return false;
  final missing = store.missingFor(required);
  if (missing == null || store.can(required)) return false;
  return missing.any(_noticeRequirements.contains);
}
