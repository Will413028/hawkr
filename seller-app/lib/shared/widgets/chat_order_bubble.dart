import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import 'hk_icon.dart';

/// 聊天訊息中的訂單卡片泡泡
///
/// 精確對齊 Figma：
/// - 送出（isSelf=true）: node `10022:101541` — bg primary, rounded 20, text #F5F2E8
/// - 收到（isSelf=false）: node `10022:101595` — bg white, border #56565620, rounded 16, text #565656
///
/// 300w × auto, padding 12h/10v, shadow, chevron-right。
class ChatOrderBubble extends StatelessWidget {
  const ChatOrderBubble({
    super.key,
    required this.messageContent,
    required this.isSelf,
    this.onTap,
  });

  final String messageContent;
  final bool isSelf;
  final VoidCallback? onTap;

  static Map<String, dynamic>? tryParseOrder(String content) {
    try {
      final parsed = json.decode(content);
      if (parsed is Map<String, dynamic>) {
        final type = (parsed['messageType'] ?? parsed['type'] ?? '').toString();
        if (type == 'order_prev' || type == 'order_check') {
          // 買家舊格式：{orders: [{orderSn, productName, ...}]} → 攤平到頂層
          if (!parsed.containsKey('orderNo') && parsed['orders'] is List) {
            final orders = parsed['orders'] as List;
            if (orders.isNotEmpty && orders.first is Map) {
              final first = Map<String, dynamic>.from(orders.first as Map);
              // 把 orderSn 映射成 orderNo
              first['orderNo'] ??= first['orderSn'];
              return {...parsed, ...first};
            }
          }
          return parsed;
        }
        if (parsed.containsKey('orderNo')) return parsed;
        // 買家舊格式 fallback
        if (parsed['orders'] is List) {
          final orders = parsed['orders'] as List;
          if (orders.isNotEmpty && orders.first is Map) {
            final first = Map<String, dynamic>.from(orders.first as Map);
            first['orderNo'] ??= first['orderSn'];
            return {...parsed, ...first};
          }
        }
      }
    } catch (_) {}
    return null;
  }

  // ── 顏色 ──
  static const _greenBg = AppColors.primary;
  static const _selfText = AppColors.bgPrimary; // --c3
  static const _recvText = AppColors.textPrimary; // --brand/primary/gray1/100
  static const _pillBg = Color(0x80E0C44C); // orange 50%
  static const _borderColor = AppColors.cardBorder; // --外框線

  @override
  Widget build(BuildContext context) {
    final order = tryParseOrder(messageContent);
    if (order == null) return const SizedBox.shrink();

    final orderNo = (order['orderNo'] ?? '').toString();
    final status = (order['status'] ?? '').toString();
    final amount = (order['totalActualAmount'] ?? order['totalAmount'] ?? '')
        .toString();
    final items = _itemsSummary(order);
    final name = _displayName(order);
    final date = _fmtDate(order['createTime']?.toString() ?? '');

    final textColor = isSelf ? _selfText : _recvText;

    return GestureDetector(
      onTap: onTap,
      child: Container(
        width: 300.w,
        padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 12.h),
        decoration: BoxDecoration(
          color: isSelf ? _greenBg : AppColors.bgCard,
          borderRadius: BorderRadius.circular(isSelf ? 20.r : 16.r),
          border: isSelf ? null : Border.all(color: _borderColor),
          boxShadow: [
            BoxShadow(
              color: AppColors.cardShadow,
              blurRadius: 4,
              offset: Offset(0, 2.h),
            ),
          ],
        ),
        child: Row(
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  // Row 1: #orderNo + status pill + $amount (Figma gap 8)
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
                            color: textColor,
                            height: 20 / 14,
                          ),
                        ),
                      ),
                      if (_statusLabel(status).isNotEmpty) ...[
                        SizedBox(width: 8.w),
                        Container(
                          padding: EdgeInsets.symmetric(
                            horizontal: 8.w,
                            vertical: 4.h,
                          ),
                          decoration: BoxDecoration(
                            color: _pillBg,
                            borderRadius: BorderRadius.circular(999.r),
                          ),
                          child: Text(
                            _statusLabel(status),
                            style: TextStyle(
                              fontSize: 12.sp,
                              fontWeight: FontWeight.w700,
                              color: textColor,
                              letterSpacing: 1,
                              height: 1.5,
                            ),
                          ),
                        ),
                      ],
                      const Spacer(),
                      if (amount.isNotEmpty)
                        Text(
                          '\$$amount',
                          style: TextStyle(
                            fontSize: 14.sp,
                            fontWeight: FontWeight.w700,
                            color: textColor,
                            height: 20 / 14,
                          ),
                        ),
                    ],
                  ),
                  SizedBox(height: 8.h),
                  // Row 2: pack icon + items (Figma gap 6)
                  if (items.isNotEmpty)
                    Row(
                      children: [
                        HkIcon(HkIconName.box, size: 16.sp, color: textColor),
                        SizedBox(width: 6.w),
                        Expanded(
                          child: Text(
                            items,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: TextStyle(
                              fontSize: 12.sp,
                              color: textColor,
                              height: 16 / 12,
                            ),
                          ),
                        ),
                      ],
                    ),
                  SizedBox(height: 8.h),
                  // Row 3: name + date
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      if (name.isNotEmpty)
                        Flexible(
                          child: Text(
                            name,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: TextStyle(
                              fontSize: 14.sp,
                              color: textColor,
                              height: 1.44,
                            ),
                          ),
                        ),
                      if (date.isNotEmpty)
                        Text(
                          date,
                          style: TextStyle(
                            fontSize: 14.sp,
                            color: textColor,
                            height: 1.44,
                          ),
                        ),
                    ],
                  ),
                ],
              ),
            ),
            SizedBox(width: 4.w),
            Icon(AppIcons.chevronRight, size: 24.sp, color: textColor),
          ],
        ),
      ),
    );
  }

  String _statusLabel(String s) => switch (s) {
    '0' => '待付款',
    '10' => '已付款',
    '20' => '待出貨',
    '30' => '配送中',
    '32' => '待取貨',
    '40' => '已送達',
    '50' => '已完成',
    '60' => '部分退款',
    '70' => '全額退款',
    '80' => '已取消',
    _ => '',
  };

  String _displayName(Map<String, dynamic> o) {
    final buyer = (o['buyerNickname'] ?? o['recipientName'] ?? '').toString();
    if (buyer.isNotEmpty) return '買家：$buyer';
    final seller = (o['sellerName'] ?? '').toString();
    if (seller.isNotEmpty) return '賣家：$seller';
    return '';
  }

  String _itemsSummary(Map<String, dynamic> o) {
    final items = o['items'];
    if (items is List && items.isNotEmpty) {
      return items
          .take(3)
          .map((e) {
            final n = (e['productName'] ?? e['skuName'] ?? e['name'] ?? '')
                .toString();
            final q = (e['quantity'] ?? '1').toString();
            return '$n x$q';
          })
          .join(', ');
    }
    final c = o['itemCount']?.toString() ?? '';
    return c.isNotEmpty ? '共 $c 件商品' : '';
  }

  String _fmtDate(String raw) {
    if (raw.isEmpty) return '';
    final dt = DateTime.tryParse(raw);
    if (dt == null) return raw;
    final l = dt.toLocal();
    return '${l.year}/${l.month.toString().padLeft(2, '0')}/${l.day.toString().padLeft(2, '0')}';
  }
}
