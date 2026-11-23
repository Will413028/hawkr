import 'package:hawkr_seller/l10n/l10n.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hawkr_api/errors.dart' show errorText;
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/l10n/ui_locale.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../../shared/widgets/hk_empty_state.dart';
import '../../../shared/widgets/hk_glass_app_bar.dart';
import '../../../shared/widgets/hk_icon.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import 'customer_controller.dart';
import 'customer_load_more_footer.dart';
import 'customer_order_card_widget.dart';

/// 顧客的訂單：店家訂單列表以 `buyer_id` 篩選。讀取失敗與「沒有訂單」分開呈現。
class CustomerOrdersScreen extends ConsumerWidget {
  const CustomerOrdersScreen({
    super.key,
    required this.customerId,
    required this.customerName,
  });

  final String customerId;
  final String customerName;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final orders = ref.watch(customerOrdersProvider(customerId));
    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      appBar: HkGlassAppBar(
        title: Text(customerName),
        subtitle: context.l10n.customerOrdersTitle,
      ),
      body: orders.when(
        loading: () => const TruckLoadingOverlay(),
        error: (e, _) => Center(
          key: const ValueKey('customer-orders-error'),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              const HkIcon(
                HkIconName.warning,
                size: 32,
                color: AppIconColors.danger,
              ),
              SizedBox(height: 8.h),
              Text(
                errorText(e, defaults: ref.read(defaultErrorTextProvider)),
                textAlign: TextAlign.center,
              ),
              SizedBox(height: 12.h),
              FilledButton.tonal(
                onPressed: () =>
                    ref.invalidate(customerOrdersProvider(customerId)),
                child: HkButtonLabel(context.l10n.commonReload),
              ),
            ],
          ),
        ),
        data: (state) {
          final list = state.orders;
          if (list.isEmpty) {
            return HkEmptyState(
              icon: HkIconName.box,
              message: context.l10n.customerOrdersEmpty,
              topPadding: 40,
            );
          }
          return ListView.separated(
            padding: EdgeInsets.fromLTRB(24.w, 16.h, 24.w, 32.h),
            itemCount: list.length + (state.hasMore ? 1 : 0),
            separatorBuilder: (_, _) => SizedBox(height: 12.h),
            itemBuilder: (_, i) {
              if (i == list.length) {
                return CustomerLoadMoreFooter(
                  errorKey: const ValueKey('customer-orders-load-more-error'),
                  onLoadMore: ref
                      .read(customerOrdersProvider(customerId).notifier)
                      .loadMore,
                  errorMessage: state.loadMoreError == null
                      ? null
                      : errorText(
                          state.loadMoreError!,
                          defaults: ref.read(defaultErrorTextProvider),
                        ),
                );
              }
              return CustomerOrderCardWidget(
                order: list[i],
                customerName: customerName,
              );
            },
          );
        },
      ),
    );
  }
}
