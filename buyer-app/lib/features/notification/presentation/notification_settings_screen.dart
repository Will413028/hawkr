import 'package:hawkr_api/errors.dart' show errorText;
import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.pb.dart'
    as pb;
import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.pbenum.dart';
import 'package:hawkr_buyer/core/l10n/ui_locale.dart';
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
import 'preference_labels.dart';
import 'notification_controller.dart';
import 'widgets/notification_type_icon.dart';

/// 通知設定頁面：每個分類一列，伺服器列出的每個渠道一個開關；不可關的分類鎖住。
class NotificationSettingsScreen extends HookConsumerWidget {
  const NotificationSettingsScreen({super.key});

  // 分類 → 圖示（只決定圖示，不含文字）。
  static const _categoryIcon = <NotificationCategory, NotificationIconKind>{
    NotificationCategory.NOTIFICATION_CATEGORY_ORDER:
        NotificationIconKind.order,
    NotificationCategory.NOTIFICATION_CATEGORY_SHIPMENT:
        NotificationIconKind.order,
    NotificationCategory.NOTIFICATION_CATEGORY_AFTERSALES:
        NotificationIconKind.refund,
    NotificationCategory.NOTIFICATION_CATEGORY_CHAT: NotificationIconKind.chat,
    NotificationCategory.NOTIFICATION_CATEGORY_COUPON:
        NotificationIconKind.coupon,
    NotificationCategory.NOTIFICATION_CATEGORY_FOLLOWED_STORE_POST:
        NotificationIconKind.post,
    NotificationCategory.NOTIFICATION_CATEGORY_COMMENT:
        NotificationIconKind.message,
    NotificationCategory.NOTIFICATION_CATEGORY_REVIEW:
        NotificationIconKind.message,
    NotificationCategory.NOTIFICATION_CATEGORY_LIKE: NotificationIconKind.post,
    NotificationCategory.NOTIFICATION_CATEGORY_FOLLOW:
        NotificationIconKind.user,
    NotificationCategory.NOTIFICATION_CATEGORY_ACCOUNT:
        NotificationIconKind.user,
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
            errorText(error, defaults: ref.read(defaultErrorTextProvider)),
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
    List<pb.PreferenceEntry> prefs,
  ) {
    final byCategory = <NotificationCategory, List<pb.PreferenceEntry>>{};
    for (final entry in prefs) {
      byCategory.putIfAbsent(entry.category, () => []).add(entry);
    }
    return SingleChildScrollView(
      padding: EdgeInsets.fromLTRB(24.w, 20.h, 24.w, 40.h),
      child: Column(
        children: [
          for (final group in byCategory.entries)
            Padding(
              padding: EdgeInsets.only(bottom: 12.h),
              child: NotificationPreferenceCard(
                category: group.key,
                entries: group.value,
                iconKind:
                    _categoryIcon[group.key] ?? NotificationIconKind.system,
                onChanged: (channel, enabled) =>
                    _onToggle(context, ref, group.key, channel, enabled),
              ),
            ),
        ],
      ),
    );
  }

  Future<void> _onToggle(
    BuildContext context,
    WidgetRef ref,
    NotificationCategory category,
    NotificationChannel channel,
    bool enabled,
  ) async {
    final success = await ref
        .read(notificationPreferencesControllerProvider.notifier)
        .updatePreference(category, channel, enabled);

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
    required this.category,
    required this.entries,
    required this.iconKind,
    required this.onChanged,
  });

  final NotificationCategory category;
  final List<pb.PreferenceEntry> entries;
  final NotificationIconKind iconKind;
  final void Function(NotificationChannel channel, bool enabled) onChanged;

  @override
  Widget build(BuildContext context) {
    return Container(
      key: ValueKey('notification-preference-${category.name}'),
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
            Expanded(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    notificationCategoryLabel(category),
                    style: TextStyle(
                      fontSize: 16,
                      fontWeight: FontWeight.w400,
                      color: AppColors.textEmphasis,
                    ),
                  ),
                  for (final entry in entries)
                    Row(
                      key: ValueKey(
                        'notification-preference-${category.name}-'
                        '${entry.channel.name}',
                      ),
                      children: [
                        Expanded(
                          child: Text(
                            notificationChannelLabel(entry.channel),
                            style: TextStyle(
                              fontSize: 14,
                              fontWeight: FontWeight.w400,
                              color: AppColors.textPrimary,
                            ),
                          ),
                        ),
                        if (!entry.canDisable)
                          const Icon(AppIcons.lockOutline, size: 16),
                        Transform.scale(
                          scale: 0.85,
                          child: Switch(
                            value: entry.enabled,
                            onChanged: entry.canDisable
                                ? (v) => onChanged(entry.channel, v)
                                : null,
                            activeThumbColor: AppColors.bgCard,
                            activeTrackColor: AppColors.primary,
                            inactiveThumbColor: AppColors.bgCard,
                            inactiveTrackColor: AppColors.gray300,
                            trackOutlineColor: WidgetStateProperty.all(
                              Colors.transparent,
                            ),
                          ),
                        ),
                      ],
                    ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
