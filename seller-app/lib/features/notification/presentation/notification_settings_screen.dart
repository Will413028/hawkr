import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:hawkr_seller/shared/widgets/hk_glass_app_bar.dart';
import 'package:flutter/material.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../core/extensions/async_value_ui.dart';
import '../../../shared/utils/floating_toast.dart';
import '../../../shared/widgets/app_toggle_switch.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../data/notification_model.dart';
import 'notification_controller.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

/// 通知設定頁面 — 6 個通知類別 toggle（對應 Figma 設計）
class NotificationSettingsScreen extends HookConsumerWidget {
  const NotificationSettingsScreen({super.key});

  // Category → icon + color mapping
  static const _categoryConfig = {
    'order_update': _CategoryStyle(
      icon: AppIcons.shoppingBagOutlined,
      color: AppColors.warning,
    ),
    'refund_update': _CategoryStyle(
      icon: AppIcons.assignmentReturnOutlined,
      color: AppColors.error,
    ),
    'system': _CategoryStyle(
      icon: AppIcons.notificationsOutlined,
      color: AppColors.textPrimary,
    ),
    'chat': _CategoryStyle(
      icon: AppIcons.chatBubbleOutline,
      color: AppColors.textPrimary,
    ),
    'follower': _CategoryStyle(
      icon: AppIcons.personOutline,
      color: AppColors.primary,
    ),
    'post': _CategoryStyle(
      icon: AppIcons.descriptionOutlined,
      color: AppColors.primary,
    ),
    // 配送異常 — 賣家強制 on 不可關（backend §3.1）。
    // 沿用退款紅色語意。
    'shipment_anomaly': _CategoryStyle(
      icon: AppIcons.assignmentReturnOutlined,
      color: AppColors.error,
    ),
  };

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final asyncPrefs = ref.watch(notificationPreferencesControllerProvider);

    ref.listen(notificationPreferencesControllerProvider, (_, next) {
      next.showSnackBarOnError(context);
    });

    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      appBar: _buildAppBar(context),
      body: asyncPrefs.when(
        loading: () => const TruckLoadingOverlay(),
        error: (error, _) => _buildErrorState(context, ref, error),
        data: (prefs) => _buildContent(context, ref, prefs),
      ),
    );
  }

  PreferredSizeWidget _buildAppBar(BuildContext context) {
    return HkGlassAppBar(
      title: const Text('通知設定'),
      onBackPressed: () => context.pop(),
    );
  }

  Widget _buildErrorState(BuildContext context, WidgetRef ref, Object error) {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(AppIcons.errorOutline, size: 64.sp, color: AppIconColors.muted),
          SizedBox(height: 16.h),
          Text(
            error.toString(),
            style: TextStyle(fontSize: 14.sp, color: AppColors.textSecondary),
            textAlign: TextAlign.center,
          ),
          SizedBox(height: 16.h),
          ElevatedButton(
            onPressed: () => ref
                .read(notificationPreferencesControllerProvider.notifier)
                .reload(),
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

  Widget _buildContent(
    BuildContext context,
    WidgetRef ref,
    List<NotificationPreferenceItem> prefs,
  ) {
    return SingleChildScrollView(
      padding: EdgeInsets.fromLTRB(24.w, 20.h, 24.w, 40.h),
      child: Column(
        children: prefs.map((pref) {
          final style = _categoryConfig[pref.category];
          return Padding(
            padding: EdgeInsets.only(bottom: 12.h),
            child: _buildPreferenceCard(
              context: context,
              ref: ref,
              pref: pref,
              style: style,
            ),
          );
        }).toList(),
      ),
    );
  }

  Widget _buildPreferenceCard({
    required BuildContext context,
    required WidgetRef ref,
    required NotificationPreferenceItem pref,
    _CategoryStyle? style,
  }) {
    // 強制 on 不可關（賣家 shipment_anomaly）。
    // 顯示文案改為說明文字；toggle 仍渲染但不接受互動（IgnorePointer + 半透明）。
    final isForced = pref.forced;
    final description = isForced ? '強制接收，無法關閉' : pref.description;
    return Container(
      height: 68.h,
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(16.r),
        border: Border.all(color: AppColors.cardBorder),
        boxShadow: [
          BoxShadow(
            color: AppColors.cardShadow,
            blurRadius: 2,
            offset: const Offset(0, 1),
          ),
        ],
      ),
      child: Padding(
        padding: EdgeInsets.symmetric(horizontal: 20.w),
        child: Row(
          children: [
            // Category icon
            Container(
              width: 32.w,
              height: 32.w,
              decoration: BoxDecoration(
                color: style?.color ?? AppColors.textPrimary,
                shape: BoxShape.circle,
              ),
              alignment: Alignment.center,
              child: Icon(
                style?.icon ?? AppIcons.notificationsOutlined,
                size: 18.w,
                color: AppIconColors.onBrand,
              ),
            ),
            SizedBox(width: 12.w),
            // Title + description
            Expanded(
              child: Column(
                mainAxisAlignment: MainAxisAlignment.center,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    pref.title,
                    style: TextStyle(
                      fontSize: 16.sp,
                      fontWeight: FontWeight.w400,
                      color: AppColors.textEmphasis,
                    ),
                  ),
                  SizedBox(height: 4.h),
                  Text(
                    description,
                    style: TextStyle(
                      fontSize: 14.sp,
                      fontWeight: FontWeight.w400,
                      color: AppColors.textPrimary,
                    ),
                  ),
                ],
              ),
            ),
            // 品牌滑桿：高度對齊 title 字體大小 (16sp)
            // forced 時鎖死：IgnorePointer + opacity 半透明（不改 layout）。
            IgnorePointer(
              ignoring: isForced,
              child: Opacity(
                opacity: isForced ? 0.5 : 1.0,
                child: AppToggleSwitch(
                  value: pref.enabled,
                  height: 16,
                  onChanged: (v) => _onToggle(context, ref, pref.category, v),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _onToggle(
    BuildContext context,
    WidgetRef ref,
    String category,
    bool enabled,
  ) async {
    final success = await ref
        .read(notificationPreferencesControllerProvider.notifier)
        .updatePreference(category, enabled);

    if (!success && context.mounted) {
      FloatingToast.error(context, '儲存失敗，請稍後再試');
    }
  }
}

class _CategoryStyle {
  final IconData icon;
  final Color color;

  const _CategoryStyle({required this.icon, required this.color});
}
