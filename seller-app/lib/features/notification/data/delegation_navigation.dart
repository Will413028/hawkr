import 'package:flutter/widgets.dart';
import 'package:go_router/go_router.dart';

import '../../../app.dart';
import '../../../core/router/app_router.dart';
import '../../post/data/post_model.dart';

/// 代行營運通知（56 號設計書）的落地頁解析 —— 收件匣與 FCM 兩條路徑共用同一份。
///
/// ## 後端契約
/// `services/notification-service/db/migrations/`
/// `20260816093000_delegation_template_ref_fields.sql:63-73` 的 `data_fields`：
///
/// | 鍵 | 值 |
/// |---|---|
/// | `refType` | 常數 `"DELEGATION"` |
/// | `refId` | `audit_log_id` |
/// | `resourceType` | `storefront` \| `product` \| `post` \| `order`（小寫封閉四值） |
/// | `resourceId` | TSID 字串；**訂單面恆為空** |
/// | `resourceName` | 訂單面＝訂單編號；其餘＝商品名／貼文摘要／店名 |
///
/// FCM 那一側的 `data.type` 另外恆為 `DELEGATION_OPERATION_COMPLETED`
/// （同目錄 `20260815130000_...template.sql:31` 的 `data_type`），且
/// `schema_url` 是 NULL ⇒ **沒有 `data.url` 可用**，只能走 App 內部路由
/// （`Info.plist` 也沒有註冊 `hawkr` scheme，`canLaunchUrl` 會回 false）。
///
/// ## 落地頁怎麼選
/// 賣家端本期沒有「代行紀錄」頁（56 §2.2 第 10 條），所以目標是**被操作的資源**。
/// 但 App 的路由表裡只有訂單有「依 id 開詳情」的入口：
///
/// | resourceType | 落地 | 精確度 |
/// |---|---|---|
/// | `order` | [OrderRoutes.matchedDetail]，`extra: {orderSn: resourceName}` | 該筆訂單 |
/// | `storefront` | [MainRoutes.category]（店面頁，店名／封面／簡介都在 header） | 該資源本身 |
/// | `product` | [MainRoutes.category]（同頁預設就是商品 tab，被改的商品在列表裡） | 只到列表 |
/// | `post` | [PostRoutes.feed]（賣家自己的貼文動態），`extra: {postId: resourceId}` | 該篇貼文；resourceId 不是數字時停在第一篇 |
///
/// 商品停在列表，是因為 App 沒有「用 id 開商品」的路由：`/product/*` 整條是需要完整
/// `existingProduct` map 的建立精靈（`app_router.dart:741-951`），硬湊一個會落在
/// `_invalidRouteData`。貼文動態吃 `postId`，在載入的清單裡找序位。
///
/// 解不出目標一律回 `null`，呼叫端**什麼都不做** —— 錯誤跳轉比不跳更糟。
@immutable
class DelegationNavTarget {
  const DelegationNavTarget({
    required this.shellPath,
    this.pushPath,
    this.pushExtra,
  });

  /// 先切到的 shell tab（保留底部導覽列）。
  final String shellPath;

  /// 再疊上去的子頁；`null` 表示只切 tab。
  final String? pushPath;

  /// [pushPath] 的 `extra`。
  final Map<String, Object?>? pushExtra;
}

/// 由通知的 `data` 解出落地頁；解不出回 `null`。
DelegationNavTarget? resolveDelegationTarget(Map<String, dynamic>? data) {
  final resourceType = data?['resourceType']?.toString().trim().toLowerCase();
  if (resourceType == null || resourceType.isEmpty) return null;

  final resourceName = data?['resourceName']?.toString().trim() ?? '';

  switch (resourceType) {
    // 訂單的 resourceId 恆為空（稽核表 resource_id 是 bigint，`O` 開頭的單號存不
    // 進去），單號只在 resourceName ——「用 refId 開訂單」在這裡是行不通的。
    case 'order':
      if (resourceName.isEmpty) return null;
      return DelegationNavTarget(
        shellPath: MainRoutes.orders,
        pushPath: OrderRoutes.matchedDetail,
        pushExtra: {'orderSn': resourceName},
      );

    // 店面頁：header 就是被 STOREFRONT_UPDATE 改到的店名／封面／簡介／位置。
    case 'storefront':
      return const DelegationNavTarget(shellPath: MainRoutes.category);

    // 同一頁的預設 tab（`page_screen.dart:154-157` initialIndex: 1）就是商品列表。
    case 'product':
      return const DelegationNavTarget(shellPath: MainRoutes.category);

    case 'post':
      return DelegationNavTarget(
        shellPath: MainRoutes.category,
        pushPath: PostRoutes.feed,
        pushExtra: postFeedExtra(data?['resourceId']?.toString()),
      );

    default:
      return null;
  }
}

/// 解析並執行導航；解不出目標時什麼都不做，回傳 `false`。
bool navigateDelegation(BuildContext context, Map<String, dynamic>? data) {
  final target = resolveDelegationTarget(data);
  if (target == null) return false;

  // 先切 shell tab 保留底部導覽列，下一幀再疊子頁 —— 與本檔案兩個呼叫端既有的
  // ORDER / CHAT / AFTERSALE 分支同一種節奏。
  context.go(target.shellPath);
  final pushPath = target.pushPath;
  if (pushPath == null) return true;
  WidgetsBinding.instance.addPostFrameCallback((_) {
    MyApp.navigatorKey.currentContext?.push(pushPath, extra: target.pushExtra);
  });
  return true;
}
