import 'package:flutter/widgets.dart';
import 'package:go_router/go_router.dart';
import 'package:url_launcher/url_launcher.dart';

import '../../../../app.dart';
import '../../../../core/router/app_router.dart';
import '../../../../shared/utils/hk_log.dart';
import '../../../order/data/after_sale_status_notifier.dart';
import '../../../order/data/order_status_notifier.dart';
import '../../../post/data/post_model.dart';
import '../delegation_navigation.dart';
import 'seller_status_push.dart';

/// 通知導航路由（純邏輯，無狀態）
///
/// 根據 FCM data payload 中的 type 執行對應頁面跳轉。
/// 所有路由規則集中於此，方便維護和擴展。
class NotificationRouter {
  NotificationRouter._();

  /// 根據通知 data 導航至對應頁面
  ///
  /// 來源：
  /// - onMessageOpenedApp（後台點擊）
  /// - getInitialMessage（FCM 冷啟動）
  /// - getNotificationAppLaunchDetails（本地通知冷啟動）
  ///
  /// Type → 目標頁對照表（契約見後端 issue B-75 / #434 與 B-130 / #626）：
  /// - ORDER / ORDER_PAID / NEW_ORDER / ORDER_CREATED / ORDER_SHIPPED /
  ///   ORDER_DELIVERED / ORDER_COMPLETED / ORDER_CANCELLED / ORDER_STATUS
  ///     → /orders/matched/detail (帶 orderSn)，並 emit OrderStatusNotifier 事件
  ///       讓已打開的訂單列表/同訂單詳情頁自動 refresh；無 orderSn 時退回 /main/orders
  /// - AFTER_SALE_REQUEST / AFTER_SALE_STATUS / REFUND_REQUESTED
  ///     → /after-sales/detail (帶 caseNo)；無 caseNo 時退回配送中 + 售後處理篩選
  /// - CERTIFICATION_* / MERCHANT_VERIFICATION / MERCHANT.VERIFICATION.* / SELLER_REVIEW_STATUS /
  ///   PAYUNI_STATUS → 導向 /main/wallet（重讀由 manager 負責）
  /// - CHAT_MESSAGE → /chat/thread (帶 conversationId)
  /// - NEW_FOLLOWER / NEW_SUBSCRIBER → /subscribers
  /// - STOCK_LOW / INVENTORY_LOW → /main/category（商品 tab）
  /// - POST_COMMENT / POST_LIKE / COMMENT_REPLY → /post/feed（商家貼文 feed，開到 refId 那一篇）
  /// - DELEGATION_OPERATION_COMPLETED / DELEGATION → 依 data.resourceType 跳到
  ///   被代行操作的資源，對照表見 `delegation_navigation.dart`；解不出就不跳
  /// - SYSTEM_MAINTENANCE / TERMS_UPDATE → 無跳轉
  static void navigate(Map<String, dynamic> data) {
    // type / refType / eventType 任一欄位都可當事件 key（後端 silent push
    // 實際會用 refType: "ORDER_STATUS" + eventType: "ORDER_PAID" 組合）
    final eventType = data['eventType']?.toString();
    final rawType = isSellerStatusPushType(eventType)
        ? eventType
        : (data['type'] ?? data['refType'] ?? eventType)?.toString();
    final refId = data['refId']?.toString();

    HKLog.logD('[NotificationRouter] type=$rawType, refId=$refId');

    if (rawType == null || rawType.isEmpty) return;

    final context = MyApp.navigatorKey.currentContext;
    final normalized = rawType.toUpperCase();

    // 認證與 PayUni 狀態的推播：到錢包看結果（重讀由 manager 在事件進來時做，這裡只導覽）。
    if (isSellerStatusPushType(normalized)) {
      HKLog.logI('[NotificationRouter] → 錢包 ($normalized)');
      context?.go(MainRoutes.wallet);
      return;
    }

    switch (normalized) {
      // ── 訂單類（賣家收到：新訂單 / 出貨 / 送達 / 狀態更新）─────
      // 先 push 訂單詳情讓 ref.watch 取最新資料，下一幀再 emit stream 事件，
      // 保證新詳情頁 ref.listen 已註冊、已打開的訂單列表 / dashboard 也能收到事件刷新。
      case 'ORDER':
      case 'ORDER_PAID':
      case 'NEW_ORDER':
      case 'ORDER_CREATED':
      case 'ORDER_SHIPPED':
      case 'ORDER_DELIVERED':
      case 'ORDER_COMPLETED':
      case 'ORDER_CANCELLED':
      case 'ORDER_STATUS':
        HKLog.logI('[NotificationRouter] → 訂單詳情 ($normalized)');
        _gotoOrderDetail(context, data, refId);
        WidgetsBinding.instance.addPostFrameCallback((_) {
          OrderStatusNotifier.instance.handlePushNotification(data);
        });

      // ── 售後類（買家申請 / 狀態異動）──────────────────
      case 'AFTERSALE':
      case 'AFTER_SALE':
      case 'AFTER_SALE_REQUEST':
      case 'AFTER_SALE_STATUS':
      case 'RETURN_SHIPPED':
      case 'REFUND_REQUESTED':
      case 'REFUND_APPROVED':
      case 'REFUND_REJECTED':
      case 'REFUND_COMPLETED':
      case 'ORDER_REFUNDED':
        HKLog.logI('[NotificationRouter] → 售後詳情 ($normalized)');
        AfterSaleStatusNotifier.instance.handlePushNotification(data);
        final caseNo = data['caseNo']?.toString() ?? refId;
        context?.go('/main/orders');
        WidgetsBinding.instance.addPostFrameCallback((_) {
          final ctx = MyApp.navigatorKey.currentContext;
          if (caseNo != null && caseNo.isNotEmpty) {
            ctx?.push('/after-sales/detail', extra: caseNo);
          } else {
            ctx?.go('/main/orders?filter=after_sale');
          }
        });

      // ── 聊天類 ─────────────────────────────────────
      case 'CHAT_MESSAGE':
        final conversationId = data['conversation_id']?.toString() ?? refId;
        HKLog.logI('[NotificationRouter] → 聊天室: $conversationId');
        context?.go('/main/dashboard');
        WidgetsBinding.instance.addPostFrameCallback((_) {
          final ctx = MyApp.navigatorKey.currentContext;
          if (conversationId != null && ctx != null) {
            ctx.push('/chat/thread', extra: {'conversationId': conversationId});
          } else {
            ctx?.push('/chat');
          }
        });

      // ── 粉絲 / 訂閱者 ──────────────────────────────
      case 'NEW_FOLLOWER':
      case 'NEW_SUBSCRIBER':
        HKLog.logI('[NotificationRouter] → 訂閱者 ($normalized)');
        context?.push('/subscribers');

      // ── 商品被收藏 / 被瀏覽（導到客戶頁，目前沒有具體 customerId → 訂閱者列表）
      case 'PRODUCT_FAVORITED':
      case 'PRODUCT_VIEWED':
      case 'BUYER_INTENT':
        HKLog.logI('[NotificationRouter] → 客戶列表 ($normalized)');
        context?.push('/customers');

      // ── 款項撥付通知（訂單結算入帳）──────────────
      case 'PAYOUT_COMPLETED':
      case 'WALLET_CREDITED':
        HKLog.logI('[NotificationRouter] → 錢包 tab ($normalized)');
        context?.go('/main/wallet');

      // ── 商品庫存 ───────────────────────────────────
      case 'STOCK_LOW':
      case 'INVENTORY_LOW':
      case 'PRODUCT_STOCK_LOW':
        HKLog.logI('[NotificationRouter] → 商品 tab ($normalized)');
        context?.go('/main/category');

      // ── 貼文互動（賣家收到買家的按讚 / 留言 / 回覆 / 點讚）
      case 'POST_LIKE':
      case 'POST_COMMENT':
      case 'POST_NEW_COMMENT':
      case 'POST_COMMENT_REPLY':
      case 'COMMENT_REPLY':
      case 'COMMENT_LIKED':
        _gotoPostFeed(context, normalized, refId);

      // ── 賣家規範更新（資訊性系統公告）──────────────
      case 'SELLER_TERMS_UPDATE':
      case 'SELLER_POLICY_UPDATE':
        HKLog.logI('[NotificationRouter] → 系統公告 ($normalized)');
      // 目前無獨立系統公告頁，留待後續補上

      // ── 代行營運（56 號設計書）─────────────────────
      // data.type 恆為 DELEGATION_OPERATION_COMPLETED（模板的 data_type），
      // data.refType 為 DELEGATION；兩個都收，避免只靠其中一個。
      // schema_url 是 NULL ⇒ 沒有 data.url，default 分支只會印一行 log 就結束，
      // 所以這裡要走 App 內部路由（Info.plist 也沒註冊 hawkr scheme）。
      // 分流與收件匣共用 `resolveDelegationTarget`，兩條路徑不會分岔。
      case 'DELEGATION_OPERATION_COMPLETED':
      case 'DELEGATION':
        HKLog.logI(
          '[NotificationRouter] → 代行資源 (${data['resourceType'] ?? '未提供'})',
        );
        if (context != null && !navigateDelegation(context, data)) {
          HKLog.logD('[NotificationRouter] 代行通知無可落地資源，維持現狀');
        }

      // ── 系統類（資訊性通知）────────────────────────
      case 'SYSTEM_MAINTENANCE':
      case 'TERMS_UPDATE':
        HKLog.logI('[NotificationRouter] → 資訊性通知 ($normalized)');

      default:
        final url = data['url']?.toString();
        if (url != null && url.isNotEmpty) {
          HKLog.logI('[NotificationRouter] → 外部 URL ($normalized): $url');
          _openUrl(url);
        } else {
          HKLog.logD('[NotificationRouter] 未知類型: $rawType');
        }
    }
  }

