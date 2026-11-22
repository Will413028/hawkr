import 'package:hawkr_api/errors.dart' show errorText;
import 'package:hawkr_api/gen/hawkr/crm/v1alpha1/crm.pbenum.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/core/theme/app_typography.dart';
import 'package:hawkr_seller/core/theme/hk_tokens.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../config/image_config.dart';
import '../../../core/l10n/ui_locale.dart';
import '../../../shared/widgets/app_dropdown.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../../shared/widgets/hk_content_card.dart';
import '../../../shared/widgets/hk_frosted_card_surface.dart';
import '../../../shared/widgets/hk_glass_app_bar.dart';
import '../../../shared/widgets/hk_empty_state.dart';
import '../../../shared/widgets/hk_icon.dart';
import '../../../shared/widgets/glass_tab_bar.dart';
import '../../../shared/widgets/optimized_image.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../data/customer_model.dart';
import 'customer_controller.dart';
import 'customer_load_more_footer.dart';

/// 排序選項（契約的 `CustomerSort`）；標籤取自共用文案目錄。
const customerSortOptions = <CustomerSort>[
  CustomerSort.CUSTOMER_SORT_ORDER_COUNT,
  CustomerSort.CUSTOMER_SORT_TOTAL_SPENT,
  CustomerSort.CUSTOMER_SORT_FIRST_ORDER_TIME,
];

/// 排序的標籤：UNSPECIFIED 依契約等同首購時間，先正規化再查目錄。
String customerSortLabel(CustomerSort sort, UiLocale l) {
  final s = sort == CustomerSort.CUSTOMER_SORT_UNSPECIFIED
      ? CustomerSort.CUSTOMER_SORT_FIRST_ORDER_TIME
      : sort;
  return s.uiText(l) ?? commonUnknown(l);
}

class CustomerManagementScreen extends ConsumerWidget {
  const CustomerManagementScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final filter = ref.watch(customerFilterProvider);
    final filterNotifier = ref.read(customerFilterProvider.notifier);
    final asyncCustomers = ref.watch(customerListProvider);
    final locale = context.uiLocale;
    final sortLabel = customerSortLabel(filter.sort, locale);

