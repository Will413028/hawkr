import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:hawkr_seller/shared/widgets/hk_glass_app_bar.dart';
import 'package:flutter/material.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:hawkr_api/errors.dart' show errorText;

import '../../../core/l10n/ui_locale.dart';
import '../../../shared/utils/floating_toast.dart';
import '../../../shared/widgets/app_toggle_switch.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../data/notification_preference_labels.dart';
import '../data/notification_repository.dart';
import 'notification_controller.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

/// 通知設定頁面：每個分類一列，伺服器回哪些渠道就有哪些開關；不可關的開關鎖住。
class NotificationSettingsScreen extends HookConsumerWidget {
  const NotificationSettingsScreen({super.key});

  static const _defaultStyle = _CategoryStyle(
    icon: AppIcons.notificationsOutlined,
    color: AppColors.textPrimary,
  );

  static const _categoryConfig = <NotificationCategory, _CategoryStyle>{
    NotificationCategory.NOTIFICATION_CATEGORY_ORDER: _CategoryStyle(
      icon: AppIcons.shoppingBagOutlined,
      color: AppColors.warning,
    ),
    NotificationCategory.NOTIFICATION_CATEGORY_SHIPMENT: _CategoryStyle(
      icon: AppIcons.shoppingBagOutlined,
      color: AppColors.warning,
    ),
    NotificationCategory.NOTIFICATION_CATEGORY_SHIPMENT_ANOMALY: _CategoryStyle(
      icon: AppIcons.assignmentReturnOutlined,
      color: AppColors.error,
    ),
    NotificationCategory.NOTIFICATION_CATEGORY_AFTERSALES: _CategoryStyle(
      icon: AppIcons.assignmentReturnOutlined,
      color: AppColors.error,
    ),
    NotificationCategory.NOTIFICATION_CATEGORY_CHAT: _CategoryStyle(
      icon: AppIcons.chatBubbleOutline,
      color: AppColors.textPrimary,
    ),
    NotificationCategory.NOTIFICATION_CATEGORY_FOLLOW: _CategoryStyle(
      icon: AppIcons.personOutline,
      color: AppColors.primary,
    ),
    NotificationCategory.NOTIFICATION_CATEGORY_COMMENT: _CategoryStyle(
      icon: AppIcons.descriptionOutlined,
      color: AppColors.primary,
    ),
    NotificationCategory.NOTIFICATION_CATEGORY_LIKE: _CategoryStyle(
      icon: AppIcons.descriptionOutlined,
      color: AppColors.primary,
    ),
  };

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final asyncPrefs = ref.watch(notificationPreferencesControllerProvider);

    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      appBar: HkGlassAppBar(
        title: const Text('通知設定'),
        onBackPressed: () => context.pop(),
      ),
      body: asyncPrefs.when(
        loading: () => const TruckLoadingOverlay(),
        error: (error, _) => _buildErrorState(context, ref, error),
        data: (prefs) => _buildContent(context, ref, prefs),
      ),
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
            errorText(error, defaults: ref.read(defaultErrorTextProvider)),
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
    List<PreferenceEntry> prefs,
  ) {
    final byCategory = <NotificationCategory, List<PreferenceEntry>>{};
    for (final e in prefs) {
      byCategory.putIfAbsent(e.category, () => []).add(e);
    }
    return SingleChildScrollView(
      padding: EdgeInsets.fromLTRB(24.w, 20.h, 24.w, 40.h),
      child: Column(
        children: [
          for (final entry in byCategory.entries)
            Padding(
              padding: EdgeInsets.only(bottom: 12.h),
              child: _buildCategoryCard(
                context: context,
                ref: ref,
                category: entry.key,
                entries: entry.value,
              ),
            ),
        ],
      ),
    );
  }

  Widget _buildCategoryCard({
    required BuildContext context,
    required WidgetRef ref,
    required NotificationCategory category,
    required List<PreferenceEntry> entries,
  }) {
    final style = _categoryConfig[category] ?? _defaultStyle;
    return Container(
      constraints: BoxConstraints(minHeight: 68.h),
      padding: EdgeInsets.symmetric(horizontal: 20.w, vertical: 8.h),
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
      child: Row(
        children: [
          Container(
            width: 32.w,
            height: 32.w,
            decoration: BoxDecoration(
              color: style.color,
              shape: BoxShape.circle,
            ),
            alignment: Alignment.center,
            child: Icon(style.icon, size: 18.w, color: AppIconColors.onBrand),
          ),
          SizedBox(width: 12.w),
          Expanded(
            child: Text(
              notificationCategoryLabel(category),
              style: TextStyle(
                fontSize: 16.sp,
                fontWeight: FontWeight.w400,
                color: AppColors.textEmphasis,
              ),
            ),
          ),
          for (final e in entries) ...[
            SizedBox(width: 12.w),
            Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                // 不可關的 (分類, 渠道)：開關鎖住（IgnorePointer + 半透明，不改 layout）。
                IgnorePointer(
                  ignoring: !e.canDisable,
                  child: Opacity(
                    opacity: e.canDisable ? 1.0 : 0.5,
                    child: AppToggleSwitch(
                      key: ValueKey(
                        'pref-${category.value}-${e.channel.value}',
                      ),
                      value: e.enabled,
                      height: 16,
                      onChanged: (v) => _onToggle(context, ref, e, v),
                    ),
                  ),
                ),
                SizedBox(height: 2.h),
                Text(
                  notificationChannelLabel(e.channel),
                  style: TextStyle(
                    fontSize: 12.sp,
                    color: AppColors.textSecondary,
                  ),
                ),
              ],
            ),
          ],
        ],
      ),
    );
  }

  Future<void> _onToggle(
    BuildContext context,
    WidgetRef ref,
    PreferenceEntry entry,
    bool enabled,
  ) async {
    final success = await ref
        .read(notificationPreferencesControllerProvider.notifier)
        .setEnabled(entry.category, entry.channel, enabled);

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
