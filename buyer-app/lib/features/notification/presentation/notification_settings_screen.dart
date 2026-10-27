import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/extensions/async_value_ui.dart';
import '../../../shared/widgets/common_top_bar.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../../shared/widgets/floating_toast.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../data/notification_model.dart';
import 'notification_controller.dart';
import 'widgets/notification_type_icon.dart';

/// 通知設定頁面 — 7 個通知類別 toggle（對應 Figma 設計）
class NotificationSettingsScreen extends HookConsumerWidget {
  const NotificationSettingsScreen({super.key});

  // Category → 通知類別圖示
  static const _categoryIcon = <String, NotificationIconKind>{
    'order_update': NotificationIconKind.order,
    'refund_update': NotificationIconKind.refund,
    'system': NotificationIconKind.system,
    'chat': NotificationIconKind.chat,
    'coupon': NotificationIconKind.coupon,
    'seller_post': NotificationIconKind.post,
    'seller_comment': NotificationIconKind.message,
    'restock': NotificationIconKind.system,
    // shipment_anomaly 不對 buyer 開放：template target_user_type='seller'，
    // buyer 永遠收不到此推送，這裡不放 mapping。
  };

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final asyncPrefs = ref.watch(notificationPreferencesControllerProvider);

    ref.listen(notificationPreferencesControllerProvider, (_, next) {
      next.showErrorSnackbar(context);
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
    return buildCommonTitleAppBar(context, title: '通知設定');
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
            style: TextStyle(fontSize: 14, color: AppColors.textSecondary),
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
          return Padding(
            padding: EdgeInsets.only(bottom: 12.h),
            child: _buildPreferenceCard(context: context, ref: ref, pref: pref),
          );
        }).toList(),
      ),
    );
  }

  Widget _buildPreferenceCard({
    required BuildContext context,
    required WidgetRef ref,
    required NotificationPreferenceItem pref,
  }) {
    final iconKind =
        _categoryIcon[pref.category] ?? NotificationIconKind.system;

    return NotificationPreferenceCard(
      preference: pref,
      iconKind: iconKind,
      onChanged: (enabled) => _onToggle(context, ref, pref.category, enabled),
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
      FloatingToast.showError('儲存失敗，請稍後再試');
    }
  }
}

/// A preference row whose content is deliberately testable independently of
/// the network-backed notification controller.
class NotificationPreferenceCard extends StatelessWidget {
  const NotificationPreferenceCard({
    super.key,
    required this.preference,
    required this.iconKind,
    required this.onChanged,
  });

  final NotificationPreferenceItem preference;
  final NotificationIconKind iconKind;
  final ValueChanged<bool> onChanged;

  @override
  Widget build(BuildContext context) {
    return Container(
      key: ValueKey('notification-preference-${preference.category}'),
      constraints: const BoxConstraints(minHeight: 68),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(16),
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
        padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 10),
        child: Row(
          children: [
            NotificationTypeIcon(kind: iconKind),
            const SizedBox(width: 12),
            // Title + description
            Expanded(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                mainAxisAlignment: MainAxisAlignment.center,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    preference.title,
                    style: TextStyle(
                      fontSize: 16,
                      fontWeight: FontWeight.w400,
                      color: AppColors.textEmphasis,
                    ),
                  ),
                  const SizedBox(height: 4),
                  Text(
                    preference.description,
                    style: TextStyle(
                      fontSize: 14,
                      fontWeight: FontWeight.w400,
                      color: AppColors.textPrimary,
                    ),
                  ),
                ],
              ),
            ),
            // Toggle switch（缩小尺寸，优化颜色）
            Transform.scale(
              scale: 0.85,
              child: Switch(
                value: preference.enabled,
                onChanged: onChanged,
                activeThumbColor: AppColors.bgCard,
                activeTrackColor: AppColors.primary,
                inactiveThumbColor: AppColors.bgCard,
                inactiveTrackColor: AppColors.gray300,
                trackOutlineColor: WidgetStateProperty.all(Colors.transparent),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
