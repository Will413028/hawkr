import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import 'hk_icon.dart';

/// 聊天室「選擇訂單」半屏 BottomSheet
///
/// 列出跟當前聊天對象有關的訂單，讓賣家 / 買家選一筆送出成訂單卡片。
/// 用法跟 `address_picker_sheet.dart` 相同風格。
///
/// 回傳被選中的 order `Map<String, dynamic>`，或 null（取消選擇）。
/// [isSeller] 決定卡片顯示「買家：XXX」(賣家視角) 或「賣家：XXX」(買家視角)
Future<Map<String, dynamic>?> showOrderPickerSheet({
  required BuildContext context,
  required List<Map<String, dynamic>> orders,
  bool loading = false,
  bool isSeller = true,
}) {
  return showModalBottomSheet<Map<String, dynamic>>(
    context: context,
    isScrollControlled: true,
    useSafeArea: true,
    backgroundColor: AppColors.bgCard,
    shape: RoundedRectangleBorder(
      borderRadius: BorderRadius.vertical(top: Radius.circular(16.r)),
    ),
    builder: (ctx) =>
        _OrderPickerBody(orders: orders, loading: loading, isSeller: isSeller),
  );
}

class _OrderPickerBody extends StatelessWidget {
  const _OrderPickerBody({
    required this.orders,
    this.loading = false,
    this.isSeller = true,
  });
  final List<Map<String, dynamic>> orders;
  final bool loading;
  final bool isSeller;

  @override
  Widget build(BuildContext context) {
    return DraggableScrollableSheet(
      initialChildSize: 0.55,
      minChildSize: 0.35,
      maxChildSize: 0.85,
      expand: false,
      builder: (ctx, scrollCtrl) => Column(
        children: [
          // 頂部把手 + 標題
          Padding(
            padding: EdgeInsets.only(top: 12.h),
            child: Container(
              width: 40.w,
              height: 4.h,
              decoration: BoxDecoration(
                color: AppColors.dividerLight,
                borderRadius: BorderRadius.circular(2.r),
              ),
            ),
          ),
          Padding(
            padding: EdgeInsets.symmetric(vertical: 12.h),
            child: Text(
              '選擇訂單',
              style: TextStyle(
                fontSize: 18.sp,
                fontWeight: FontWeight.w700,
                color: AppColors.textEmphasis,
              ),
            ),
          ),
          const Divider(height: 1),
          // 列表
          Expanded(
            child: loading
                ? const Center(
                    child: CircularProgressIndicator(color: AppColors.primary),
                  )
                : orders.isEmpty
                ? Center(
                    child: Text(
                      '暫無訂單',
                      style: TextStyle(
                        fontSize: 14.sp,
                        color: AppColors.textSecondary,
                      ),
                    ),
                  )
                : ListView.separated(
                    controller: scrollCtrl,
                    padding: EdgeInsets.symmetric(
                      horizontal: 16.w,
                      vertical: 12.h,
                    ),
                    itemCount: orders.length,
                    separatorBuilder: (_, __) => SizedBox(height: 10.h),
                    itemBuilder: (_, i) => _OrderPickerCard(
                      order: orders[i],
                      isSeller: isSeller,
                      onTap: () => Navigator.pop(ctx, orders[i]),
                    ),
                  ),
          ),
        ],
      ),
    );
  }
}

/// 訂單卡片（picker 用白底版，聊天泡泡用 [ChatOrderBubble]）
class _OrderPickerCard extends StatelessWidget {
  const _OrderPickerCard({
    required this.order,
    required this.onTap,
    this.isSeller = true,
  });
  final Map<String, dynamic> order;
  final VoidCallback onTap;
  final bool isSeller;

