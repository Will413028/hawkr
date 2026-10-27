import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/core/theme/app_typography.dart';
import 'package:hawkr_seller/core/theme/hk_tokens.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../config/image_config.dart';
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

class CustomerManagementScreen extends HookConsumerWidget {
  const CustomerManagementScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final searchCtrl = useTextEditingController();

    // Watch filter state
    final filter = ref.watch(customerFilterProvider);
    final filterNotifier = ref.read(customerFilterProvider.notifier);

    String sortLabel(String v) => switch (v) {
      'visitCount' => '消費次數',
      'totalSpent' => '消費金額',
      'lastVisit' => '最近光顧',
      _ => '消費次數',
    };

    // Watch list state
    final asyncCustomers = ref.watch(customerListProvider);

    // Helper to handle debounce
    final debouncer = useMemoized(() => _Debouncer(milliseconds: 500));

    // Sync text controller with filter state initially or when changed externally
    // But we avoid loop by checking value
    useEffect(() {
      if (searchCtrl.text != (filter.keyword ?? '')) {
        searchCtrl.text = filter.keyword ?? '';
      }
      return null;
    }, []);

    // Listen to text changes
    useEffect(() {
      void listener() {
        debouncer.run(() {
          if (searchCtrl.text != (filter.keyword ?? '')) {
            filterNotifier.setKeyword(searchCtrl.text);
          }
        });
      }

      searchCtrl.addListener(listener);
      return () => searchCtrl.removeListener(listener);
    }, [searchCtrl]);

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
            // Search and Filter Row
            _CustomerControls(
              searchController: searchCtrl,
              sortLabel: sortLabel(filter.sortBy),
              onSortSelected: (label) {
                final value = switch (label) {
                  '消費次數' => 'visitCount',
                  '消費金額' => 'totalSpent',
                  '最近光顧' => 'lastVisit',
                  _ => 'visitCount',
                };
                filterNotifier.setSortBy(value);
              },
            ),
            // Tab Bar
            _CustomerTabBar(
              selectedStatus: filter.customerType,
              onTabChanged: (type) => filterNotifier.setCustomerType(type),
            ),
            // Customer List
            Expanded(
              // 里面的内容 垂直start
              child: asyncCustomers.when(
                data: (customers) {
                  if (customers.isEmpty) {
                    final isSearching =
                        filter.keyword?.trim().isNotEmpty == true;
                    return HkEmptyState(
                      key: const ValueKey('seller-customers-empty-state'),
                      iconKey: const ValueKey(
                        'seller-customers-empty-state-icon',
                      ),
                      icon: isSearching ? HkIconName.search : HkIconName.users,
                      message: isSearching ? '找不到相關顧客' : '暫無顧客',
                      description: isSearching
                          ? '試試不同的關鍵字，或清除搜尋條件。'
                          : '有人下單後會自動加入這份名單。',
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
                        itemCount: customers.length,
                        itemBuilder: (context, index) {
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

class _CustomerControls extends StatelessWidget {
  const _CustomerControls({
    required this.searchController,
    required this.sortLabel,
    required this.onSortSelected,
  });

  final TextEditingController searchController;
  final String sortLabel;
  final ValueChanged<String> onSortSelected;

  @override
  Widget build(BuildContext context) {
    final search = SizedBox(
      height: HkControl.minTap,
      child: TextField(
        controller: searchController,
        textAlignVertical: TextAlignVertical.center,
        style: AppTypography.body,
        decoration: InputDecoration(
          hintText: '搜尋姓名或電話',
          hintStyle: AppTypography.body.copyWith(
            color: AppColors.textSecondary,
          ),
          prefixIcon: const HkIcon(
            HkIconName.search,
            size: 20,
            color: AppIconColors.muted,
          ),
          filled: true,
          fillColor: AppColors.neutralWhite.withValues(alpha: 0.74),
          contentPadding: const EdgeInsets.symmetric(
            horizontal: HkSpace.md,
            vertical: HkSpace.md,
          ),
          border: OutlineInputBorder(
            borderRadius: BorderRadius.circular(HkRadius.pill),
            borderSide: const BorderSide(color: AppColors.cardBorder),
          ),
          enabledBorder: OutlineInputBorder(
            borderRadius: BorderRadius.circular(HkRadius.pill),
            borderSide: const BorderSide(color: AppColors.cardBorder),
          ),
          focusedBorder: OutlineInputBorder(
            borderRadius: BorderRadius.circular(HkRadius.pill),
            borderSide: const BorderSide(
              color: AppColors.actionPrimary,
              width: 1.5,
            ),
          ),
        ),
      ),
    );
    final sort = AppDropdown(
      items: const ['消費次數', '消費金額', '最近光顧'],
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
              child: LayoutBuilder(
                builder: (context, constraints) {
                  if (constraints.maxWidth < 420) {
                    return Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        search,
                        const SizedBox(height: HkSpace.sm),
                        Align(alignment: Alignment.centerRight, child: sort),
                      ],
                    );
                  }
                  return Row(
                    children: [
                      Expanded(child: search),
                      const SizedBox(width: HkSpace.sm),
                      sort,
                    ],
                  );
                },
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _CustomerTabBar extends StatelessWidget {
  final String? selectedStatus;
  final ValueChanged<String?> onTabChanged;

  const _CustomerTabBar({
    required this.selectedStatus,
    required this.onTabChanged,
  });

  @override
  Widget build(BuildContext context) {
    final tabs = <({String label, String? status})>[
      (label: '全部顧客', status: null),
      (label: 'VIP', status: 'vip'),
      (label: '黑名單', status: 'blocklist'),
    ];
    final selectedIndex = tabs.indexWhere(
      (tab) => tab.status == selectedStatus,
    );
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: HkSpace.lg),
      child: HkGlassTabBar(
        labels: tabs.map((tab) => tab.label).toList(),
        selectedIndex: selectedIndex < 0 ? 0 : selectedIndex,
        onTap: (index) => onTabChanged(tabs[index].status),
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
      semanticLabel: '查看顧客 ${customer.name}',
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
                // Avatar with VIP / blacklist badge
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
                    // VIP badge — hidden when blacklisted (blacklist takes priority)
                    if (customer.isVip && !customer.isBlacklisted)
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
                    // Blacklist badge
                    if (customer.isBlacklisted)
                      Positioned(
                        right: -4.w,
                        bottom: -8.h,
                        child: Container(
                          padding: EdgeInsets.symmetric(
                            horizontal: 8.w,
                            vertical: 4.h,
                          ),
                          decoration: BoxDecoration(
                            color: AppColors.textSecondary,
                            borderRadius: BorderRadius.circular(12.r),
                          ),
                          child: Text(
                            '黑名單',
                            style: TextStyle(
                              fontSize: 12.sp,
                              fontWeight: FontWeight.w500,
                              color: AppColors.bgCard,
                            ),
                          ),
                        ),
                      ),
                  ],
                ),
                SizedBox(width: 16.w),

                // Name + phone
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      Text(
                        customer.name,
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
                        customer.phone.isNotEmpty
                            ? customer.phone
                            : 'ID: ${customer.id}',
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
                      '${customer.visitCount} 次消費',
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
  const _CustomerErrorState({required this.onRetry});

  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    return Center(
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
            Text('顧客資料載入失敗', style: AppTypography.sectionTitle),
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

class _Debouncer {
  final int milliseconds;
  Timer? _timer;

  _Debouncer({required this.milliseconds});

  void run(VoidCallback action) {
    _timer?.cancel();
    _timer = Timer(Duration(milliseconds: milliseconds), action);
  }
}
