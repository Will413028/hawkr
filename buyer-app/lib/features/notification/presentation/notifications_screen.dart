import 'package:hawkr_api/errors.dart' show errorText;
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/image.pb.dart' as common;
import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.pb.dart'
    as pb;
import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.pbenum.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../config/image_config.dart';
import '../../../core/l10n/ui_locale.dart';
import 'widgets/notification_type_icon.dart';
import '../../../shared/widgets/app_cached_image.dart';
import '../../../shared/widgets/common_top_bar.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../../shared/widgets/hk_empty_state.dart';
import '../../../shared/widgets/hk_tap_target.dart';
import '../../../shared/widgets/fruit_refresh_indicator.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../data/push/manager.dart';
import '../data/push/router.dart';
import 'notification_controller.dart';
import 'package:hawkr_buyer/shared/widgets/hk_brand_loader.dart';
import '../../../shared/time/taipei_display.dart';
import '../../../l10n/l10n.dart';

/// Injectable FCM inbox stream so the notification surface can be exercised
/// without bootstrapping Firebase in widget tests.
final notificationInboxStreamProvider = Provider<Stream<Object?>>(
  (_) => PushNotificationManager.instance.onInboxMessage,
);

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

    useEffect(() {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        ref.read(notificationListControllerProvider.notifier).silentRefresh();
      });
      return null;
    }, const []);

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
    pb.InboxItem notification,
  ) {
    if (!notification.read) {
      ref
          .read(notificationListControllerProvider.notifier)
          .markAsRead(notification.id);
    }
    NotificationRouter.pushDeepLink(context, notification.deepLink);
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

class _PaginationErrorRetry extends ConsumerWidget {
  const _PaginationErrorRetry({required this.error, required this.onRetry});

  final Object error;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final message = errorText(
      error,
      defaults: ref.read(defaultErrorTextProvider),
    );
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

class _NotificationCard extends ConsumerStatefulWidget {
  final pb.InboxItem notification;
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
    final kind = _iconKind(n.presentation);
    final title = n.title;
    final content = n.body;
    final timeLabel = _formatTime(context, n.createTime.toDateTime());
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

                    Stack(
                      clipBehavior: Clip.none,
                      children: [
                        NotificationTypeIcon(kind: kind, size: 34.w),
                        if (!n.read)
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

                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
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
                    if (n.hasImage()) ...[
                      SizedBox(width: 8.w),
                      _NotificationSenderAvatar(image: n.image),
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

  /// 呈現分類只決定圖示；不認得的值與 UNSPECIFIED 用系統的呈現。
  static NotificationIconKind _iconKind(InboxPresentation presentation) {
    return switch (presentation) {
      InboxPresentation.INBOX_PRESENTATION_ORDER => NotificationIconKind.order,
      InboxPresentation.INBOX_PRESENTATION_REFUND =>
        NotificationIconKind.refund,
      InboxPresentation.INBOX_PRESENTATION_MESSAGE => NotificationIconKind.chat,
      InboxPresentation.INBOX_PRESENTATION_SOCIAL => NotificationIconKind.post,
      InboxPresentation.INBOX_PRESENTATION_REVIEW =>
        NotificationIconKind.message,
      InboxPresentation.INBOX_PRESENTATION_PROMOTION =>
        NotificationIconKind.coupon,
      InboxPresentation.INBOX_PRESENTATION_ACCOUNT => NotificationIconKind.user,
      _ => NotificationIconKind.system,
    };
  }

  static String _formatTime(BuildContext context, DateTime dateTime) =>
      context.taipeiDisplay.timeAgoIn(context.l10n, dateTime);
}

class _NotificationSenderAvatar extends StatelessWidget {
  const _NotificationSenderAvatar({required this.image});

  final common.Image image;

  @override
  Widget build(BuildContext context) {
    return ClipOval(
      child: AppCachedImage.api(
        image: image,
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