  @override
  Widget build(BuildContext context) {
    final orderNo = (order['orderNo'] ?? '').toString();
    final status = (order['status'] ?? '').toString();
    final statusLabel = _statusLabel(status);
    final amount =
        (order['totalActualAmount'] ??
                order['total_actual_amount'] ??
                order['totalAmount'] ??
                '')
            .toString();
    final items = _itemsSummary(order);
    final buyerName =
        (order['buyerNickname'] ??
                order['buyer_nickname'] ??
                order['recipientName'] ??
                '')
            .toString();
    final sellerName = (order['sellerName'] ?? '').toString();
    // 賣家看買家名、買家看賣家名
    final displayName = isSeller
        ? (buyerName.isNotEmpty ? '買家：$buyerName' : '')
        : (sellerName.isNotEmpty ? '賣家：$sellerName' : '');
    final date = _formatDate(
      order['createTime']?.toString() ?? order['create_time']?.toString() ?? '',
    );

    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 12.h),
        decoration: BoxDecoration(
          color: AppColors.bgCard,
          borderRadius: BorderRadius.circular(16.r),
          border: Border.all(color: AppColors.cardBorder),
          boxShadow: const [
            BoxShadow(
              color: AppColors.cardShadow,
              blurRadius: 4,
              offset: Offset(0, 2),
            ),
          ],
        ),
        child: Row(
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  // Row 1: #orderNo + status + $amount
                  Row(
                    children: [
                      Flexible(
                        child: Text(
                          '#$orderNo',
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: TextStyle(
                            fontSize: 14.sp,
                            fontWeight: FontWeight.w700,
                            color: AppColors.textPrimary,
                          ),
                        ),
                      ),
                      SizedBox(width: 8.w),
                      if (statusLabel.isNotEmpty)
                        Container(
                          padding: EdgeInsets.symmetric(
                            horizontal: 8.w,
                            vertical: 4.h,
                          ),
                          decoration: BoxDecoration(
                            color: const Color(0x80E0C44C),
                            borderRadius: BorderRadius.circular(999.r),
                          ),
                          child: Text(
                            statusLabel,
                            style: TextStyle(
                              fontSize: 12.sp,
                              fontWeight: FontWeight.w700,
                              color: AppColors.textPrimary,
                              letterSpacing: 1,
                            ),
                          ),
                        ),
                      const Spacer(),
                      if (amount.isNotEmpty)
                        Text(
                          '\$$amount',
                          style: TextStyle(
                            fontSize: 14.sp,
                            fontWeight: FontWeight.w700,
                            color: AppColors.textPrimary,
                          ),
                        ),
                    ],
                  ),
                  SizedBox(height: 6.h),
                  // Row 2: items summary
                  if (items.isNotEmpty)
                    Row(
                      children: [
                        HkIcon(
                          HkIconName.box,
                          size: 16.sp,
                          color: AppIconColors.defaultColor,
                        ),
                        SizedBox(width: 6.w),
                        Expanded(
                          child: Text(
                            items,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: TextStyle(
                              fontSize: 12.sp,
                              color: AppColors.textPrimary,
                            ),
                          ),
                        ),
                      ],
                    ),
                  SizedBox(height: 6.h),
                  // Row 3: name + date
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      if (displayName.isNotEmpty)
                        Flexible(
                          child: Text(
                            displayName,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: TextStyle(
                              fontSize: 14.sp,
                              color: AppColors.textPrimary,
                            ),
                          ),
                        ),
                      if (date.isNotEmpty)
                        Text(
                          date,
                          style: TextStyle(
                            fontSize: 14.sp,
                            color: AppColors.textPrimary,
                          ),
                        ),
                    ],
                  ),
                ],
              ),
            ),
            SizedBox(width: 4.w),
            Icon(
              AppIcons.chevronRight,
              size: 24.sp,
              color: AppIconColors.defaultColor,
            ),
          ],
        ),
      ),
    );
  }

  String _statusLabel(String status) {
    switch (status) {
      case '0':
        return '待付款';
      case '10':
        return '已付款';
      case '20':
        return '待出貨';
      case '30':
        return '配送中';
      case '32':
        return '待取貨';
      case '35':
        return '配送異常';
      case '40':
        return '已送達';
      case '50':
        return '已完成';
      case '60':
        return '部分退款';
      case '70':
        return '全額退款';
      case '80':
        return '已取消';
      default:
        return '';
    }
  }

  String _itemsSummary(Map<String, dynamic> o) {
    final items = o['items'];
    if (items is List && items.isNotEmpty) {
      return items
          .take(3)
          .map((e) {
            final name = (e['productName'] ?? e['skuName'] ?? e['name'] ?? '')
                .toString();
            final qty = (e['quantity'] ?? '1').toString();
            return '$name x$qty';
          })
          .join(', ');
    }
    final count = o['itemCount']?.toString() ?? '';
    return count.isNotEmpty ? '共 $count 件商品' : '';
  }

  String _formatDate(String raw) {
    if (raw.isEmpty) return '';
    final dt = DateTime.tryParse(raw);
    if (dt == null) return raw;
    final l = dt.toLocal();
    return '${l.year}/${l.month.toString().padLeft(2, '0')}/${l.day.toString().padLeft(2, '0')}';
  }
}