  static Future<void> _openUrl(String url) async {
    final uri = Uri.tryParse(url);
    if (uri == null) {
      HKLog.logD('[NotificationRouter] 無效 url: $url');
      return;
    }
    if (await canLaunchUrl(uri)) {
      await launchUrl(uri, mode: LaunchMode.externalApplication);
    }
  }

  /// 賣家自己的貼文動態，開到 [refId] 那一篇；refId 缺值或不在清單裡時停在第一篇。
  static void _gotoPostFeed(
    BuildContext? context,
    String normalized,
    String? refId,
  ) {
    HKLog.logI('[NotificationRouter] → 商家貼文 feed ($normalized)');
    context?.push(PostRoutes.feed, extra: postFeedExtra(refId));
  }

  static void _gotoOrderDetail(
    BuildContext? context,
    Map<String, dynamic> data,
    String? refId,
  ) {
    final orderSn =
        data['orderId']?.toString() ??
        data['orderSn']?.toString() ??
        data['orderNo']?.toString() ??
        refId;
    if (orderSn != null && orderSn.isNotEmpty) {
      HKLog.logI('[NotificationRouter] → 訂單詳情: $orderSn');
      context?.push('/orders/matched/detail', extra: {'orderSn': orderSn});
    } else {
      HKLog.logI('[NotificationRouter] → 訂單列表（缺 orderSn）');
      context?.go('/main/orders');
    }
  }
}