    return Scaffold(
      backgroundColor: AppColors.surfacePage,
      appBar: const HkGlassAppBar(title: Text('顧客管理')),
      body: DecoratedBox(
        decoration: BoxDecoration(
          gradient: LinearGradient(
            begin: Alignment.topLeft,
            end: Alignment.bottomRight,
            colors: [
              AppColors.primarySoft,
              AppColors.surfacePage,
              AppColors.brandSecondarySoft.withValues(alpha: 0.18),
            ],
            stops: const [0, 0.46, 1],
          ),
        ),
        child: Column(
          children: [
            _CustomerControls(
              sortLabel: sortLabel,
              onSortSelected: (label) {
                final option = customerSortOptions.firstWhere(
                  (o) => customerSortLabel(o, locale) == label,
                );
                filterNotifier.setSort(option);
              },
            ),
            _CustomerTabBar(
              selectedType: filter.type,
              onTabChanged: filterNotifier.setType,
            ),
            Expanded(
              child: asyncCustomers.when(
                data: (state) {
                  final customers = state.customers;
                  if (customers.isEmpty) {
                    return const HkEmptyState(
                      key: ValueKey('seller-customers-empty-state'),
                      iconKey: ValueKey('seller-customers-empty-state-icon'),
                      icon: HkIconName.users,
                      message: '暫無顧客',
                      description: '有人下單後會自動加入這份名單。',
                      topPadding: 48,
                    );
                  }
                  return Center(
                    child: ConstrainedBox(
                      constraints: const BoxConstraints(
                        maxWidth: HkControl.chromeMaxWidth,
                      ),
                      child: ListView.builder(
                        padding: const EdgeInsets.fromLTRB(
                          HkSpace.lg,
                          HkSpace.md,
                          HkSpace.lg,
                          HkSpace.large,
                        ),
                        itemCount: customers.length + (state.hasMore ? 1 : 0),
                        itemBuilder: (context, index) {
                          if (index == customers.length) {
                            return CustomerLoadMoreFooter(
                              errorKey: const ValueKey(
                                'seller-customers-load-more-error',
                              ),
                              onLoadMore: ref
                                  .read(customerListProvider.notifier)
                                  .loadMore,
                              errorMessage: state.loadMoreError == null
                                  ? null
                                  : errorText(
                                      state.loadMoreError!,
                                      defaults: ref.read(
                                        defaultErrorTextProvider,
                                      ),
                                    ),
                            );
                          }
                          final customer = customers[index];
                          return Padding(
                            padding: const EdgeInsets.only(bottom: HkSpace.md),
                            child: _CustomerCard(
                              customer: customer,
                              onTap: () async {
                                await context.push(
                                  '/customer/detail',
                                  extra: customer,
                                );
                                ref.invalidate(customerListProvider);
                              },
                            ),
                          );
                        },
                      ),
                    ),
                  );
                },
                loading: () => const TruckLoadingOverlay(),
                error: (err, stack) => _CustomerErrorState(
                  message: errorText(
                    err,
                    defaults: ref.read(defaultErrorTextProvider),
                  ),
                  onRetry: () => ref.invalidate(customerListProvider),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// 列表尾端：進到畫面時自動載入下一頁；失敗時顯示錯誤與重試，已載入的客戶不受影響。
class _CustomerControls extends StatelessWidget {
  const _CustomerControls({
    required this.sortLabel,
    required this.onSortSelected,
  });

  final String sortLabel;
  final ValueChanged<String> onSortSelected;

  @override
  Widget build(BuildContext context) {
    final sort = AppDropdown(
      items: [
        for (final o in customerSortOptions)
          customerSortLabel(o, context.uiLocale),
      ],
      selectedItem: sortLabel,
      onSelected: onSortSelected,
      showSearch: false,
      child: Container(
        height: HkControl.minTap,
        padding: const EdgeInsets.symmetric(horizontal: HkSpace.md),
        decoration: BoxDecoration(
          color: AppColors.neutralWhite.withValues(alpha: 0.74),
          borderRadius: BorderRadius.circular(HkRadius.pill),
          border: Border.all(color: AppColors.cardBorder),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            const HkIcon(HkIconName.filter, size: 20),
            const SizedBox(width: HkSpace.sm),
            Text(sortLabel, style: AppTypography.meta),
            const SizedBox(width: HkSpace.xs),
            const HkIcon(HkIconName.chevronDown, size: 18),
          ],
        ),
      ),
    );

    return Center(
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: HkControl.chromeMaxWidth),
        child: Padding(
          padding: const EdgeInsets.fromLTRB(
            HkSpace.lg,
            HkSpace.lg,
            HkSpace.lg,
            HkSpace.md,
          ),
          child: HkFrostedCardSurface(
            child: Padding(
              padding: const EdgeInsets.all(HkSpace.sm),
              child: Align(alignment: Alignment.centerRight, child: sort),
            ),
          ),
        ),
      ),
    );
  }
}

class _CustomerTabBar extends StatelessWidget {
  final CustomerType? selectedType;
  final ValueChanged<CustomerType?> onTabChanged;

  const _CustomerTabBar({
    required this.selectedType,
    required this.onTabChanged,
  });

  @override
  Widget build(BuildContext context) {
    final tabs = <({String label, CustomerType? type})>[
      (label: '全部顧客', type: null),
      (
        label:
            CustomerType.CUSTOMER_TYPE_VIP.uiText(context.uiLocale) ??
            commonUnknown(context.uiLocale),
        type: CustomerType.CUSTOMER_TYPE_VIP,
      ),
    ];
    final selectedIndex = tabs.indexWhere((tab) => tab.type == selectedType);
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: HkSpace.lg),
      child: HkGlassTabBar(
        labels: tabs.map((tab) => tab.label).toList(),
        selectedIndex: selectedIndex < 0 ? 0 : selectedIndex,
        onTap: (index) => onTabChanged(tabs[index].type),
      ),
    );
  }
}

class _CustomerCard extends StatelessWidget {
  final Customer customer;
  final VoidCallback? onTap;

  const _CustomerCard({required this.customer, this.onTap});

  String _formatCurrency(int amount) {
    if (amount == 0) return '\$0';
    final s = amount.toString();
    final buf = StringBuffer();
    for (int i = 0; i < s.length; i++) {
      final idx = s.length - 1 - i;
      buf.write(s[idx]);
      if (i % 3 == 2 && idx != 0) buf.write(',');
    }
    return '\$${buf.toString().split('').reversed.join()}';
  }

  @override
  Widget build(BuildContext context) {
    return HkContentCard(
      onTap: onTap,
      semanticLabel: '查看顧客 ${customer.displayName(context.uiLocale)}',
      padding: const EdgeInsets.symmetric(
        horizontal: HkSpace.lg,
        vertical: HkSpace.md,
      ),
      radius: HkRadius.lg,
      elevated: true,
      child: ConstrainedBox(
        constraints: const BoxConstraints(minHeight: 88),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Row(
              children: [
                // Avatar with VIP badge
                Stack(
                  clipBehavior: Clip.none,
                  children: [
                    Container(
                      width: 56.w,
                      height: 56.h,
                      decoration: BoxDecoration(
                        shape: BoxShape.circle,
                        border: Border.all(
                          color: AppColors.textSecondary,
                          width: 1,
                        ),
                        color: AppColors.gray100,
                      ),
                      child: ClipOval(
                        child:
                            customer.avatarUrl != null &&
                                customer.avatarUrl!.isNotEmpty &&
                                customer.avatarUrl!.startsWith('http')
                            ? OptimizedNetworkImage(
                                imageUrl: customer.avatarUrl!,
                                scene: ImageScene.avatarMedium,
                                fit: BoxFit.cover,
                                width: 56.w,
                                height: 56.h,
                                placeholder: const SizedBox.shrink(),
                                errorWidget: const HkIcon(
                                  HkIconName.user,
                                  size: 30,
                                  color: AppIconColors.muted,
                                ),
                              )
                            : const HkIcon(
                                HkIconName.user,
                                size: 30,
                                color: AppIconColors.muted,
                              ),
                      ),
                    ),
                    if (customer.isVip)
                      Positioned(
                        right: -2.w,
                        bottom: -2.h,
                        child: Container(
                          width: 20.w,
                          height: 20.h,
                          decoration: const BoxDecoration(
                            color: AppColors.accentFill,
                            shape: BoxShape.circle,
                          ),
                          child: const HkIcon(
                            HkIconName.award,
                            size: 13,
                            color: AppIconColors.onAccent,
                          ),
                        ),
                      ),
                  ],
                ),
                SizedBox(width: 16.w),

                // Name + ID
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      Text(
                        customer.displayName(context.uiLocale),
                        style: TextStyle(
                          fontSize: 16.sp,
                          fontWeight: FontWeight.w700,
                          color: AppColors.textPrimary,
                        ),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                      SizedBox(height: 6.h),
                      Text(
                        'ID: ${customer.id}',
                        style: TextStyle(
                          fontSize: 14.sp,
                          fontWeight: FontWeight.w400,
                          color: AppColors.textSecondary,
                        ),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ],
                  ),
                ),

                SizedBox(width: 8.w),

                // Amount + visit count (right-aligned)
                Column(
                  crossAxisAlignment: CrossAxisAlignment.end,
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    Text(
                      _formatCurrency(customer.totalSpent),
                      style: TextStyle(
                        fontSize: 16.sp,
                        fontWeight: FontWeight.w700,
                        color: AppColors.primary,
                      ),
                    ),
                    SizedBox(height: 6.h),
                    Text(
                      '${customer.orderCount} 次消費',
                      style: TextStyle(
                        fontSize: 14.sp,
                        fontWeight: FontWeight.w400,
                        color: AppColors.textSecondary,
                      ),
                    ),
                  ],
                ),
              ],
            ),
            // Tags row — show first 3 tags from API
            if (customer.tags.isNotEmpty) ...[
              SizedBox(height: 8.h),
              Row(
                children: [
                  SizedBox(width: 72.w), // align with text after avatar
                  Expanded(
                    child: Wrap(
                      spacing: 6.w,
                      runSpacing: 4.h,
                      children: customer.tags.take(3).map((tag) {
                        return Container(
                          padding: EdgeInsets.symmetric(
                            horizontal: 8.w,
                            vertical: 4.h,
                          ),
                          decoration: BoxDecoration(
                            color: AppColors.primarySoft,
                            borderRadius: BorderRadius.circular(999.r),
                          ),
                          child: Text(
                            tag.name,
                            style: TextStyle(
                              fontSize: 12.sp,
                              fontWeight: FontWeight.w400,
                              color: AppColors.textAccentOnTint,
                            ),
                          ),
                        );
                      }).toList(),
                    ),
                  ),
                ],
              ),
            ],
          ],
        ),
      ),
    );
  }
}

class _CustomerErrorState extends StatelessWidget {
  const _CustomerErrorState({required this.message, required this.onRetry});

  final String message;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    return Center(
      key: const ValueKey('seller-customers-error-state'),
      child: HkContentCard(
        padding: const EdgeInsets.all(HkSpace.xl),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const HkIcon(
              HkIconName.warning,
              size: 32,
              color: AppIconColors.danger,
            ),
            const SizedBox(height: HkSpace.sm),
            Text(
              message,
              textAlign: TextAlign.center,
              style: AppTypography.sectionTitle,
            ),
            const SizedBox(height: HkSpace.md),
            FilledButton.tonal(
              onPressed: onRetry,
              child: const HkButtonLabel('重新載入'),
            ),
          ],
        ),
      ),
    );
  }
}
