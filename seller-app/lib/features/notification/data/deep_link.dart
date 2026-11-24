import 'package:flutter/widgets.dart';
import 'package:go_router/go_router.dart';

import '../../../app.dart';
import '../../../core/router/app_router.dart';
import '../../post/data/post_model.dart';

/// 通知的深連結（伺服器在 `InboxItem.deep_link` 與推播的 `deep_link` 給）：App 內路徑加查詢參數，
/// 例如 `/orders/matched/detail?orderSn=O123`。client 只把它對到現有路由，不依通知型別分支；
/// 對不到路由或空字串＝沒有目的地，什麼都不做。
@immutable
class NotificationDeepLink {
  const NotificationDeepLink._(this.uri);

  final Uri uri;

  static NotificationDeepLink? parse(String? link) {
    if (link == null || link.isEmpty) return null;
    final uri = Uri.tryParse(link);
    if (uri == null || !uri.path.startsWith('/')) return null;
    return NotificationDeepLink._(uri);
  }

  String? get _conversationId => uri.path == SettingsRoutes.chatThread
      ? _nonEmpty(uri.queryParameters['conversationId'])
      : null;

  /// 連結指向的聊天室；不是聊天室連結回 null。前景推播用它判斷使用者是否已在那個聊天室。
  String? get chatConversationId => _conversationId;

  /// 開啟目的地；回傳是否對到路由。
  bool open() {
    final ctx = MyApp.navigatorKey.currentContext;
    if (ctx == null) return false;
    final q = uri.queryParameters;
    switch (uri.path) {
      case MainRoutes.dashboard:
      case MainRoutes.orders:
      case MainRoutes.wallet:
      case MainRoutes.category:
        ctx.go(uri.hasQuery ? uri.toString() : uri.path);
      case OrderRoutes.matchedDetail:
        final orderSn = _nonEmpty(q['orderSn']);
        if (orderSn == null) return false;
        _overShell(
          ctx,
          MainRoutes.orders,
          (c) => c.push(OrderRoutes.matchedDetail, extra: {'orderSn': orderSn}),
        );
      case ToolRoutes.afterSalesDetail:
        final caseNo = _nonEmpty(q['caseNo']);
        if (caseNo == null) return false;
        _overShell(
          ctx,
          MainRoutes.orders,
          (c) => c.push(ToolRoutes.afterSalesDetail, extra: caseNo),
        );
      case SettingsRoutes.chatThread:
        final id = _conversationId;
        if (id == null) return false;
        _overShell(
          ctx,
          MainRoutes.dashboard,
          (c) =>
              c.push(SettingsRoutes.chatThread, extra: {'conversationId': id}),
        );
      case PostRoutes.feed:
        _overShell(
          ctx,
          MainRoutes.category,
          (c) => c.push(
            PostRoutes.feed,
            extra: postFeedExtra(_nonEmpty(q['postId'])),
          ),
        );
      case CustomerRoutes.subscribers:
      case CustomerRoutes.customers:
        ctx.push(uri.path);
      default:
        return false;
    }
    return true;
  }

  // 先切 shell tab 保留底部導覽列，下一幀再疊子頁。
  static void _overShell(
    BuildContext ctx,
    String shell,
    void Function(BuildContext) push,
  ) {
    ctx.go(shell);
    WidgetsBinding.instance.addPostFrameCallback((_) {
      final next = MyApp.navigatorKey.currentContext;
      if (next != null) push(next);
    });
  }

  static String? _nonEmpty(String? v) =>
      (v == null || v.trim().isEmpty) ? null : v.trim();
}
