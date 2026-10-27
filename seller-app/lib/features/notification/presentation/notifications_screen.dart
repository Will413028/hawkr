import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:hawkr_seller/shared/widgets/hk_glass_app_bar.dart';
import 'package:hawkr_seller/shared/widgets/hk_tap_target.dart';
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../app.dart';
import '../../../config/image_config.dart';
import '../../../core/exceptions/app_exception.dart';
import '../../../shared/widgets/hk_empty_state.dart';
import '../../../shared/widgets/fruit_refresh_indicator.dart';
import '../../../shared/widgets/optimized_image.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../../media/data/media_repository.dart';
import '../../order/data/order_repository.dart';
import '../data/delegation_navigation.dart';
import '../data/notification_model.dart';
import '../data/push/manager.dart';
import 'notification_controller.dart';

final _notificationBuyerMetaProvider =
    FutureProvider.family<_NotificationBuyerMeta?, String>((
      ref,
      orderNo,
    ) async {
      final normalized = orderNo.trim();
      if (normalized.isEmpty) return null;
      final repo = ref.watch(orderRepositoryProvider);
      final detail = await repo.fetchMatchedOrderDetails(orderSn: normalized);
      final avatarUrl = MediaRepository.resolveUrl(detail.buyerAvatar) ?? '';
      final name = detail.buyerName.trim().isNotEmpty
          ? detail.buyerName.trim()
          : detail.recipientName.trim();
      if (name.isEmpty && avatarUrl.isEmpty) return null;
      return _NotificationBuyerMeta(name: name, avatarUrl: avatarUrl);
    });

