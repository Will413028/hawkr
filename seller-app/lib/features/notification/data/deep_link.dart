import 'package:flutter/widgets.dart';
import 'package:go_router/go_router.dart';

import '../../../app.dart';
import '../../../core/router/app_router.dart';
import '../../chat/data/active_chat_gate.dart';

/// 通知的深連結（`InboxItem.deep_link` 與推播 data 的 `deep_link`）是 website 的相對路徑，
/// 格式與路徑表見 `backend/docs/plans/2026-11-23-a8-notification/deep-links.md`（D20）。
/// 這個類別把賣家（STORE）受眾的路徑對到 App 的畫面：先去掉語系前綴 `/en`，再依第一段與段數對照；
/// 空字串、外部連結（含 scheme、host、`//` 開頭）與不認得的路徑都落在收件匣。
@immutable
class NotificationDeepLink {
  const NotificationDeepLink._(this._segments, this._rejected);

  final List<String> _segments;
  final bool _rejected;

  /// 沒有目的地（空字串、null）。
  static const empty = NotificationDeepLink._([], false);

  static NotificationDeepLink parse(String? link) {
    if (link == null || link.isEmpty) return empty;
    if (!link.startsWith('/') || link.startsWith('//')) return _reject;
    final uri = Uri.tryParse(link);
    if (uri == null || uri.hasScheme || uri.hasAuthority) return _reject;
    var segments = uri.pathSegments.where((s) => s.isNotEmpty).toList();
    if (segments.isNotEmpty && segments.first == 'en') {
      segments = segments.sublist(1);
    }
    return NotificationDeepLink._(segments, false);
  }

  static const _reject = NotificationDeepLink._([], true);

  /// 連結本身沒有目的地（空字串）。
  bool get isEmpty => _segments.isEmpty && !_rejected;

  /// 會落在收件匣（空字串、被拒絕的外部連結、不認得的路徑）。從收件匣點項目時用它避免再開一個收件匣。
  bool get opensInbox => _destination is _Inbox;

  /// 連結指向的聊天對象（`/seller-center/messages/{targetId}` 的 `targetId` 是買家帳號 id）；不是聊天室連結回 null。
  String? get chatBuyerId =>
      _destination is _Chat ? (_destination as _Chat).id : null;

  /// 使用者目前開著的聊天室就是這條連結指向的買家（前景推播據此不彈 banner）。
  bool isOpenIn(ActiveChat active) {
    final buyerId = chatBuyerId;
    return buyerId != null && active.peerId == buyerId;
  }

  _Destination get _destination {
    final s = _segments;
    if (_rejected || s.isEmpty) return const _Inbox();
    switch (s.first) {
      case 'seller-center':
        if (s.length == 3 && s[1] == 'messages') return _Chat(s[2]);
        if (s.length == 3 && s[1] == 'orders') return _Order(s[2]);
        if (s.length == 2 && s[1] == 'wallet') return const _Wallet();
        if (s.length == 2 && s[1] == 'suspended') return const _Suspended();
        return const _Inbox();
      case 'seller':
        if (s.length == 2 && s[1] == 'onboarding') return const _Kyc();
        if (s.length >= 3 && s[1] == 'measures') {
          final id = s[2];
          if (s.length == 3) return _Measure(id, remediation: false);
          if (s.length == 4 && s[3] == 'appeal') {
            return _Measure(id, remediation: false);
          }
          if (s.length == 4 && s[3] == 'remediation') {
            return _Measure(id, remediation: true);
          }
        }
        if (s.length == 3 && s[1] == 'inquiries') return _Inquiry(s[2]);
        return const _Inbox();
      case 'sellers':
        if (s.length == 4 && s[2] == 'messages') return const _Forum();
        return const _Inbox();
      default:
        return const _Inbox();
    }
  }

  /// 開啟目的地。
  void open() {
    final ctx = MyApp.navigatorKey.currentContext;
    if (ctx == null) return;
    switch (_destination) {
      case _Chat(:final id):
        _overShell(
          ctx,
          MainRoutes.dashboard,
          (c) =>
              c.push(SettingsRoutes.chatThread, extra: {'participantId': id}),
        );
      case _Order(:final orderNo):
        _overShell(
          ctx,
          MainRoutes.orders,
          (c) => c.push(OrderRoutes.matchedDetail, extra: {'orderSn': orderNo}),
        );
      case _Wallet():
        ctx.go(MainRoutes.wallet);
      case _Suspended():
        ctx.push(SellerRoutes.suspensionNotice);
      case _Kyc():
        ctx.push(SellerRoutes.certificationUpload);
      case _Measure(:final id, :final remediation):
        ctx.push(
          remediation
              ? SellerRoutes.measureRemediationFor(id)
              : SellerRoutes.measureRequestFor(id),
        );
      case _Inquiry(:final id):
        ctx.push(SellerRoutes.inquiryDetailFor(id));
      case _Forum():
        ctx.go(MainRoutes.category);
      case _Inbox():
        _overShell(
          ctx,
          MainRoutes.dashboard,
          (c) => c.push(SettingsRoutes.notifications),
        );
    }
  }

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
}

sealed class _Destination {
  const _Destination();
}

class _Inbox extends _Destination {
  const _Inbox();
}

class _Chat extends _Destination {
  const _Chat(this.id);
  final String id;
}

class _Order extends _Destination {
  const _Order(this.orderNo);
  final String orderNo;
}

class _Wallet extends _Destination {
  const _Wallet();
}

class _Suspended extends _Destination {
  const _Suspended();
}

class _Kyc extends _Destination {
  const _Kyc();
}

class _Measure extends _Destination {
  const _Measure(this.id, {required this.remediation});
  final String id;
  final bool remediation;
}

class _Inquiry extends _Destination {
  const _Inquiry(this.id);
  final String id;
}

class _Forum extends _Destination {
  const _Forum();
}
