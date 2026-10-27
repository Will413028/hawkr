import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../data/customer_model.dart';
import '../../coupon/data/coupon_model.dart';
import '../../coupon/presentation/add_coupon_screen.dart';
import '../../coupon/presentation/coupon_card_widget.dart';
import '../../../shared/widgets/app_dropdown.dart';
import '../../../shared/widgets/hk_glass_app_bar.dart';
import '../../../shared/widgets/glass_tab_bar.dart';
import '../../../shared/widgets/hk_empty_state.dart';
import 'customer_order_card_widget.dart';

/// 顧客消費歷史頁面 — 3-tab view: 消費金額 | 訂單 | 優惠卷
class CustomerSpendingHistoryScreen extends HookConsumerWidget {
  final String customerName;
  final String customerId;
  final String customerPhone;
  final int totalSpent;
  final int totalRefund;
  final List<SpendingRecord> records;
  final List<OrderRecord> orders;
  final List<Coupon> coupons;
  final int initialTab;

  const CustomerSpendingHistoryScreen({
    super.key,
    required this.customerName,
    required this.customerId,
    required this.customerPhone,
    required this.totalSpent,
    required this.totalRefund,
    required this.records,
    required this.orders,
    required this.coupons,
    this.initialTab = 0,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final selectedTab = useState(initialTab);

    const tabs = ['消費金額', '訂單', '優惠卷'];

    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      appBar: HkGlassAppBar(title: Text(customerName), subtitle: '顧客消費紀錄'),
      body: Column(
        children: [
          HkGlassTabBar(
            labels: tabs,
            selectedIndex: selectedTab.value,
            onTap: (index) => selectedTab.value = index,
          ),
          Expanded(
            child: IndexedStack(
              index: selectedTab.value,
              children: [
                _SpendingTab(
                  records: records,
                  totalSpent: totalSpent,
                  totalRefund: totalRefund,
                ),
                _OrdersTab(orders: orders, customerName: customerName),
                _CouponsTab(
                  coupons: coupons,
                  customerName: customerName,
                  customerId: customerId,
                  customerPhone: customerPhone,
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Tab 0 — 消費金額
// ---------------------------------------------------------------------------

class _SpendingTab extends StatelessWidget {
  final List<SpendingRecord> records;
  final int totalSpent;
  final int totalRefund;

  const _SpendingTab({
    required this.records,
    required this.totalSpent,
    required this.totalRefund,
  });

  @override
  Widget build(BuildContext context) {
    // Filter out records with no actual data (amount == 0 and no orderId)
    final validRecords = records
        .where((r) => r.amount > 0 || r.orderId.isNotEmpty)
        .toList();

    if (validRecords.isEmpty) {
      return const _EmptyState(icon: HkIconName.chart, message: '暫無消費');
    }

    return SingleChildScrollView(
      padding: EdgeInsets.only(top: 16.h),
      child: Column(
        children: [
          _SummaryCard(totalSpent: totalSpent, totalRefund: totalRefund),
          SizedBox(height: 20.h),
          _SpendingTable(records: validRecords),
        ],
      ),
    );
  }
}

/// Figma: white card, rounded-15, border rgba(86,86,86,0.2),
/// shadow 0 1 2 rgba(0,0,0,0.1), px-16 py-12
/// Two sections w-120 each, gap-32, vertical divider 31px
/// Labels: 16sp Bold #565656 center, line under label, gap-7
/// Values: 20sp Bold primary center
class _SummaryCard extends StatelessWidget {
  final int totalSpent;
  final int totalRefund;

  const _SummaryCard({required this.totalSpent, required this.totalRefund});

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: EdgeInsets.symmetric(horizontal: 16.w),
      padding: EdgeInsets.symmetric(horizontal: 16.w, vertical: 12.h),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(16.r),
        border: Border.all(color: AppColors.textPrimary.withValues(alpha: 0.2)),
        boxShadow: const [
          BoxShadow(
            color: AppColors.cardShadow,
            blurRadius: 2,
            offset: Offset(0, 1),
          ),
        ],
      ),
      child: IntrinsicHeight(
        child: Row(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            // Left section
            SizedBox(
              width: 120.w,
              child: Column(
                children: [
                  Text(
                    '累積消費總額',
                    textAlign: TextAlign.center,
                    style: TextStyle(
                      fontSize: 16.sp,
                      fontWeight: FontWeight.w700,
                      color: AppColors.textPrimary,
                      height: 1.5,
                    ),
                  ),
                  SizedBox(height: 7.h),
                  Container(
                    height: 1,
                    color: AppColors.textPrimary.withValues(alpha: 0.2),
                  ),
                  SizedBox(height: 7.h),
                  Text(
                    'NT\$${_fmt(totalSpent)}',
                    textAlign: TextAlign.center,
                    style: TextStyle(
                      fontSize: 20.sp,
                      fontWeight: FontWeight.w700,
                      color: AppColors.primary,
                      height: 1.4,
                    ),
                  ),
                ],
              ),
            ),
            SizedBox(width: 32.w),
            // Vertical divider
            Container(
              width: 1,
              height: 31.h,
              color: AppColors.textPrimary.withValues(alpha: 0.2),
            ),
            SizedBox(width: 32.w),
            // Right section
            SizedBox(
              width: 120.w,
              child: Column(
                children: [
                  Text(
                    '退回總額',
                    textAlign: TextAlign.center,
                    style: TextStyle(
                      fontSize: 16.sp,
                      fontWeight: FontWeight.w700,
                      color: AppColors.textPrimary,
                      height: 1.5,
                    ),
                  ),
                  SizedBox(height: 7.h),
                  Container(
                    height: 1,
                    color: AppColors.textPrimary.withValues(alpha: 0.2),
                  ),
                  SizedBox(height: 7.h),
                  Text(
                    'NT\$${_fmt(totalRefund)}',
                    textAlign: TextAlign.center,
                    style: TextStyle(
                      fontSize: 20.sp,
                      fontWeight: FontWeight.w700,
                      color: AppColors.primary,
                      height: 1.4,
                    ),
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

/// Figma: header bg primary 50% rounded-10 py-8
/// Headers: 16sp Bold #565656 center, w-92 each, vertical dividers
/// Data rows: gap-5, px-11 py-6, 16sp Regular #565656 center, w-92 each
class _SpendingTable extends StatelessWidget {
  final List<SpendingRecord> records;

  const _SpendingTable({required this.records});

  static const _headers = ['日期', '訂單編號', '消費金額', '退回'];

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        // Header row
        Container(
          decoration: BoxDecoration(
            color: AppColors.primary.withValues(alpha: 0.5),
            borderRadius: BorderRadius.circular(12.r),
          ),
          padding: EdgeInsets.symmetric(vertical: 8.h),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.center,
            children: _headers.asMap().entries.map((entry) {
              final i = entry.key;
              final h = entry.value;
              return Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  if (i > 0) _verticalDivider(),
                  SizedBox(
                    width: 92.w,
                    child: Text(
                      h,
                      textAlign: TextAlign.center,
                      style: TextStyle(
                        fontSize: 16.sp,
                        fontWeight: FontWeight.w700,
                        color: AppColors.textPrimary,
                        height: 1.5,
                      ),
                    ),
                  ),
                ],
              );
            }).toList(),
          ),
        ),
        // Data rows — gap-5 between header and first row
        ...records.map((r) {
          return Padding(
            padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 8.h),
            child: Row(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                _dataCell(r.date),
                SizedBox(width: 8.w),
                _dataCell(r.orderId),
                SizedBox(width: 8.w),
                _dataCell('NT\$${_fmt(r.amount)}'),
                SizedBox(width: 8.w),
                _dataCell(r.refund > 0 ? 'NT\$${_fmt(r.refund)}' : 'NT\$0'),
              ],
            ),
          );
        }),
      ],
    );
  }

  Widget _verticalDivider() {
    return SizedBox(
      height: 17.h,
      child: VerticalDivider(
        width: 0,
        thickness: 1,
        color: AppColors.textPrimary.withValues(alpha: 0.3),
      ),
    );
  }

  Widget _dataCell(String text) {
    return Flexible(
      child: SizedBox(
        width: 92.w,
        child: Text(
          text,
          textAlign: TextAlign.center,
          style: TextStyle(
            fontSize: 16.sp,
            fontWeight: FontWeight.w400,
            color: AppColors.textPrimary,
            height: 1.5,
          ),
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Tab 1 — 訂單
// ---------------------------------------------------------------------------

class _OrdersTab extends HookWidget {
  final List<OrderRecord> orders;
  final String customerName;

  const _OrdersTab({required this.orders, required this.customerName});

  @override
  Widget build(BuildContext context) {
    final filterLabel = useState('全部');

    if (orders.isEmpty) {
      return const _EmptyState(icon: HkIconName.box, message: '暫無訂單');
    }

    List<OrderRecord> filtered() {
      return switch (filterLabel.value) {
        '訂單完成' =>
          orders.where((o) => o.status == OrderStatus.completed).toList(),
        '訂單進行中' =>
          orders.where((o) => o.status == OrderStatus.delivering).toList(),
        '訂單取消' =>
          orders.where((o) => o.status == OrderStatus.cancelled).toList(),
        '售後處理' => orders.where((o) => o.status == OrderStatus.issue).toList(),
        _ => orders,
      };
    }

    final visibleOrders = filtered();

    return Column(
      children: [
        // Filter row — Figma: filter icon in header right area
        Padding(
          padding: EdgeInsets.fromLTRB(24.w, 12.h, 24.w, 4.h),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.end,
            children: [
              AppDropdown(
                items: const ['全部', '訂單完成', '訂單進行中', '訂單取消', '售後處理'],
                selectedItem: filterLabel.value,
                onSelected: (v) => filterLabel.value = v,
                showSearch: false,
                child: Container(
                  height: 36.h,
                  padding: EdgeInsets.symmetric(horizontal: 12.w),
                  decoration: BoxDecoration(
                    color: AppColors.bgCard,
                    borderRadius: BorderRadius.circular(999.r),
                    boxShadow: [
                      BoxShadow(
                        color: AppColors.cardShadow,
                        blurRadius: 6.r,
                        offset: Offset(0, 2.h),
                      ),
                    ],
                    border: Border.all(color: AppColors.cardBorder),
                  ),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      SizedBox(
                        width: 16.w,
                        height: 16.h,
                        child: CustomPaint(painter: _FilterIconPainter()),
                      ),
                      SizedBox(width: 6.w),
                      Text(
                        filterLabel.value,
                        style: TextStyle(
                          fontSize: 14.sp,
                          color: AppColors.textPrimary,
                        ),
                      ),
                      SizedBox(width: 6.w),
                      Icon(
                        AppIcons.keyboardArrowDown,
                        size: 16.sp,
                        color: AppIconColors.defaultColor,
                      ),
                    ],
                  ),
                ),
              ),
            ],
          ),
        ),
        // Order cards — Figma: 370w (22px padding each side), gap-16
        Expanded(
          child: visibleOrders.isEmpty
              ? const _EmptyState(icon: HkIconName.box, message: '暫無訂單')
              : ListView.separated(
                  padding: EdgeInsets.symmetric(
                    horizontal: 24.w,
                    vertical: 16.h,
                  ),
                  itemCount: visibleOrders.length,
                  separatorBuilder: (_, __) => SizedBox(height: 16.h),
                  itemBuilder: (context, index) {
                    return CustomerOrderCardWidget(
                      order: visibleOrders[index],
                      customerName: customerName,
                    );
                  },
                ),
        ),
      ],
    );
  }
}

// ---------------------------------------------------------------------------
// Tab 2 — 優惠卷
// ---------------------------------------------------------------------------

class _CouponsTab extends StatelessWidget {
  final List<Coupon> coupons;
  final String customerName;
  final String customerId;
  final String customerPhone;

  const _CouponsTab({
    required this.coupons,
    required this.customerName,
    required this.customerId,
    required this.customerPhone,
  });

  @override
  Widget build(BuildContext context) {
    return Stack(
      children: [
        coupons.isEmpty
            ? const _EmptyState(icon: HkIconName.coupon, message: '暫無優惠卷')
            : ListView.separated(
                padding: EdgeInsets.fromLTRB(24.w, 16.h, 24.w, 80.h),
                itemCount: coupons.length,
                separatorBuilder: (_, __) => SizedBox(height: 16.h),
                itemBuilder: (context, index) {
                  final coupon = coupons[index];
                  return CouponCardWidget(
                    coupon: coupon,
                    // actionLabel: '收回',
                    // actionColor: AppColors.warning,
                    // onEdit: () {
                    //   // revoke placeholder
                    // },
                  );
                },
              ),
        // FAB — Figma: centered horizontally, above home indicator
        Positioned(
          left: 0,
          right: 0,
          bottom: 24.h,
          child: Center(
            child: GestureDetector(
              onTap: () {
                context.push(
                  '/coupon/add',
                  extra: {
                    'preselectedUser': PreselectedUser(
                      name: customerName,
                      id: customerId,
                      phone: customerPhone,
                    ),
                  },
                );
              },
              child: Container(
                width: 60,
                height: 60,
                decoration: const BoxDecoration(
                  color: AppColors.primary,
                  shape: BoxShape.circle,
                  boxShadow: [
                    BoxShadow(
                      color: AppColors.cardShadow,
                      blurRadius: 12,
                      offset: Offset(0, 6),
                    ),
                  ],
                ),
                child: Center(
                  child: Icon(
                    AppIcons.add,
                    size: 28.w,
                    color: AppIconColors.onBrand,
                  ),
                ),
              ),
            ),
          ),
        ),
      ],
    );
  }
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

/// 顧客詳情三個分頁共用的語意空狀態。
class _EmptyState extends StatelessWidget {
  final HkIconName icon;
  final String message;

  const _EmptyState({required this.icon, required this.message});

  @override
  Widget build(BuildContext context) {
    return HkEmptyState(icon: icon, message: message, topPadding: 40);
  }
}

class _FilterIconPainter extends CustomPainter {
  @override
  void paint(Canvas canvas, Size size) {
    final paint = Paint()
      ..color = AppColors.textPrimary
      ..strokeWidth = 1.5
      ..strokeCap = StrokeCap.round;
    canvas.drawLine(Offset(2.w, 4.h), Offset(14.w, 4.h), paint);
    canvas.drawLine(Offset(4.w, 8.h), Offset(12.w, 8.h), paint);
    canvas.drawLine(Offset(6.w, 12.h), Offset(10.w, 12.h), paint);
  }

  @override
  bool shouldRepaint(covariant CustomPainter oldDelegate) => false;
}

String _fmt(int number) {
  final str = number.toString();
  final buffer = StringBuffer();
  int count = 0;
  for (int i = str.length - 1; i >= 0; i--) {
    buffer.write(str[i]);
    count++;
    if (count % 3 == 0 && i != 0) {
      buffer.write(',');
    }
  }
  return buffer.toString().split('').reversed.join();
}
