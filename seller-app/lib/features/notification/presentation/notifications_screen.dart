import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:hawkr_seller/shared/widgets/hk_glass_app_bar.dart';
import 'package:hawkr_seller/shared/widgets/hk_tap_target.dart';
import 'package:flutter/material.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../config/image_config.dart';
import 'package:hawkr_api/errors.dart' show errorText;
import 'package:hawkr_api/images.dart';

import '../../../core/l10n/ui_locale.dart';
import '../../../shared/widgets/hk_empty_state.dart';
import '../../../shared/widgets/fruit_refresh_indicator.dart';
import '../../../shared/widgets/optimized_image.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../data/deep_link.dart';
import '../data/notification_repository.dart';
import 'notification_controller.dart';
import '../../../shared/time/taipei_display.dart';
import '../../../l10n/l10n.dart';

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
                            isSelected: selectedIds.value.contains(n.id),
                            onSelectionChanged: (selected) {
                              final s = Set<String>.from(selectedIds.value);
                              selected ? s.add(n.id) : s.remove(n.id);
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
    InboxItem notification,
  ) {
    if (!notification.read) {
      ref
          .read(notificationListControllerProvider.notifier)
          .markAsRead(notification.id);
    }
    // 目的地由伺服器給；空字串或對不到路由就不導航（跳到猜的頁面比不跳更糟）。
    NotificationDeepLink.parse(notification.deepLink)?.open();
  }

  Widget _buildErrorState(BuildContext context, WidgetRef ref, Object error) {
    final message = errorText(
      error,
      defaults: ref.read(defaultErrorTextProvider),
    );
    const icon = AppIcons.errorOutline;
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
  final InboxItem notification;
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
    final config = _iconConfig(n.presentation);
    final buyerAvatar = n.hasImage() && n.image.uriTemplate.isNotEmpty
        ? n.image.uri(ImagePreset.avatar)
        : '';
    final title = n.title;
    final body = n.body;
    final time = _formatTime(context, n.createTime.toDateTime());
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
                      if (!n.read)
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

  /// 圖示與顏色只依伺服器給的呈現分類；UNSPECIFIED 與不認得的值用 SYSTEM 的呈現。
  static ({IconData icon, Color color}) _iconConfig(
    InboxPresentation presentation,
  ) {
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
    return switch (presentation) {
      InboxPresentation.INBOX_PRESENTATION_ORDER ||
      InboxPresentation.INBOX_PRESENTATION_PAYOUT ||
      InboxPresentation.INBOX_PRESENTATION_PROCUREMENT => orderStyle,
      InboxPresentation.INBOX_PRESENTATION_REFUND => refundStyle,
      InboxPresentation.INBOX_PRESENTATION_MESSAGE => chatStyle,
      InboxPresentation.INBOX_PRESENTATION_SOCIAL ||
      InboxPresentation.INBOX_PRESENTATION_REVIEW => postStyle,
      InboxPresentation.INBOX_PRESENTATION_PROMOTION => couponStyle,
      InboxPresentation.INBOX_PRESENTATION_ACCOUNT => userStyle,
      _ => systemStyle,
    };
  }

  static String _formatTime(BuildContext context, DateTime dateTime) =>
      context.taipeiDisplay.timeAgoIn(context.l10n, dateTime);
}

class _NotificationBuyerAvatar extends StatelessWidget {
  const _NotificationBuyerAvatar({required this.url});

  final String url;

  @override
  Widget build(BuildContext context) {
    final imageUrl = url;
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
