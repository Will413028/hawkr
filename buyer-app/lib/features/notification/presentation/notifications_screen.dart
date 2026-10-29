import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../config/image_config.dart';
import 'widgets/notification_type_icon.dart';
import '../../../core/exceptions/app_exception.dart';
import '../../../shared/widgets/app_cached_image.dart';
import '../../../shared/widgets/common_top_bar.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../../shared/widgets/hk_empty_state.dart';
import '../../../shared/widgets/hk_tap_target.dart';
import '../../../shared/widgets/fruit_refresh_indicator.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../../order/data/order_repository.dart';
import '../../seller/data/store_card_display.dart';
import '../../seller/data/store_card_provider.dart';
import '../data/notification_model.dart';
import '../data/push/manager.dart';
import 'notification_controller.dart';
import 'package:hawkr_buyer/shared/widgets/hk_brand_loader.dart';

/// Injectable FCM inbox stream so the notification surface can be exercised
/// without bootstrapping Firebase in widget tests.
final notificationInboxStreamProvider = Provider<Stream<Object?>>(
  (_) => PushNotificationManager.instance.onInboxMessage,
);

final _notificationSenderMetaProvider =
    FutureProvider.family<_NotificationSenderMeta?, String>((
      ref,
      lookupKey,
    ) async {
      final parts = lookupKey.split(':');
      if (parts.length != 2 || parts[1].trim().isEmpty) return null;
      final kind = parts[0];
      final id = parts[1].trim();

      if (kind == 'order') {
        final order = await ref
            .watch(orderRepositoryProvider)
            .getOrderDetail(id);
        final name = order.sellerName.trim();
        final avatar = order.sellerAvatarUrl.trim();
        if (name.isEmpty && avatar.isEmpty) return null;
        return _NotificationSenderMeta(name: name, avatarUrl: avatar);
      }

      if (kind == 'seller') {
        final card = await ref.watch(storeCardProvider(id).future);
        if (card == null) return null;
        final name = card.displayName;
        final avatar = card.logoUri;
        return _NotificationSenderMeta(name: name, avatarUrl: avatar);
      }

      return null;
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
    final inboxMessages = ref.watch(notificationInboxStreamProvider);

    // 每次進入頁面時靜默刷新通知列表（不顯示 loading 遮罩）
    useEffect(() {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        ref.read(notificationListControllerProvider.notifier).silentRefresh();
      });
      return null;
    }, const []);

    // 訂閱 FCM 訊息總線：收到任何推播 / 使用者點擊後台通知，
    // 都在 300ms 內觸發一次 silentRefresh，讓新通知即時出現在列表
    useEffect(() {
      Timer? debounce;
      final sub = inboxMessages.listen((_) {
        debounce?.cancel();
        debounce = Timer(const Duration(milliseconds: 300), () {
          ref.read(notificationListControllerProvider.notifier).silentRefresh();
        });
      });
      return () {
        debounce?.cancel();
        sub.cancel();
      };
    }, [inboxMessages]);

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
      appBar: buildCommonTitleAppBar(
        context,
        title: '通知訊息',
        actions: [
          if (unreadCount > 0 && !isDeleteMode.value)
            HkTapTarget(
              key: const ValueKey('notifications-mark-all-read-action'),
              semanticLabel: '全部標示為已讀',
              hint: '將所有通知設為已讀',
              tooltip: '全部標示為已讀',
              onTap: () async {
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
              child: Icon(
                AppIcons.checkCircleOutline,
                color: AppIconColors.onBrand,
                size: 24.w,
              ),
            ),
          HkTapTarget(
            key: const ValueKey('notifications-delete-mode-action'),
            semanticLabel: isDeleteMode.value ? '結束刪除模式' : '進入刪除模式',
            hint: isDeleteMode.value ? '返回一般通知列表' : '選取要刪除的通知',
            tooltip: isDeleteMode.value ? '結束刪除模式' : '進入刪除模式',
            toggled: isDeleteMode.value,
            onTap: () {
              isDeleteMode.value = !isDeleteMode.value;
              selectedIds.value = {};
            },
            child: Icon(
              isDeleteMode.value ? AppIcons.close : AppIcons.deleteOutline,
              color: AppIconColors.onBrand,
              size: 24.w,
            ),
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
                            ((state.isLoadingMore ||
                                    state.paginationError != null)
                                ? 1
                                : 0),
                        itemBuilder: (context, index) {
                          if (index == state.notifications.length) {
                            if (state.isLoadingMore) {
                              return Padding(
                                padding: EdgeInsets.all(16.w),
                                child: const Center(
                                  child: HkBrandLoader(
                                    size: HkBrandLoader.inline,
                                  ),
                                ),
                              );
                            }
                            return _PaginationErrorRetry(
                              error: state.paginationError!,
                              onRetry: () => ref
                                  .read(
                                    notificationListControllerProvider.notifier,
                                  )
                                  .retryLoadMore(),
                            );
                          }
                          final n = state.notifications[index];
                          return _NotificationCard(
                            notification: n,
                            isDeleteMode: isDeleteMode.value,
                            isSelected: selectedIds.value.contains(
                              n.id.toString(),
                            ),
                            onSelectionChanged: (selected) {
                              final s = Set<String>.from(selectedIds.value);
                              selected
                                  ? s.add(n.id.toString())
                                  : s.remove(n.id.toString());
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
          context.push('/orders/detail', extra: {'orderSn': refId});
        } else {
          context.go('/?tab=3');
        }
      case 'CHAT':
        if (refId != null) {
          context.push(
            '/chat/room',
            extra: {'conversationId': refId, 'merchantId': ''},
          );
        } else {
          context.push('/chat');
        }
      case 'AFTERSALE':
      case 'AFTER_SALE':
      case 'REFUND':
        if (refId != null && refId.isNotEmpty) {
          context.push('/settings/after-sale/detail', extra: {'caseNo': refId});
        } else {
          context.go('/?tab=3');
        }
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
            style: TextStyle(fontSize: 14, color: AppColors.textSecondary),
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
          key: ValueKey('notifications-empty-state'),
          iconKey: ValueKey('notifications-empty-state-icon'),
          icon: HkIconName.emptyNotifications,
          message: '暫無通知',
        ),
      ],
    );
  }
}

class _PaginationErrorRetry extends StatelessWidget {
  const _PaginationErrorRetry({required this.error, required this.onRetry});

  final Object error;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    final message = error is AppException
        ? (error as AppException).message
        : '載入更多失敗，請稍後再試';
    return Padding(
      key: const ValueKey('notifications-pagination-error'),
      padding: EdgeInsets.fromLTRB(24.w, 12.h, 24.w, 20.h),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(
            message,
            textAlign: TextAlign.center,
            style: TextStyle(fontSize: 12, color: AppColors.textSecondary),
          ),
          SizedBox(height: 8.h),
          OutlinedButton(
            key: const ValueKey('notifications-pagination-retry'),
            onPressed: onRetry,
            style: OutlinedButton.styleFrom(
              minimumSize: const Size(120, HkControl.minTap),
            ),
            child: const HkButtonLabel('重試載入更多'),
          ),
        ],
      ),
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
    final directSenderName = _senderNameFromData(n);
    final directSenderAvatar = _senderAvatarFromData(n);
    final lookupKey = _senderLookupKey(n);
    final shouldFetchSender =
        (directSenderName.isEmpty || directSenderAvatar.isEmpty) &&
        lookupKey.isNotEmpty;
    final fetchedSender = shouldFetchSender
        ? ref.watch(_notificationSenderMetaProvider(lookupKey)).value
        : null;
    final senderName = directSenderName.isNotEmpty
        ? directSenderName
        : (fetchedSender?.name ?? '');
    final senderAvatar = directSenderAvatar.isNotEmpty
        ? directSenderAvatar
        : (fetchedSender?.avatarUrl ?? '');
    final title = _personalize(n.title, senderName);
    final content = _personalize(n.content, senderName);
    final timeLabel = _formatTime(n.createdAt);
    final handleTap = widget.isDeleteMode
        ? () => widget.onSelectionChanged?.call(!widget.isSelected)
        : widget.onTap;

    return Listener(
      onPointerDown: handleTap == null ? null : (_) => _pressCtrl.forward(),
      onPointerUp: handleTap == null ? null : (_) => _pressCtrl.reverse(),
      onPointerCancel: handleTap == null ? null : (_) => _pressCtrl.reverse(),
      child: HkTapTarget(
        key: ValueKey('notification-row-${n.id}'),
        semanticLabel: widget.isDeleteMode
            ? '選取通知：$title，$content'
            : '通知：$title，$content，$timeLabel',
        hint: widget.isDeleteMode
            ? (widget.isSelected ? '取消選取這則通知' : '選取這則通知')
            : '開啟通知內容',
        checked: widget.isDeleteMode ? widget.isSelected : null,
        button: !widget.isDeleteMode,
        onTap: handleTap,
        child: SizedBox(
          width: double.infinity,
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
                    // The whole row is the checkbox target in delete mode.
                    // Keep this visual compact and exclude duplicate semantics.
                    if (widget.isDeleteMode)
                      Padding(
                        padding: EdgeInsets.only(right: 12.w, top: 4.h),
                        child: ExcludeSemantics(
                          child: AnimatedContainer(
                            key: ValueKey(
                              'notification-selection-indicator-${n.id}',
                            ),
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
                      ),

                    // 圓形圖標（未讀時右上角顯示紅點）
                    Stack(
                      clipBehavior: Clip.none,
                      children: [
                        NotificationTypeIcon(kind: config.kind, size: 34.w),
                        if (!n.isRead)
                          Positioned(
                            top: -2,
                            right: -2,
                            child: Container(
                              width: 9.w,
                              height: 9.w,
                              decoration: const BoxDecoration(
                                color: AppColors.danger,
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
                                    fontSize: 16,
                                    fontWeight: FontWeight.w700,
                                    color: AppColors.textEmphasis,
                                  ),
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                ),
                              ),
                              SizedBox(width: 8.w),
                              Text(
                                timeLabel,
                                style: TextStyle(
                                  fontSize: 12,
                                  color: AppColors.textSecondary,
                                ),
                              ),
                            ],
                          ),
                          SizedBox(height: 4.h),
                          // Body
                          Text(
                            content,
                            style: TextStyle(
                              fontSize: 14,
                              color: AppColors.textSecondary,
                              height: 1.4,
                            ),
                            maxLines: 2,
                            overflow: TextOverflow.ellipsis,
                          ),
                        ],
                      ),
                    ),
                    if (senderAvatar.isNotEmpty) ...[
                      SizedBox(width: 8.w),
                      _NotificationSenderAvatar(url: senderAvatar),
                    ],
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }

  static ({NotificationIconKind kind, Color color}) _iconConfig(
    NotificationItem n,
  ) {
    // 依序嘗試：1) type 精確匹配 → 2) refType → 3) title 關鍵字 → 4) system
    const orderStyle = (
      kind: NotificationIconKind.order,
      color: AppColors.warning,
    );
    const refundStyle = (
      kind: NotificationIconKind.refund,
      color: AppColors.danger,
    );
    const chatStyle = (
      kind: NotificationIconKind.chat,
      color: AppColors.textPrimary,
    );
    const postStyle = (
      kind: NotificationIconKind.post,
      color: AppColors.primary,
    );
    const messageStyle = (
      kind: NotificationIconKind.message,
      color: AppColors.primary,
    );
    const userStyle = (
      kind: NotificationIconKind.user,
      color: AppColors.primary,
    );
    const couponStyle = (
      kind: NotificationIconKind.coupon,
      color: AppColors.danger,
    );
    const systemStyle = (
      kind: NotificationIconKind.system,
      color: AppColors.textPrimary,
    );

    // 1. type 精確匹配（買家端收到的通知類型）
    switch (n.type) {
      case 'CHAT_MESSAGE':
      case 'CHAT_GROUP_INVITE':
        return chatStyle;
      case 'ORDER_STATUS':
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
      case 'PROMOTION':
      case 'COUPON_RECEIVED':
      case 'COUPON_EXPIRING':
      case 'POINT_EARNED':
      case 'POINT_EXPIRING':
        return couponStyle;
      case 'PRODUCT_RESTOCK':
      case 'PRODUCT_PRICE_DROP':
      case 'FAVORITE_UPDATE':
      case 'SELLER_POST':
      case 'POST_CREATED':
      case 'POST_LIKE':
        return postStyle;
      case 'SELLER_COMMENT':
      case 'POST_COMMENT':
      case 'COMMENT_REPLY':
      case 'MESSAGE_REPLY':
        return messageStyle;
      case 'SELLER_FOLLOW':
      case 'MERCHANT_UPDATE':
        return userStyle;
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
      case 'PRODUCT':
        return postStyle;
      case 'SELLER':
      case 'MERCHANT':
        return userStyle;
      case 'COUPON':
      case 'POINT':
        return couponStyle;
    }

    // 3. title 關鍵字 fallback（買家用語）
    final title = n.title;
    if (title.contains('退貨') || title.contains('退款') || title.contains('售後')) {
      return refundStyle;
    }
    if (title.contains('訂單') ||
        title.contains('出貨') ||
        title.contains('到貨') ||
        title.contains('配送') ||
        title.contains('已送達') ||
        title.contains('取貨')) {
      return orderStyle;
    }
    if (title.contains('優惠') ||
        title.contains('折價') ||
        title.contains('券') ||
        title.contains('點數') ||
        title.contains('紅利')) {
      return couponStyle;
    }
    if (title.contains('聊天') ||
        title.contains('訊息') ||
        title.contains('店家回覆')) {
      return chatStyle;
    }
    if (title.contains('留言') || title.contains('回覆')) {
      return messageStyle;
    }
    if (title.contains('貼文') ||
        title.contains('動態') ||
        title.contains('商品') ||
        title.contains('補貨') ||
        title.contains('降價') ||
        title.contains('收藏')) {
      return postStyle;
    }
    if (title.contains('追蹤') || title.contains('關注') || title.contains('賣家')) {
      return userStyle;
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

  static String _senderNameFromData(NotificationItem n) {
    return _firstDataString(n.data, const [
      'senderName',
      'sender_name',
      'sellerName',
      'seller_name',
      'merchantName',
      'merchant_name',
      'storeName',
      'store_name',
      'fromName',
      'from_name',
      'displayName',
    ]);
  }

  static String _senderAvatarFromData(NotificationItem n) {
    return _firstDataString(n.data, const [
      'senderAvatarUrl',
      'sender_avatar_url',
      'senderAvatar',
      'sender_avatar',
      'sellerAvatarUrl',
      'seller_avatar_url',
      'sellerAvatar',
      'seller_avatar',
      'userAvatarUrl',
      'logoKey',
      'logoUrl',
      'merchantAvatar',
      'merchant_avatar',
      'avatarUrl',
      'avatar_url',
      'iconUrl',
      'icon_url',
      'imageUrl',
      'image_url',
      'logo',
      'avatar',
    ]);
  }

  static String _senderLookupKey(NotificationItem n) {
    final orderNo = _orderNoFromNotification(n);
    if (orderNo.isNotEmpty && _isOrderNotification(n)) {
      return 'order:$orderNo';
    }
    final sellerId = _firstDataString(n.data, const [
      'storeId',
      'store_id',
      'sellerId',
      'seller_id',
      'merchantId',
      'merchant_id',
    ]);
    if (sellerId.isNotEmpty) return 'seller:$sellerId';
    return '';
  }

  static String _orderNoFromNotification(NotificationItem n) {
    final refType = (n.refType ?? '').toUpperCase();
    if (refType == 'ORDER' && (n.refId ?? '').isNotEmpty) {
      return n.refId!.trim();
    }
    return _firstDataString(n.data, const [
      'orderNo',
      'order_no',
      'orderSn',
      'order_sn',
      'orderId',
      'order_id',
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
      final value = _firstFromMap(source, keys);
      if (value.isNotEmpty) return value;
    }
    return '';
  }

  static String _personalize(String text, String senderName) {
    final name = senderName.trim();
    if (text.isEmpty || name.isEmpty) return text;
    return text
        .replaceAll('賣家', name)
        .replaceAll('店家', name)
        .replaceAll('商家', name)
        .replaceAll('我們', name);
  }
}

String _firstFromMap(Map<String, dynamic> data, List<String> keys) {
  for (final key in keys) {
    final v = data[key];
    if (v == null) continue;
    final s = v.toString().trim();
    if (s.isNotEmpty) return s;
  }
  return '';
}

class _NotificationSenderMeta {
  const _NotificationSenderMeta({required this.name, required this.avatarUrl});

  final String name;
  final String avatarUrl;
}

class _NotificationSenderAvatar extends StatelessWidget {
  const _NotificationSenderAvatar({required this.url});

  final String url;

  @override
  Widget build(BuildContext context) {
    return ClipOval(
      child: AppCachedImage(
        imageUrl: url,
        scene: ImageScene.avatarSmall,
        width: 32.w,
        height: 32.w,
        fit: BoxFit.cover,
        errorWidget: Container(
          width: 32.w,
          height: 32.w,
          decoration: const BoxDecoration(
            color: AppColors.gray100,
            shape: BoxShape.circle,
          ),
          child: Icon(
            AppIcons.storefrontOutlined,
            size: 18.sp,
            color: AppIconColors.secondary,
          ),
        ),
      ),
    );
  }
}

// ══════════════════════════════════════════════════════════════
// 全部已讀確認彈窗
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
              decoration: const BoxDecoration(
                color: AppColors.gray100,
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
                fontSize: 16,
                fontWeight: FontWeight.w700,
                color: AppColors.textEmphasis,
              ),
            ),
            SizedBox(height: 6.h),
            Text(
              '所有未讀通知已標記完畢',
              style: TextStyle(fontSize: 14, color: AppColors.textSecondary),
            ),
            SizedBox(height: 22.h),
            SizedBox(
              width: double.infinity,
              height: HkControl.minTap,
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
                    fontSize: 14,
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
        height: HkControl.minTap,
        child: ElevatedButton(
          onPressed: count > 0 ? onDelete : null,
          style: ElevatedButton.styleFrom(
            backgroundColor: AppColors.danger,
            disabledBackgroundColor: AppColors.dividerLight,
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(12.r),
            ),
          ),
          child: HkButtonLabel(
            count > 0 ? '刪除 ($count)' : '請選擇要刪除的通知',
            style: TextStyle(
              color: AppColors.bgCard,
              fontSize: 16,
              fontWeight: FontWeight.w700,
            ),
          ),
        ),
      ),
    );
  }
}