/// 通知列表頁面
class NotificationsScreen extends HookConsumerWidget {
  const NotificationsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final asyncState = ref.watch(notificationListControllerProvider);
    final scrollController = useScrollController();
    final isDeleteMode = useState(false);
    final selectedIds = useState(<String>{});
    // 每次進入頁面時靜默刷新通知列表（不顯示 loading 遮罩）
    useEffect(() {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        ref.read(notificationListControllerProvider.notifier).silentRefresh();
      });
      return null;
    }, const []);

    // 前景收到 FCM 時即時刷新通知列表，避免賣家停在通知頁時看不到新聊天/訂單。
    useEffect(() {
      Timer? debounce;
      final sub = PushNotificationManager.instance.onInboxMessage.listen((_) {
        debounce?.cancel();
        debounce = Timer(const Duration(milliseconds: 250), () {
          ref.read(notificationListControllerProvider.notifier).silentRefresh();
        });
      });
      return () {
        debounce?.cancel();
        sub.cancel();
      };
    }, const []);

    // 分頁滾動
    useEffect(() {
      void onScroll() {
        if (scrollController.position.pixels >=
            scrollController.position.maxScrollExtent - 200) {
          ref.read(notificationListControllerProvider.notifier).loadMore();
        }
      }

      scrollController.addListener(onScroll);
      return () => scrollController.removeListener(onScroll);
    }, [scrollController]);

    final unreadCount = asyncState.value?.unreadCount ?? 0;

    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      appBar: HkGlassAppBar(
        title: const Text('通知中心'),
        onBackPressed: () => context.pop(),
        actions: [
          // 全部已讀
          if (unreadCount > 0 && !isDeleteMode.value)
            IconButton(
              tooltip: '全部標為已讀',
              icon: Icon(
                AppIcons.checkCircleOutline,
                color: AppIconColors.defaultColor,
                size: 22.sp,
              ),
              onPressed: () async {
                await ref
                    .read(notificationListControllerProvider.notifier)
                    .markAllAsRead();
                if (!context.mounted) return;
                await showDialog<void>(
                  context: context,
                  barrierDismissible: true,
                  builder: (_) => const _MarkAllReadDialog(),
                );
                if (!context.mounted) return;
                ref
                    .read(notificationListControllerProvider.notifier)
                    .silentRefresh();
              },
            ),
          // 刪除模式
          IconButton(
            tooltip: isDeleteMode.value ? '結束選取' : '選取要刪除的通知',
            icon: isDeleteMode.value
                ? Icon(
                    AppIcons.close,
                    color: AppIconColors.defaultColor,
                    size: 22.sp,
                  )
                : Icon(
                    AppIcons.deleteOutline,
                    color: AppIconColors.defaultColor,
                    size: 22.sp,
                  ),
            onPressed: () {
              isDeleteMode.value = !isDeleteMode.value;
              selectedIds.value = {};
            },
          ),
        ],
      ),
      body: Stack(
        children: [
          Positioned.fill(
            child: asyncState.when(
              loading: () => const TruckLoadingOverlay(),
              error: (error, _) => _buildErrorState(context, ref, error),
              data: (state) => FruitRefreshIndicator(
                onRefresh: () => ref
                    .read(notificationListControllerProvider.notifier)
                    .silentRefresh(),
                child: state.notifications.isEmpty
                    ? _buildEmptyState()
                    : ListView.builder(
                        physics: const AlwaysScrollableScrollPhysics(),
                        controller: scrollController,
                        padding: EdgeInsets.only(
                          top: 8.h,
                          bottom: isDeleteMode.value ? 80.h : 16.h,
                        ),
                        itemCount:
                            state.notifications.length +
                            (state.isLoadingMore ? 1 : 0),
                        itemBuilder: (context, index) {
                          if (index == state.notifications.length) {
                            return Padding(
                              padding: EdgeInsets.all(16.w),
                              child: const Center(
                                child: CircularProgressIndicator(
                                  strokeWidth: 2,
                                ),
                              ),
                            );
                          }
                          final n = state.notifications[index];
                          return _NotificationCard(
                            notification: n,
                            isDeleteMode: isDeleteMode.value,
                            isSelected: selectedIds.value.contains('${n.id}'),
                            onSelectionChanged: (selected) {
                              final s = Set<String>.from(selectedIds.value);
                              selected ? s.add('${n.id}') : s.remove('${n.id}');
                              selectedIds.value = s;
                            },
                            onTap: () => _navigateToDetail(context, ref, n),
                          );
                        },
                      ),
              ),
            ),
          ),

          // 刪除模式底部欄
          if (isDeleteMode.value)
            Positioned(
              left: 0,
              right: 0,
              bottom: 0,
              child: _DeleteBottomBar(
                count: selectedIds.value.length,
                onDelete: () async {
                  final ids = selectedIds.value.toList();
                  selectedIds.value = {};
                  isDeleteMode.value = false;
                  await ref
                      .read(notificationListControllerProvider.notifier)
                      .deleteNotifications(ids);
                },
              ),
            ),
        ],
      ),
    );
  }

  void _navigateToDetail(
    BuildContext context,
    WidgetRef ref,
    NotificationItem notification,
  ) {
    if (!notification.isRead) {
      ref
          .read(notificationListControllerProvider.notifier)
          .markAsRead(notification.id);
    }
    final refType = notification.refType?.toUpperCase();
    final refId = notification.refId;
    if (refType == null) return;
    switch (refType) {
      case 'ORDER':
        if (refId != null && refId.isNotEmpty) {
          context.push('/orders/matched/detail', extra: {'orderSn': refId});
        } else {
          // 切到 shell 內的訂單 tab（保留 nav bar）
          context.go('/main/orders');
        }
      case 'CHAT':
        if (refId != null) {
          // 先切到首頁 tab（保留 nav bar），再 push 聊天室
          context.go('/main/dashboard');
          WidgetsBinding.instance.addPostFrameCallback((_) {
            MyApp.navigatorKey.currentContext?.push(
              '/chat/thread',
              extra: {'conversationId': refId},
            );
          });
        } else {
          context.go('/main/dashboard');
          WidgetsBinding.instance.addPostFrameCallback((_) {
            MyApp.navigatorKey.currentContext?.push('/chat');
          });
        }
      case 'AFTERSALE':
      case 'AFTER_SALE':
      case 'REFUND':
        context.go('/main/orders');
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (refId != null && refId.isNotEmpty) {
            MyApp.navigatorKey.currentContext?.push(
              '/after-sales/detail',
              extra: refId,
            );
          } else {
            MyApp.navigatorKey.currentContext?.go(
              '/main/orders?filter=after_sale',
            );
          }
        });

      // 代行營運（56 號設計書）。refId 是 audit_log_id，賣家端沒有代行紀錄頁可
      // 落地，因此改依 data.resourceType 跳到**被操作的資源**；解不出目標就不跳。
      case 'DELEGATION':
        navigateDelegation(context, notification.data);

      // 沒有對應分支＝這一則沒有落地頁。不導航是刻意的：跳到一個猜的頁面比不跳更糟。
      default:
        break;
    }
  }

  Widget _buildErrorState(BuildContext context, WidgetRef ref, Object error) {
    final message = error is AppException ? error.message : '載入失敗，請稍後再試';
    final icon = switch (error) {
      UnauthorizedException() => AppIcons.lockOutline,
      NetworkException() || TimeoutException() => AppIcons.wifiOffOutlined,
      _ => AppIcons.errorOutline,
    };
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(icon, size: 64.sp, color: AppIconColors.muted),
          SizedBox(height: 16.h),
          Text(
            message,
            style: TextStyle(fontSize: 14.sp, color: AppColors.textSecondary),
            textAlign: TextAlign.center,
          ),
          SizedBox(height: 16.h),
          ElevatedButton(
            onPressed: () =>
                ref.read(notificationListControllerProvider.notifier).refresh(),
            style: ElevatedButton.styleFrom(backgroundColor: AppColors.primary),
            child: const HkButtonLabel(
              '重試',
              style: TextStyle(color: AppColors.bgCard),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildEmptyState() {
    return ListView(
      physics: const AlwaysScrollableScrollPhysics(),
      children: const [
        HkEmptyState(
          key: ValueKey('seller-notifications-empty-state'),
          iconKey: ValueKey('seller-notifications-empty-state-icon'),
          icon: HkIconName.notification,
          message: '暫無通知',
          description: '新訂單、訊息與系統提醒會顯示在這裡。',
          topPadding: 88,
        ),
      ],
    );
  }
}

// ══════════════════════════════════════════════════════════════
// 通知卡片
// ══════════════════════════════════════════════════════════════

class _NotificationCard extends ConsumerStatefulWidget {
  final NotificationItem notification;
  final bool isDeleteMode;
  final bool isSelected;
  final ValueChanged<bool>? onSelectionChanged;
  final VoidCallback? onTap;

  const _NotificationCard({
    required this.notification,
    this.isDeleteMode = false,
    this.isSelected = false,
    this.onSelectionChanged,
    this.onTap,
  });

  @override
  ConsumerState<_NotificationCard> createState() => _NotificationCardState();
}

class _NotificationCardState extends ConsumerState<_NotificationCard>
    with SingleTickerProviderStateMixin {
  late final AnimationController _pressCtrl = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 100),
  );
  late final Animation<double> _scale = Tween<double>(
    begin: 1.0,
    end: 0.97,
  ).animate(CurvedAnimation(parent: _pressCtrl, curve: Curves.easeInOut));

  @override
  void dispose() {
    _pressCtrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final n = widget.notification;
    final config = _iconConfig(n);
    final directBuyerName = _buyerNameFromData(n);
    final directBuyerAvatar = _buyerAvatarFromData(n);
    final orderNo = _orderNoFromNotification(n);
    final shouldFetchBuyerMeta =
        (directBuyerName.isEmpty || directBuyerAvatar.isEmpty) &&
        orderNo.isNotEmpty &&
        _isOrderNotification(n);
    final fetchedBuyerMeta = shouldFetchBuyerMeta
        ? ref.watch(_notificationBuyerMetaProvider(orderNo)).value
        : null;
    final buyerName = directBuyerName.isNotEmpty
        ? directBuyerName
        : (fetchedBuyerMeta?.name ?? '');
    final buyerAvatar = directBuyerAvatar.isNotEmpty
        ? directBuyerAvatar
        : (fetchedBuyerMeta?.avatarUrl ?? '');
    final title = _personalize(n.title, n, buyerName: buyerName);
    final body = _personalize(n.body, n, buyerName: buyerName);
    final time = _formatTime(n.createdAt);
    final onActivate = widget.isDeleteMode
        ? () => widget.onSelectionChanged?.call(!widget.isSelected)
        : widget.onTap;

    return HkTapTarget(
      key: ValueKey('notification-row-${n.id}'),
      semanticLabel: widget.isDeleteMode
          ? '通知：$title，$body'
          : '查看通知：$title，$body，$time',
      hint: widget.isDeleteMode
          ? (widget.isSelected ? '取消選取此通知' : '選取此通知')
          : '開啟通知內容',
      checked: widget.isDeleteMode ? widget.isSelected : null,
      button: !widget.isDeleteMode,
      onTap: onActivate,
      child: Listener(
        onPointerDown: (_) => _pressCtrl.forward(),
        onPointerUp: (_) => _pressCtrl.reverse(),
        onPointerCancel: (_) => _pressCtrl.reverse(),
        child: ScaleTransition(
          scale: _scale,
          child: Container(
            margin: EdgeInsets.symmetric(horizontal: 16.w),
            padding: EdgeInsets.symmetric(vertical: 12.h),
            decoration: BoxDecoration(
              border: Border(
                bottom: BorderSide(color: AppColors.dividerLight, width: 1.2),
              ),
            ),
            child: Padding(
              padding: EdgeInsets.symmetric(horizontal: 4.w),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.center,
                children: [
                  // Checkbox（刪除模式）
                  if (widget.isDeleteMode)
                    Padding(
                      padding: EdgeInsets.only(right: 12.w, top: 4.h),
                      child: AnimatedContainer(
                        duration: const Duration(milliseconds: 150),
                        width: 22.w,
                        height: 22.w,
                        decoration: BoxDecoration(
                          color: widget.isSelected
                              ? AppColors.primary
                              : AppColors.bgCard,
                          borderRadius: BorderRadius.circular(6.r),
                          border: Border.all(
                            color: widget.isSelected
                                ? AppColors.primary
                                : AppColors.gray300,
                            width: 2,
                          ),
                        ),
                        child: widget.isSelected
                            ? Icon(
                                AppIcons.check,
                                size: 14.sp,
                                color: AppIconColors.onBrand,
                              )
                            : null,
                      ),
                    ),

                  // 圓形圖標（未讀時右上角顯示紅點）
                  Stack(
                    clipBehavior: Clip.none,
                    children: [
                      Container(
                        width: 34.w,
                        height: 34.w,
                        decoration: BoxDecoration(
                          color: config.color,
                          shape: BoxShape.circle,
                        ),
                        alignment: Alignment.center,
                        child: Icon(
                          config.icon,
                          size: 19.w,
                          color: AppIconColors.onBrand,
                        ),
                      ),
                      if (!n.isRead)
                        Positioned(
                          top: -2,
                          right: -2,
                          child: Container(
                            width: 9.w,
                            height: 9.w,
                            decoration: const BoxDecoration(
                              color: AppColors.error,
                              shape: BoxShape.circle,
                            ),
                          ),
                        ),
                    ],
                  ),
                  SizedBox(width: 12.w),

                  // 內容
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        // 標題 + 時間（同行）
                        Row(
                          children: [
                            Expanded(
                              child: Text(
                                title,
                                style: TextStyle(
                                  fontSize: 16.sp,
                                  fontWeight: FontWeight.w700,
                                  color: AppColors.textEmphasis,
                                ),
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                              ),
                            ),
                            SizedBox(width: 8.w),
                            Text(
                              time,
                              style: TextStyle(
                                fontSize: 12.sp,
                                color: AppColors.textSecondary,
                              ),
                            ),
                          ],
                        ),
                        SizedBox(height: 4.h),
                        // Body — 把通用「買家」替換成具體名字（data 有提供才會替換）
                        Text(
                          body,
                          style: TextStyle(
                            fontSize: 14.sp,
                            color: AppColors.textSecondary,
                            height: 1.4,
                          ),
                          maxLines: 2,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ],
                    ),
                  ),
                  if (buyerAvatar.isNotEmpty) ...[
                    SizedBox(width: 8.w),
                    _NotificationBuyerAvatar(url: buyerAvatar),
                  ],
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }

  static ({IconData icon, Color color}) _iconConfig(NotificationItem n) {
    // 依序嘗試：1) type 精確匹配 → 2) refType → 3) title 關鍵字 → 4) system
    const orderStyle = (
      icon: AppIcons.shoppingBagOutlined,
      color: AppColors.warning,
    );
    const refundStyle = (
      icon: AppIcons.assignmentReturnOutlined,
      color: AppColors.error,
    );
    const chatStyle = (
      icon: AppIcons.chatBubbleOutline,
      color: AppColors.textPrimary,
    );
    const postStyle = (
      icon: AppIcons.descriptionOutlined,
      color: AppColors.primary,
    );
    const userStyle = (icon: AppIcons.personOutline, color: AppColors.primary);
    const couponStyle = (icon: AppIcons.couponOutlined, color: AppColors.error);
    const systemStyle = (
      icon: AppIcons.notificationsOutlined,
      color: AppColors.textPrimary,
    );

    // 1. type 精確匹配
    switch (n.type) {
      case 'CHAT_MESSAGE':
      case 'CHAT_GROUP_INVITE':
        return chatStyle;
      case 'ORDER_CREATED':
      case 'ORDER_PAID':
      case 'ORDER_SHIPPED':
      case 'ORDER_DELIVERED':
      case 'ORDER_COMPLETED':
        return orderStyle;
      case 'AFTER_SALE_STATUS':
      case 'ORDER_CANCELLED':
      case 'ORDER_REFUND':
      case 'ORDER_REFUNDED':
      case 'RETURN_SHIPPED':
      case 'REFUND_REQUESTED':
      case 'REFUND_APPROVED':
      case 'REFUND_REJECTED':
      case 'REFUND_COMPLETED':
        return refundStyle;
      case 'POST_LIKE':
      case 'POST_COMMENT':
      case 'COMMENT_REPLY':
      case 'PRODUCT_VIEW':
      case 'PRODUCT_VISIT':
        return postStyle;
      case 'NEW_FOLLOWER':
      case 'NEW_SUBSCRIBER':
        return userStyle;
      case 'MENTION':
        return postStyle;
      case 'PROMOTION':
      case 'COUPON_RECEIVED':
      case 'COUPON_EXPIRING':
        return couponStyle;
    }

    // 2. refType 匹配
    switch ((n.refType ?? '').toUpperCase()) {
      case 'ORDER':
        return orderStyle;
      case 'REFUND':
      case 'AFTERSALE':
      case 'AFTER_SALE':
        return refundStyle;
      case 'CHAT':
        return chatStyle;
      case 'POST':
      case 'PRODUCT':
        return postStyle;
      case 'USER':
      case 'FOLLOWER':
        return userStyle;
      case 'COUPON':
        return couponStyle;
    }

    // 3. title 關鍵字 fallback（後端 type 未覆蓋時的最後防線）
    final title = n.title;
    if (title.contains('退貨') || title.contains('退款') || title.contains('售後')) {
      return refundStyle;
    }
    if (title.contains('訂單') ||
        title.contains('下單') ||
        title.contains('出貨') ||
        title.contains('撥款')) {
      return orderStyle;
    }
    if (title.contains('瀏覽') ||
        title.contains('按讚') ||
        title.contains('留言') ||
        title.contains('商機') ||
        title.contains('商品')) {
      return postStyle;
    }
    if (title.contains('聊天') || title.contains('訊息')) {
      return chatStyle;
    }
    if (title.contains('追蹤') || title.contains('粉絲') || title.contains('收藏')) {
      return userStyle;
    }
    if (title.contains('優惠') || title.contains('折價') || title.contains('券')) {
      return couponStyle;
    }

    return systemStyle;
  }

  static String _formatTime(DateTime dateTime) {
    final diff = DateTime.now().difference(dateTime);
    if (diff.inMinutes < 1) return '剛剛';
    if (diff.inHours < 1) return '${diff.inMinutes} 分鐘前';
    if (diff.inDays < 1) return '${diff.inHours} 小時前';
    if (diff.inDays < 7) return '${diff.inDays} 天前';
    return '${dateTime.month}/${dateTime.day}';
  }

  /// 從 notification.data 取得買家名稱（多種後端 key fallback）。
  /// 沒有的話回傳空字串，caller 用原文 fallback。
  static String _buyerNameFromData(NotificationItem n) {
    return _firstDataString(n.data, const [
      'buyerNickname',
      'buyer_nickname',
      'buyerName',
      'buyer_name',
      'buyerNickName',
      'buyer_nick_name',
      'nickname',
      'userName',
      'user_name',
      'senderName',
      'sender_name',
      'fromName',
      'from_name',
    ]);
  }

  static String _buyerAvatarFromData(NotificationItem n) {
    final raw = _firstDataString(n.data, const [
      'buyerAvatarUrl',
      'buyer_avatar_url',
      'buyerAvatar',
      'buyer_avatar',
      'userAvatar',
      'user_avatar',
      'avatarUrl',
      'avatar_url',
      'senderAvatar',
      'sender_avatar',
      'senderAvatarUrl',
      'sender_avatar_url',
      'fromAvatar',
      'from_avatar',
    ]);
    return MediaRepository.resolveUrl(raw) ?? raw;
  }

  static String _orderNoFromNotification(NotificationItem n) {
    final refType = (n.refType ?? '').toUpperCase();
    if (refType == 'ORDER' && (n.refId ?? '').isNotEmpty) {
      return n.refId!.trim();
    }
    return _firstDataString(n.data, const [
      'orderNo',
      'orderSn',
      'order_sn',
      'orderId',
      'refId',
      'ref_id',
    ]);
  }

  static bool _isOrderNotification(NotificationItem n) {
    final type = n.type.toUpperCase();
    final refType = (n.refType ?? '').toUpperCase();
    final category = _firstDataString(n.data, const ['category']).toUpperCase();
    return refType == 'ORDER' ||
        type.contains('ORDER') ||
        category.contains('ORDER');
  }

  static String _firstDataString(
    Map<String, dynamic>? data,
    List<String> keys,
  ) {
    if (data == null) return '';
    for (final source in [
      data,
      if (data['payload'] is Map<String, dynamic>)
        data['payload'] as Map<String, dynamic>,
      if (data['metadata'] is Map<String, dynamic>)
        data['metadata'] as Map<String, dynamic>,
      if (data['meta'] is Map<String, dynamic>)
        data['meta'] as Map<String, dynamic>,
    ]) {
      for (final key in keys) {
        final v = source[key];
        if (v == null) continue;
        final s = v.toString().trim();
        if (s.isNotEmpty) return s;
      }
    }
    return '';
  }

  /// 把 notification 文字裡的「買家」替換成具體買家名（如果 data 有提供）。
  /// 後端通知模板統一寫「買家已下單 / 買家已支付」等通用文字，
  /// 前端拿 data.buyerNickname 等欄位做 client-side 個人化顯示。
  /// data 沒給就 fallback 顯示原文。
  static String _personalize(
    String text,
    NotificationItem n, {
    String? buyerName,
  }) {
    if (text.isEmpty) return text;
    final providedName = buyerName?.trim() ?? '';
    final name = providedName.isNotEmpty ? providedName : _buyerNameFromData(n);
    if (name.isEmpty) return text;
    return text.replaceAll('買家', name);
  }
}

class _NotificationBuyerMeta {
  const _NotificationBuyerMeta({required this.name, required this.avatarUrl});

  final String name;
  final String avatarUrl;
}

class _NotificationBuyerAvatar extends StatelessWidget {
  const _NotificationBuyerAvatar({required this.url});

  final String url;

  @override
  Widget build(BuildContext context) {
    final imageUrl = MediaRepository.resolveUrl(url) ?? url;
    return ClipOval(
      child: OptimizedNetworkImage(
        imageUrl: imageUrl,
        scene: ImageScene.avatarSmall,
        width: 32.w,
        height: 32.w,
        fit: BoxFit.cover,
        errorWidget: Container(
          width: 32.w,
          height: 32.w,
          decoration: const BoxDecoration(
            color: AppColors.primarySoft,
            shape: BoxShape.circle,
          ),
          child: Icon(
            AppIcons.personOutline,
            size: 18.sp,
            color: AppIconColors.secondary,
          ),
        ),
      ),
    );
  }
}

// ══════════════════════════════════════════════════════════════
// 全部已讀成功彈窗
// ══════════════════════════════════════════════════════════════

class _MarkAllReadDialog extends StatelessWidget {
  const _MarkAllReadDialog();

  @override
  Widget build(BuildContext context) {
    return Dialog(
      backgroundColor: Colors.transparent,
      elevation: 0,
      child: Container(
        width: 260.w,
        padding: EdgeInsets.symmetric(horizontal: 24.w, vertical: 24.h),
        decoration: BoxDecoration(
          color: AppColors.bgCard,
          borderRadius: BorderRadius.circular(16.r),
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              width: 56.w,
              height: 56.w,
              decoration: BoxDecoration(
                color: AppColors.primarySoft,
                shape: BoxShape.circle,
              ),
              child: Icon(
                AppIcons.checkRounded,
                size: 30.sp,
                color: AppIconColors.active,
              ),
            ),
            SizedBox(height: 16.h),
            Text(
              '已將全部通知設為已讀',
              style: TextStyle(
                fontSize: 16.sp,
                fontWeight: FontWeight.w700,
                color: AppColors.textEmphasis,
              ),
            ),
            SizedBox(height: 6.h),
            Text(
              '所有未讀通知已標記完畢',
              style: TextStyle(fontSize: 14.sp, color: AppColors.textSecondary),
            ),
            SizedBox(height: 22.h),
            SizedBox(
              width: double.infinity,
              height: 48,
              child: ElevatedButton(
                onPressed: () => Navigator.of(context).pop(),
                style: ElevatedButton.styleFrom(
                  backgroundColor: AppColors.primary,
                  elevation: 0,
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(999.r),
                  ),
                ),
                child: HkButtonLabel(
                  '確定',
                  style: TextStyle(
                    fontSize: 14.sp,
                    fontWeight: FontWeight.w700,
                    color: AppColors.bgCard,
                  ),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

// ══════════════════════════════════════════════════════════════
// 刪除模式底部欄
// ══════════════════════════════════════════════════════════════

class _DeleteBottomBar extends StatelessWidget {
  final int count;
  final VoidCallback onDelete;
  const _DeleteBottomBar({required this.count, required this.onDelete});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: EdgeInsets.fromLTRB(
        24.w,
        12.h,
        24.w,
        MediaQuery.of(context).padding.bottom + 12.h,
      ),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        boxShadow: [
          BoxShadow(
            color: AppColors.cardShadow,
            blurRadius: 8,
            offset: const Offset(0, -2),
          ),
        ],
      ),
      child: SizedBox(
        width: double.infinity,
        height: 48,
        child: ElevatedButton(
          onPressed: count > 0 ? onDelete : null,
          style: ElevatedButton.styleFrom(
            backgroundColor: AppColors.error,
            disabledBackgroundColor: AppColors.dividerLight,
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(12.r),
            ),
          ),
          child: HkButtonLabel(
            count > 0 ? '刪除 ($count)' : '請選擇要刪除的通知',
            style: TextStyle(
              color: AppColors.bgCard,
              fontSize: 16.sp,
              fontWeight: FontWeight.w700,
            ),
          ),
        ),
      ),
    );
  }
}
