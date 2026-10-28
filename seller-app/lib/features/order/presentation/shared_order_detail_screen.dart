import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../config/image_config.dart';
import '../../../core/codemeta/code_meta_display.dart';
import '../../../core/codemeta/shipping_presentation.dart';
import '../../../core/theme/app_typography.dart';
import '../data/after_sale_status_notifier.dart';
import '../data/order_model.dart';
import '../data/order_repository.dart';
import '../data/order_status_notifier.dart';
import '../../../shared/widgets/hk_glass_app_bar.dart';
import '../../../shared/widgets/optimized_image.dart';
import '../../../shared/widgets/hk_icon.dart';
import '../../../shared/widgets/buyer_avatar_widget.dart';
import '../../../shared/widgets/step_indicator.dart';
import 'order_status_presentation.dart';

class SharedOrderDetailScreen extends HookConsumerWidget {
  const SharedOrderDetailScreen({
    super.key,
    required this.orderId,
    this.initialSummary,
    this.fallbackDateText,
    this.fallbackItemsText,
    this.fallbackAmount,
  });

  final String orderId;
  final OrderSummary? initialSummary;
  final String? fallbackDateText;
  final String? fallbackItemsText;
  final int? fallbackAmount;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final orderDetail = useState<OrderSummary?>(initialSummary);
    final rawJson = useState<Map<String, dynamic>?>(null);
    final loading = useState(initialSummary == null);
    // 遞增此 counter 觸發 useEffect 重新載入（FCM / lifecycle 用）
    final reloadTick = useState(0);

    useEffect(() {
      var disposed = false;

      Future<void> load() async {
        if (orderId.isEmpty) {
          loading.value = false;
          return;
        }

        try {
          final detail = await ref
              .read(orderRepositoryProvider)
              .fetchMatchedOrderDetails(orderSn: orderId);
          if (!disposed) orderDetail.value = detail;
        } catch (_) {
          // Keep initial fallback data if detail fetch fails.
        } finally {
          if (!disposed) loading.value = false;
        }
      }

      Future<void> loadRaw() async {
        if (orderId.isEmpty) return;
        try {
          final json = await ref
              .read(orderRepositoryProvider)
              .fetchRawOrderJson(orderId);
          if (!disposed) rawJson.value = json;
        } catch (_) {}
      }

      load();
      loadRaw();
      return () => disposed = true;
    }, [orderId, reloadTick.value]);

    // FCM Silent Push: 訂單狀態變更，僅當事件 orderSn 與本頁訂單相符才 reload
    ref.listen(orderStatusStreamProvider, (_, next) {
      next.whenData((event) {
        if (event.orderSn == orderId || event.orderId == orderId) {
          reloadTick.value++;
        }
      });
    });

    // FCM Silent Push: 售後案件狀態變更（買家申請/補件等），僅與本訂單相關才 reload
    ref.listen(afterSaleStatusStreamProvider, (_, next) {
      next.whenData((event) {
        if (event.orderNo == orderId) {
          reloadTick.value++;
        }
      });
    });

    final detail = orderDetail.value ?? initialSummary;
    final raw = rawJson.value;
    final detailItems = detail?.items ?? const <OrderItem>[];
    final paymentMethod = _stringValue(
      raw?['paymentMethod'] ?? raw?['payment_method'] ?? raw?['pay_type'],
    );
    final payTime = _stringValue(raw?['payTime'] ?? raw?['pay_time']);
    final orderRemarkText = _stringValue(
      raw?['shippingNote'] ??
          raw?['shipping_note'] ??
          raw?['orderRemark'] ??
          raw?['order_remark'] ??
          raw?['note'] ??
          raw?['remark'],
    );
    // 訂單詳情不再顯示「照片舉證」card（屬於售後爭議流程，已移除）。
    // shipmentPhotoUrls / receiptPhotoUrls 在售後 detail 那邊單獨呈現。

    // 訂單評分（買家評論這筆訂單後可見）
    // 優先從 detail API 自帶欄位讀；沒回的話會在 _OrderRatingCard 內部
    // fallback 到 GET /app/sellers/me/reviews 反查（B-145 完成前的解法）。
    final orderRating = _parseRating(
      raw?['rating'] ?? raw?['reviewRating'] ?? raw?['review']?['rating'],
    );
    final orderReviewContent =
        (raw?['review']?['content'] ?? raw?['reviewContent'] ?? '').toString();
    final isReviewed = raw?['isReviewed'] == true;

    // 收集 OrderItem id 集合給 review 反查用
    final orderItemIds = <String>{};
    final itemsRaw = raw?['items'];
    if (itemsRaw is List) {
      for (final item in itemsRaw) {
        if (item is Map) {
          final id = (item['id'] ?? '').toString().trim();
          if (id.isNotEmpty) orderItemIds.add(id);
        }
      }
    }

    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      appBar: HkGlassAppBar(
        title: const Text('訂單細節'),
        subtitle: orderId.isEmpty ? null : orderId,
        onBackPressed: context.pop,
      ),
      body: loading.value && detail == null
          ? const Center(child: CircularProgressIndicator(strokeWidth: 2))
          : ListView(
              key: const PageStorageKey('seller-order-detail-scroll'),
              padding: EdgeInsets.fromLTRB(24.w, 16.h, 24.w, 32.h),
              children: [
                if (detail != null &&
                    (detail.buyerName.isNotEmpty ||
                        detail.recipientName.isNotEmpty)) ...[
                  _BuyerInfoCard(order: detail),
                  SizedBox(height: 18.h),
                ],
                const _DetailSectionHeader(title: '交易總覽'),
                SizedBox(height: 10.h),
                _TransactionOverviewCard(order: detail, raw: raw),
                SizedBox(height: 16.h),
                _OrderInfoCard(
                  order: detail,
                  orderId: orderId,
                  fallbackDateText: fallbackDateText,
                ),
                SizedBox(height: 18.h),
                const _DetailSectionHeader(title: '配送進度'),
                SizedBox(height: 10.h),
                _DeliveryProgressCard(
                  order: detail,
                  statusCode: _resolvedStatusCode(detail, raw),
                ),
                SizedBox(height: 18.h),
                const _DetailSectionHeader(title: '商品明細'),
                SizedBox(height: 10.h),
                _DetailCard(
                  children: [
                    if (detailItems.isNotEmpty)
                      ...detailItems.asMap().entries.map((entry) {
                        final item = entry.value;
                        final isLast = entry.key == detailItems.length - 1;
                        return Column(
                          children: [
                            _ProductRowRich(item: item),
                            if (!isLast) ...[
                              SizedBox(height: 12.h),
                              Divider(
                                height: 1,
                                color: const Color(
                                  0xFF42493F,
                                ).withValues(alpha: 0.1),
                              ),
                              SizedBox(height: 12.h),
                            ],
                          ],
                        );
                      })
                    else
                      _ProductRow(
                        name: _displayItemName(detail, fallbackItemsText),
                        amount: _resolveFallbackAmount(detail, fallbackAmount),
                      ),
                  ],
                ),
                SizedBox(height: 18.h),
                const _DetailSectionHeader(title: '付款詳情'),
                SizedBox(height: 10.h),
                _DetailCard(
                  children: [
                    _SvgPaymentRow(
                      icon: HkIconName.goods,
                      label: '商品總金額',
                      amount:
                          'NT\$ ${_fmt(_resolveSubtotal(detail, fallbackAmount))}',
                    ),
                    SizedBox(height: 10.h),
                    _SvgPaymentRow(
                      icon: HkIconName.truck,
                      label: '運費總金額',
                      amount: 'NT\$ ${_fmt(_resolveShippingFee(detail))}',
                    ),
                    SizedBox(height: 10.h),
                    _SvgPaymentRow(
                      icon: HkIconName.tag,
                      label: '折扣券',
                      amount:
                          'NT\$ ${_resolveDiscount(detail) > 0 ? '-' : ''}${_fmt(_resolveDiscount(detail))}',
                    ),
                    SizedBox(height: 10.h),
                    Divider(
                      height: 1,
                      color: AppColors.textPrimary.withValues(alpha: 0.1),
                    ),
                    SizedBox(height: 10.h),
                    Row(
                      children: [
                        Text(
                          '總付款金額',
                          style: TextStyle(
                            fontSize: 16.sp,
                            color: AppColors.textSecondary,
                          ),
                        ),
                        const Spacer(),
                        Text(
                          'NT\$ ${_fmt(_resolveTotal(detail, fallbackAmount))}',
                          style: TextStyle(
                            fontSize: 20.sp,
                            fontWeight: FontWeight.w900,
                            color: AppColors.brandPrimaryStrong,
                          ),
                        ),
                      ],
                    ),
                  ],
                ),
                if (orderRemarkText.isNotEmpty) ...[
                  SizedBox(height: 16.h),
                  _DetailCard(
                    children: [
                      Row(
                        children: [
                          HkIcon(
                            HkIconName.edit,
                            size: 22.sp,
                            color: AppIconColors.defaultColor,
                          ),
                          SizedBox(width: 6.w),
                          Text(
                            '訂單備註',
                            style: TextStyle(
                              fontSize: 18.sp,
                              fontWeight: FontWeight.w700,
                              color: AppColors.textPrimary,
                              height: 1.23,
                            ),
                          ),
                        ],
                      ),
                      SizedBox(height: 9.h),
                      Text(
                        orderRemarkText,
                        style: TextStyle(
                          fontSize: 14.sp,
                          color: AppColors.textEmphasis,
                          height: 20 / 14,
                        ),
                      ),
                    ],
                  ),
                ],
                // 商品評分區塊：買家評價過此訂單才顯示
                // 反查 review 是 async，spacing 由 card 內部依
                // 顯示與否管控，避免反查無結果時留下空白 16.h。
                if (isReviewed)
                  _OrderRatingCard(
                    fallbackRating: orderRating,
                    fallbackContent: orderReviewContent,
                    orderItemIds: orderItemIds,
                  ),
                if (paymentMethod.isNotEmpty) ...[
                  SizedBox(height: 16.h),
                  _DetailCard(
                    children: [
                      Row(
                        children: [
                          HkIcon(
                            HkIconName.payment,
                            size: 22.sp,
                            color: AppIconColors.defaultColor,
                          ),
                          SizedBox(width: 6.w),
                          Text(
                            '付款方式',
                            style: TextStyle(
                              fontSize: 18.sp,
                              fontWeight: FontWeight.w700,
                              color: AppColors.textPrimary,
                              height: 1.23,
                            ),
                          ),
                        ],
                      ),
                      SizedBox(height: 12.h),
                      Row(
                        children: [
                          Expanded(
                            child: Text(
                              _paymentMethodLabel(paymentMethod),
                              style: TextStyle(
                                fontSize: 16.sp,
                                color: AppColors.textEmphasis,
                              ),
                            ),
                          ),
                          Container(
                            padding: EdgeInsets.symmetric(
                              horizontal: 8.w,
                              vertical: 4.h,
                            ),
                            decoration: BoxDecoration(
                              color: _isPaid(detail, raw)
                                  ? AppColors.brandPrimaryStrong
                                  : AppColors.warning,
                              borderRadius: BorderRadius.circular(16.r),
                            ),
                            child: Text(
                              _isPaid(detail, raw) ? '已付款' : '待付款',
                              style: TextStyle(
                                fontSize: 14.sp,
                                fontWeight: FontWeight.w700,
                                color:
                                    (_isPaid(detail, raw)
                                                ? AppColors.primaryDark
                                                : AppColors.warning)
                                            .computeLuminance() >
                                        0.45
                                    ? AppColors.textEmphasis
                                    : AppColors.bgCard,
                              ),
                            ),
                          ),
                        ],
                      ),
                      if (payTime.isNotEmpty) ...[
                        SizedBox(height: 12.h),
                        Divider(
                          height: 1,
                          color: AppColors.textPrimary.withValues(alpha: 0.1),
                        ),
                        SizedBox(height: 12.h),
                        Row(
                          children: [
                            Text(
                              '付款時間',
                              style: TextStyle(
                                fontSize: 14.sp,
                                color: AppColors.textPrimary,
                              ),
                            ),
                            const Spacer(),
                            Text(
                              _formatDateTime(payTime),
                              style: TextStyle(
                                fontSize: 14.sp,
                                color: AppColors.textPrimary,
                              ),
                            ),
                          ],
                        ),
                      ],
                    ],
                  ),
                ],
                _PaymentHistoryCard(orderId: orderId),
              ],
            ),
    );
  }
}

class _DetailSectionHeader extends StatelessWidget {
  const _DetailSectionHeader({required this.title});

  final String title;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.symmetric(horizontal: 4.w),
      child: Row(
        children: [
          Container(
            width: 4.w,
            height: 18.h,
            decoration: BoxDecoration(
              color: AppColors.brandPrimaryStrong,
              borderRadius: BorderRadius.circular(2.r),
            ),
          ),
          SizedBox(width: 10.w),
          Text(
            title,
            style: TextStyle(
              fontSize: 18.sp,
              fontWeight: FontWeight.w700,
              color: AppColors.textPrimary,
              letterSpacing: 0.2,
            ),
          ),
        ],
      ),
    );
  }
}

class _BuyerInfoCard extends StatelessWidget {
  const _BuyerInfoCard({required this.order});

  final OrderSummary order;

  @override
  Widget build(BuildContext context) {
    final buyerName = order.buyerName.isNotEmpty
        ? order.buyerName
        : order.recipientName.isNotEmpty
        ? order.recipientName
        : '買家';
    final participantId = order.buyerId?.toString() ?? '';

    void openChat() {
      if (participantId.isEmpty) return;
      context.push(
        '/chat/thread',
        extra: {'title': buyerName, 'participantId': participantId},
      );
    }

    return Material(
      color: Colors.transparent,
      child: InkWell(
        onTap: participantId.isEmpty ? null : openChat,
        borderRadius: BorderRadius.circular(16.r),
        child: Container(
          padding: EdgeInsets.fromLTRB(16.w, 16.h, 12.w, 16.h),
          decoration: _orderDetailCardDecoration(),
          child: Row(
            children: [
              BuyerAvatarWidget(
                size: 44,
                imageUrl: order.buyerAvatar,
                name: buyerName,
              ),
              SizedBox(width: 12.w),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      buyerName,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        fontSize: 18.sp,
                        fontWeight: FontWeight.w700,
                        color: AppColors.textPrimary,
                      ),
                    ),
                    SizedBox(height: 3.h),
                    Row(
                      children: [
                        HkIcon(
                          HkIconName.user,
                          size: 14.sp,
                          color: AppIconColors.muted,
                        ),
                        SizedBox(width: 4.w),
                        Text(
                          participantId.isEmpty ? '買家資訊' : '聯絡買家',
                          style: TextStyle(
                            fontSize: 14.sp,
                            color: AppColors.textSecondary,
                          ),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
              if (participantId.isNotEmpty) ...[
                IconButton(
                  onPressed: openChat,
                  tooltip: '聯絡買家',
                  icon: HkIcon(
                    HkIconName.chat,
                    size: 22.sp,
                    color: AppIconColors.active,
                  ),
                ),
                HkIcon(
                  HkIconName.chevronRight,
                  size: 22.sp,
                  color: AppIconColors.disabled,
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }
}

class _TransactionOverviewCard extends StatelessWidget {
  const _TransactionOverviewCard({required this.order, required this.raw});

  final OrderSummary? order;
  final Map<String, dynamic>? raw;

  @override
  Widget build(BuildContext context) {
    final status = _resolvedStatusCode(order, raw);
    final visual = OrderStatusPresentation.fromOrder(order, statusCode: status);
    final color = visual.color;
    final deadline = order?.autoCancelAt.trim() ?? '';

    return Container(
      width: double.infinity,
      padding: EdgeInsets.all(16.w),
      decoration: _orderDetailCardDecoration(),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                width: 44.w,
                height: 44.w,
                alignment: Alignment.center,
                decoration: BoxDecoration(
                  color: color.withValues(alpha: 0.12),
                  shape: BoxShape.circle,
                ),
                child: HkIcon(visual.icon, size: 24.sp, color: color),
              ),
              SizedBox(width: 14.w),
              Expanded(
                child: Text(
                  visual.label,
                  style: TextStyle(
                    fontSize: 24.sp,
                    fontWeight: FontWeight.w900,
                    color: color,
                    height: 1.1,
                  ),
                ),
              ),
            ],
          ),
          SizedBox(height: 10.h),
          Text(
            visual.description,
            style: TextStyle(
              fontSize: 14.sp,
              height: 1.5,
              color: AppColors.textSecondary,
            ),
          ),
          if (deadline.isNotEmpty && (status == 10 || status == 20)) ...[
            SizedBox(height: 14.h),
            const Divider(height: 1, color: AppColors.borderSubtle),
            SizedBox(height: 10.h),
            _OrderDetailInfoLine(
              label: '出貨處理期限',
              value: _displayOrderDateTime(deadline),
              valueColor: AppColors.warningText,
            ),
          ],
        ],
      ),
    );
  }
}

class _DeliveryProgressCard extends StatelessWidget {
  const _DeliveryProgressCard({required this.order, required this.statusCode});

  final OrderSummary? order;
  final int statusCode;

  @override
  Widget build(BuildContext context) {
    final delivery = DeliveryStatusPresentation.fromOrder(
      order,
      statusCode: statusCode,
    );
    final cancelled = statusCode == 80;
    final shippingMethod = CodeMetaDisplay.shippingMethod(
      order?.shippingMethod ?? '',
      fallback: '尚未指定',
    );
    final shipmentCode = [
      order?.odno.trim() ?? '',
      order?.validationNo.trim() ?? '',
    ].where((value) => value.isNotEmpty).join(' ');
    final rows = <Widget>[
      _OrderDetailInfoLine(label: '配送方式', value: shippingMethod),
      _OrderDetailInfoLine(
        label: '物流貨態',
        value: delivery.label,
        valueColor: delivery.color,
      ),
      if (shipmentCode.isNotEmpty)
        _OrderDetailInfoLine(
          label: '超商寄件碼',
          value: shipmentCode,
          copyValue: shipmentCode,
        ),
    ];

    return Container(
      width: double.infinity,
      padding: EdgeInsets.all(16.w),
      decoration: _orderDetailCardDecoration(),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          StepIndicator(
            steps: cancelled
                ? StepIndicator.cancelledLogisticsSteps
                : StepIndicator.logisticsSteps,
            currentStep: cancelled ? 1 : delivery.progressStep,
          ),
          SizedBox(height: 16.h),
          Container(
            width: double.infinity,
            padding: EdgeInsets.all(12.w),
            decoration: BoxDecoration(
              color: delivery.softColor,
              borderRadius: BorderRadius.circular(12.r),
            ),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                HkIcon(delivery.icon, size: 22.sp, color: delivery.color),
                SizedBox(width: 10.w),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        delivery.label,
                        style: AppTypography.bodyBold.copyWith(
                          color: delivery.color,
                        ),
                      ),
                      SizedBox(height: 4.h),
                      Text(
                        delivery.description,
                        style: AppTypography.metaMuted,
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
          SizedBox(height: 16.h),
          ..._withOrderDetailDividers(rows),
        ],
      ),
    );
  }
}

class _OrderInfoCard extends StatelessWidget {
  const _OrderInfoCard({
    required this.order,
    required this.orderId,
    required this.fallbackDateText,
  });

  final OrderSummary? order;
  final String orderId;
  final String? fallbackDateText;

  @override
  Widget build(BuildContext context) {
    // 收件資訊依配送方式**明確分派**；未知 / 本版未支援者不猜是門市還是宅配。
    final shipping = shippingPresentationOf(order?.shippingMethod);
    final dateSource = order?.dateTime.isNotEmpty == true
        ? order!.dateTime
        : _stringValue(fallbackDateText);
    final rows = <Widget>[
      _OrderDetailInfoLine(
        label: '訂單編號',
        value: orderId.isNotEmpty ? orderId : '—',
        copyValue: orderId,
      ),
      _OrderDetailInfoLine(
        label: '建立時間',
        value: _displayOrderDateTime(dateSource),
      ),
      _OrderDetailInfoLine(
        label: switch (shipping) {
          ShippingPresentation.cvsStorePickup => '取貨門市',
          ShippingPresentation.homeDelivery => '配送地址',
          ShippingPresentation.unsupported => '配送資訊',
        },
        value:
            (shipping != ShippingPresentation.homeDelivery) &&
                (order?.pickupStoreName.isNotEmpty ?? false)
            ? order!.pickupStoreName
            : _displayAddress(order),
      ),
      _OrderDetailInfoLine(label: '聯絡電話', value: _displayPhone(order)),
    ];

    return Container(
      width: double.infinity,
      padding: EdgeInsets.all(16.w),
      decoration: _orderDetailCardDecoration(),
      child: Column(children: _withOrderDetailDividers(rows)),
    );
  }
}

class _OrderDetailInfoLine extends StatelessWidget {
  const _OrderDetailInfoLine({
    required this.label,
    required this.value,
    this.copyValue,
    this.valueColor,
  });

  final String label;
  final String value;
  final String? copyValue;
  final Color? valueColor;

  @override
  Widget build(BuildContext context) {
    final normalized = value.trim().isEmpty ? '—' : value.trim();
    final copyText = copyValue?.trim() ?? '';
    return Row(
      crossAxisAlignment: CrossAxisAlignment.center,
      children: [
        SizedBox(
          width: 88.w,
          child: Text(
            label,
            style: TextStyle(
              fontSize: 14.sp,
              color: AppColors.textSecondary,
              height: 1.45,
            ),
          ),
        ),
        SizedBox(width: 10.w),
        Expanded(
          child: Text(
            normalized,
            textAlign: TextAlign.end,
            style: TextStyle(
              fontSize: 14.sp,
              fontWeight: FontWeight.w500,
              color: valueColor ?? AppColors.textEmphasis,
              height: 1.45,
            ),
          ),
        ),
        if (copyText.isNotEmpty) ...[
          SizedBox(width: 6.w),
          IconButton(
            onPressed: () async {
              await Clipboard.setData(ClipboardData(text: copyText));
              if (!context.mounted) return;
              ScaffoldMessenger.of(
                context,
              ).showSnackBar(const SnackBar(content: Text('已複製訂單編號')));
            },
            tooltip: '複製訂單編號',
            constraints: BoxConstraints.tightFor(width: 36.w, height: 36.w),
            padding: EdgeInsets.zero,
            icon: HkIcon(
              HkIconName.copy,
              size: 18.sp,
              color: AppIconColors.active,
            ),
          ),
        ],
      ],
    );
  }
}

List<Widget> _withOrderDetailDividers(List<Widget> rows) {
  final result = <Widget>[];
  for (var index = 0; index < rows.length; index++) {
    if (index > 0) {
      result
        ..add(SizedBox(height: 10.h))
        ..add(const Divider(height: 1, color: AppColors.borderSubtle))
        ..add(SizedBox(height: 10.h));
    }
    result.add(rows[index]);
  }
  return result;
}

BoxDecoration _orderDetailCardDecoration() => BoxDecoration(
  color: AppColors.bgCard,
  borderRadius: BorderRadius.circular(16.r),
  boxShadow: const [
    BoxShadow(color: AppColors.cardShadow, blurRadius: 8, offset: Offset(0, 2)),
    BoxShadow(color: AppColors.cardShadow, blurRadius: 1),
  ],
);

int _resolvedStatusCode(OrderSummary? order, Map<String, dynamic>? raw) {
  final rawStatus =
      raw?['status'] ?? raw?['orderStatus'] ?? raw?['order_status'];
  return order?.statusCode ??
      int.tryParse(order?.subtitle ?? '') ??
      int.tryParse(rawStatus?.toString() ?? '') ??
      0;
}

String _displayOrderDateTime(String raw) {
  if (raw.trim().isEmpty) return '—';
  final parsed = DateTime.tryParse(raw.trim());
  if (parsed == null) return raw;
  final local = parsed.toLocal();
  String pad(int value) => value.toString().padLeft(2, '0');
  return '${local.year}/${pad(local.month)}/${pad(local.day)} '
      '${pad(local.hour)}:${pad(local.minute)}';
}

String _stringValue(dynamic value) {
  if (value == null) return '';
  final text = value.toString().trim();
  return text == 'null' ? '' : text;
}

/// 解析訂單評分 — 後端可能回 int (1~5)、String "4" 或浮點 "4.0"。
/// 0 表示未評分。
int _parseRating(dynamic value) {
  if (value is int) return value.clamp(0, 5);
  if (value is num) return value.round().clamp(0, 5);
  if (value is String) {
    final n = num.tryParse(value);
    if (n != null) return n.round().clamp(0, 5);
  }
  return 0;
}

String _displayAddress(OrderSummary? detail) {
  if (detail == null) return '—';
  if (detail.address.isNotEmpty) return detail.address;
  if (detail.shippingAddress.isNotEmpty) return detail.shippingAddress;
  return '—';
}

String _displayPhone(OrderSummary? detail) {
  if (detail == null) return '—';
  if (detail.phone.isNotEmpty) return detail.phone;
  return '—';
}

String _displayItemName(OrderSummary? detail, String? fallbackItemsText) {
  if (detail?.productName.isNotEmpty ?? false) return detail!.productName;
  final fallback = _stringValue(fallbackItemsText);
  return fallback.isNotEmpty ? fallback : '商品';
}

int _parsePrice(String raw) {
  final clean = raw.replaceAll(RegExp(r'[^0-9.]'), '');
  if (clean.isEmpty) return 0;
  return (double.tryParse(clean) ?? 0).round();
}

int _resolveFallbackAmount(OrderSummary? detail, int? fallbackAmount) {
  if (detail?.price.isNotEmpty ?? false) return _parsePrice(detail!.price);
  return fallbackAmount ?? 0;
}

int _resolveSubtotal(OrderSummary? detail, int? fallbackAmount) {
  if (detail != null) {
    if (detail.items.isNotEmpty) {
      var subtotal = 0;
      for (final item in detail.items) {
        // PayUni #758: 小計優先用 actualPayAmount（已含 shareDiscount）
        final actual = num.tryParse(item.actualPayAmount);
        if (actual != null && actual > 0) {
          subtotal += actual.round();
        } else {
          subtotal += _parsePrice(item.price) * item.quantity;
        }
      }
      if (subtotal > 0) return subtotal;
    }
    if (detail.price.isNotEmpty) return _parsePrice(detail.price);
  }
  return fallbackAmount ?? 0;
}

int _resolveShippingFee(OrderSummary? detail) =>
    (detail?.shippingFee ?? 0).round();

int _resolveDiscount(OrderSummary? detail) =>
    (detail?.discountAmount ?? 0).round();

int _resolveTotal(OrderSummary? detail, int? fallbackAmount) {
  final total = (detail?.totalAmount ?? 0).round();
  if (total > 0) return total;
  if (fallbackAmount != null && fallbackAmount > 0) return fallbackAmount;
  return _resolveSubtotal(detail, fallbackAmount) +
      _resolveShippingFee(detail) -
      _resolveDiscount(detail);
}

bool _isPaid(OrderSummary? detail, Map<String, dynamic>? raw) {
  if ((detail?.statusCode ?? 0) >= 10) return true;
  final paymentStatus = _stringValue(
    raw?['paymentStatus'] ?? raw?['payment_status'],
  ).toLowerCase();
  return paymentStatus == 'paid' || paymentStatus == 'success';
}

class _SvgPaymentRow extends StatelessWidget {
  const _SvgPaymentRow({
    required this.icon,
    required this.label,
    required this.amount,
  });

  final HkIconName icon;
  final String label;
  final String amount;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        HkIcon(icon, size: 24.w, color: AppIconColors.defaultColor),
        SizedBox(width: 9.w),
        Text(
          label,
          style: TextStyle(
            fontSize: 16.sp,
            color: AppColors.textPrimary,
            height: 1.5,
          ),
        ),
        const Spacer(),
        Text(
          amount,
          style: TextStyle(
            fontSize: 18.sp,
            color: AppColors.textPrimary,
            letterSpacing: 0.77,
          ),
          textAlign: TextAlign.right,
        ),
      ],
    );
  }
}

String _paymentMethodLabel(String method) {
  return CodeMetaDisplay.paymentMethod(method);
}

String _formatDateTime(String raw) {
  if (raw.isEmpty) return '';
  final dt = DateTime.tryParse(raw);
  if (dt == null) return raw;
  final local = dt.toLocal();
  return '${local.year}年${local.month.toString().padLeft(2, '0')}月${local.day.toString().padLeft(2, '0')}日 ${local.hour.toString().padLeft(2, '0')}:${local.minute.toString().padLeft(2, '0')}';
}

String _fmt(int number) {
  final str = number.toString();
  final buffer = StringBuffer();
  var count = 0;
  for (int i = str.length - 1; i >= 0; i--) {
    buffer.write(str[i]);
    count++;
    if (count % 3 == 0 && i != 0) buffer.write(',');
  }
  return buffer.toString().split('').reversed.join();
}

class _DetailCard extends StatelessWidget {
  const _DetailCard({required this.children});

  final List<Widget> children;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: EdgeInsets.all(16.w),
      decoration: _orderDetailCardDecoration(),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: children,
      ),
    );
  }
}

class _PaymentHistoryCard extends ConsumerStatefulWidget {
  const _PaymentHistoryCard({required this.orderId});
  final String orderId;

  @override
  ConsumerState<_PaymentHistoryCard> createState() =>
      _PaymentHistoryCardState();
}

class _PaymentHistoryCardState extends ConsumerState<_PaymentHistoryCard> {
  late final Future<List<Map<String, dynamic>>> _future;

  @override
  void initState() {
    super.initState();
    _future = widget.orderId.isEmpty
        ? Future.value(const <Map<String, dynamic>>[])
        : ref.read(orderRepositoryProvider).fetchPaymentHistory(widget.orderId);
  }

  @override
  Widget build(BuildContext context) {
    return FutureBuilder<List<Map<String, dynamic>>>(
      future: _future,
      builder: (context, snapshot) {
        if (snapshot.connectionState != ConnectionState.done) {
          return const SizedBox.shrink();
        }
        final records = snapshot.data ?? const <Map<String, dynamic>>[];
        if (records.isEmpty) return const SizedBox.shrink();

        return Padding(
          padding: EdgeInsets.only(top: 16.h),
          child: _DetailCard(
            children: [
              Row(
                children: [
                  HkIcon(
                    HkIconName.history,
                    size: 24.w,
                    color: AppIconColors.defaultColor,
                  ),
                  SizedBox(width: 6.w),
                  Text(
                    '付款歷程',
                    style: TextStyle(
                      fontSize: 18.sp,
                      fontWeight: FontWeight.w700,
                      color: AppColors.textPrimary,
                      height: 1.23,
                    ),
                  ),
                ],
              ),
              SizedBox(height: 12.h),
              ...records.asMap().entries.map((entry) {
                final isLast = entry.key == records.length - 1;
                return Column(
                  children: [
                    _PaymentHistoryRow(record: entry.value),
                    if (!isLast) ...[
                      SizedBox(height: 10.h),
                      Divider(
                        height: 1,
                        color: AppColors.textPrimary.withValues(alpha: 0.1),
                      ),
                      SizedBox(height: 10.h),
                    ],
                  ],
                );
              }),
            ],
          ),
        );
      },
    );
  }
}

class _PaymentHistoryRow extends StatelessWidget {
  const _PaymentHistoryRow({required this.record});
  final Map<String, dynamic> record;

  @override
  Widget build(BuildContext context) {
    final paymentType = _historyValue(record, 'paymentType', 'payment_type');
    final status = _historyValue(
      record,
      'status',
      'paymentStatus',
      'payment_status',
    );
    final displayTradeNo = _historyValue(
      record,
      'displayTradeNo',
      'display_trade_no',
    );
    final tradeNo = displayTradeNo.isNotEmpty
        ? displayTradeNo
        : _historyValue(record, 'tradeNo', 'trade_no');
    final amount = _historyValue(record, 'amount');
    final createdAt = _historyValue(record, 'createdAt', 'created_at');
    final payTime = _historyValue(record, 'payTime', 'pay_time');
    final isDuplicate =
        record['isDuplicate'] == true ||
        record['is_duplicate'] == true ||
        _historyValue(
          record,
          'duplicateSequence',
          'duplicate_sequence',
        ).isNotEmpty;
    final badge = _paymentHistoryBadge(status, isDuplicate: isDuplicate);

    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Container(
          width: 8.w,
          height: 8.w,
          margin: EdgeInsets.only(top: 8.h),
          decoration: BoxDecoration(color: badge.color, shape: BoxShape.circle),
        ),
        SizedBox(width: 10.w),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Expanded(
                    child: Text(
                      CodeMetaDisplay.paymentMethod(
                        paymentType,
                        fallback: paymentType.isEmpty ? '付款方式未指定' : paymentType,
                      ),
                      style: TextStyle(
                        fontSize: 16.sp,
                        fontWeight: FontWeight.w700,
                        color: AppColors.textPrimary,
                        height: 1.5,
                      ),
                    ),
                  ),
                  Container(
                    padding: EdgeInsets.symmetric(
                      horizontal: 8.w,
                      vertical: 4.h,
                    ),
                    decoration: BoxDecoration(
                      color: badge.color,
                      borderRadius: BorderRadius.circular(16.r),
                    ),
                    child: Text(
                      badge.label,
                      style: TextStyle(
                        fontSize: 12.sp,
                        fontWeight: FontWeight.w700,
                        color: badge.color.computeLuminance() > 0.45
                            ? AppColors.textEmphasis
                            : AppColors.bgCard,
                      ),
                    ),
                  ),
                ],
              ),
              SizedBox(height: 4.h),
              if (tradeNo.isNotEmpty)
                Text(
                  '交易編號 $tradeNo',
                  style: TextStyle(
                    fontSize: 14.sp,
                    color: AppColors.textPrimary,
                    height: 1.5,
                  ),
                ),
              Text(
                [
                  if (amount.isNotEmpty) '金額 NT\$ $amount',
                  if (payTime.isNotEmpty) '付款 ${_formatDateTime(payTime)}',
                  if (payTime.isEmpty && createdAt.isNotEmpty)
                    '建立 ${_formatDateTime(createdAt)}',
                ].join('｜'),
                style: TextStyle(
                  fontSize: 14.sp,
                  color: AppColors.textPrimary,
                  height: 1.5,
                ),
              ),
              if (isDuplicate) ...[
                SizedBox(height: 4.h),
                Text(
                  '此筆為重複付款紀錄，請以退款進度為準。',
                  style: TextStyle(
                    fontSize: 14.sp,
                    color: AppColors.warningText,
                    height: 1.5,
                  ),
                ),
              ],
            ],
          ),
        ),
      ],
    );
  }
}

String _historyValue(
  Map<String, dynamic> map,
  String key, [
  String? alt1,
  String? alt2,
]) {
  final value =
      map[key] ??
      (alt1 == null ? null : map[alt1]) ??
      (alt2 == null ? null : map[alt2]);
  final text = value?.toString().trim() ?? '';
  return text == 'null' ? '' : text;
}

({String label, Color color}) _paymentHistoryBadge(
  String status, {
  required bool isDuplicate,
}) {
  if (isDuplicate) {
    return (label: '重複付款', color: AppColors.warning);
  }
  switch (status.toUpperCase()) {
    case '0':
    case 'INIT':
      return (label: '待付款', color: AppColors.warning);
    case '1':
    case 'PAID':
    case 'SUCCESS':
      return (label: '付款完成', color: AppColors.brandPrimaryStrong);
    case '2':
    case 'FAILED':
      return (label: '付款失敗', color: AppColors.error);
    case '3':
    case 'EXPIRED':
      return (label: '已逾期', color: AppColors.textSecondary);
    case '4':
    case 'CONFLICT':
      return (label: '狀態衝突', color: AppColors.error);
    case '5':
    case 'CLOSED':
      return (label: '已關閉', color: AppColors.textSecondary);
    case '6':
    case 'CLOSED_BY_SWITCH':
      return (label: '已切換', color: AppColors.textSecondary);
    default:
      return (
        label: status.isEmpty ? '未知' : status,
        color: AppColors.textSecondary,
      );
  }
}

class _ProductRow extends StatelessWidget {
  const _ProductRow({required this.name, required this.amount});

  final String name;
  final int amount;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Container(
          width: 80.w,
          height: 80.h,
          decoration: BoxDecoration(
            color: AppColors.gray100,
            borderRadius: BorderRadius.circular(12.r),
          ),
          child: Icon(
            AppIcons.imageOutlined,
            size: 32.sp,
            color: AppIconColors.disabled,
          ),
        ),
        SizedBox(width: 12.w),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                name,
                style: TextStyle(
                  fontSize: 16.sp,
                  color: AppColors.textPrimary,
                  height: 1.5,
                ),
              ),
              SizedBox(height: 6.h),
              Text(
                '數量：1',
                style: TextStyle(
                  fontSize: 16.sp,
                  color: AppColors.textPrimary,
                  height: 1.5,
                ),
              ),
            ],
          ),
        ),
        Text(
          'NT\$ ${_fmt(amount)}',
          style: TextStyle(
            fontSize: 20.sp,
            fontWeight: FontWeight.w700,
            color: AppColors.brandPrimaryStrong,
          ),
        ),
      ],
    );
  }
}

class _ProductRowRich extends StatelessWidget {
  const _ProductRowRich({required this.item});

  final OrderItem item;

  @override
  Widget build(BuildContext context) {
    // PayUni #758: 顯示 effectiveUnitPrice，有 sale 時旁邊加 listPrice 刪除線
    final unitDisp = item.effectiveUnitPrice;
    final listDisp = item.effectiveListPrice;
    final showStrike = item.shouldStrikeListPrice;
    final discount = item.shareDiscountValue;
    final actualAmount = item.actualPayAmount.trim().isNotEmpty
        ? item.actualPayAmount.trim()
        : _itemLineTotal(item);
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        ClipRRect(
          borderRadius: BorderRadius.circular(12.r),
          child: item.imageKey.isNotEmpty
              ? OptimizedNetworkImage(
                  // 傳 objectKey：[OptimizedNetworkImage] 內部走
                  // `ImageConfig.urlFor` → `buildImageUrl` 這個唯一組裝入口。
                  imageUrl: item.imageKey,
                  scene: ImageScene.orderItem,
                  width: 80.w,
                  height: 80.h,
                  fit: BoxFit.cover,
                  errorWidget: _imgPlaceholder(),
                )
              : _imgPlaceholder(),
        ),
        SizedBox(width: 12.w),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                item.name.isNotEmpty ? item.name : '商品',
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
                style: AppTypography.bodyBold,
              ),
              if (item.skuId.trim().isNotEmpty) ...[
                SizedBox(height: 2.h),
                Text('規格編號 ${item.skuId}', style: AppTypography.metaMuted),
              ],
              SizedBox(height: 6.h),
              Text(
                '單價 NT\$ $unitDisp × ${item.quantity}',
                style: AppTypography.metaMuted,
              ),
              if (discount > 0) ...[
                SizedBox(height: 4.h),
                Text(
                  '品項折抵 -NT\$${discount.toInt()}',
                  style: TextStyle(fontSize: 12.sp, color: AppColors.warning),
                ),
              ],
            ],
          ),
        ),
        Column(
          crossAxisAlignment: CrossAxisAlignment.end,
          children: [
            Text(
              'NT\$ $actualAmount',
              style: AppTypography.priceTotal.copyWith(
                color: AppColors.brandPrimaryStrong,
              ),
            ),
            if (showStrike) ...[
              SizedBox(height: 2.h),
              Text(
                'NT\$ $listDisp',
                style: TextStyle(
                  fontSize: 12.sp,
                  color: AppColors.textSecondary,
                  decoration: TextDecoration.lineThrough,
                  decorationColor: AppColors.textSecondary,
                ),
              ),
            ],
          ],
        ),
      ],
    );
  }

  Widget _imgPlaceholder() => Container(
    width: 80.w,
    height: 80.h,
    decoration: BoxDecoration(
      color: AppColors.gray100,
      borderRadius: BorderRadius.circular(12.r),
    ),
    child: Icon(
      AppIcons.imageOutlined,
      size: 32.sp,
      color: AppIconColors.disabled,
    ),
  );
}

String _itemLineTotal(OrderItem item) {
  final unit = num.tryParse(item.effectiveUnitPrice.replaceAll(',', '')) ?? 0;
  final total = unit * item.quantity;
  if (total == total.roundToDouble()) return total.toInt().toString();
  return total.toStringAsFixed(2);
}

// ─────────────────────────────────────────────────────────────────
// 商品評分區塊（買家評價這筆訂單後顯示）
//
// 資料來源優先順序：
// 1. caller 傳入 fallbackRating > 0 → 用 detail API 自帶（B-145 之後生效）
// 2. fallback：呼叫 OrderRepository.findReviewByOrderItemIds 反查
//    /app/sellers/me/reviews，依 order_item_id 比對找出對應 review
// ─────────────────────────────────────────────────────────────────

class _OrderRatingCard extends ConsumerStatefulWidget {
  const _OrderRatingCard({
    required this.fallbackRating,
    required this.fallbackContent,
    required this.orderItemIds,
  });

  /// detail API 自帶的 rating（B-145 後端補欄位後 > 0；目前現況通常 0）
  final int fallbackRating;

  /// detail API 自帶的 review content
  final String fallbackContent;

  /// 該訂單的所有 OrderItem id（給 review 反查比對用）
  final Set<String> orderItemIds;

  @override
  ConsumerState<_OrderRatingCard> createState() => _OrderRatingCardState();
}

class _OrderRatingCardState extends ConsumerState<_OrderRatingCard> {
  late final Future<({int rating, String content, List<String> images})?>
  _reviewFuture;

  @override
  void initState() {
    super.initState();
    _reviewFuture = _loadReview();
  }

  Future<({int rating, String content, List<String> images})?>
  _loadReview() async {
    // 優先 detail API 自帶
    if (widget.fallbackRating > 0) {
      return (
        rating: widget.fallbackRating,
        content: widget.fallbackContent,
        images: const <String>[],
      );
    }
    // Fallback：反查 sellers/me/reviews
    try {
      if (widget.orderItemIds.isEmpty) return null;
      final review = await ref
          .read(orderRepositoryProvider)
          .findReviewByOrderItemIds(widget.orderItemIds);
      if (review == null) return null;
      final r = review['rating'];
      int rating = 0;
      if (r is int) {
        rating = r.clamp(0, 5);
      } else if (r is num) {
        rating = r.round().clamp(0, 5);
      } else if (r is String) {
        final n = num.tryParse(r);
        if (n != null) rating = n.round().clamp(0, 5);
      }
      final content = (review['content'] ?? '').toString().trim();
      final imagesRaw = review['imageUrls'];
      final images = <String>[];
      if (imagesRaw is List) {
        for (final item in imagesRaw) {
          final url = item?.toString().trim() ?? '';
          if (url.isNotEmpty) images.add(url);
        }
      }
      return (rating: rating, content: content, images: images);
    } catch (_) {
      return null;
    }
  }

  @override
  Widget build(BuildContext context) {
    return FutureBuilder<({int rating, String content, List<String> images})?>(
      future: _reviewFuture,
      builder: (context, snapshot) {
        if (snapshot.connectionState != ConnectionState.done) {
          return const SizedBox.shrink();
        }
        final data = snapshot.data;
        if (data == null || data.rating <= 0) {
          return const SizedBox.shrink();
        }
        return Padding(
          padding: EdgeInsets.only(top: 16.h),
          child: _DetailCard(
            children: [
              Row(
                children: [
                  Icon(
                    AppIcons.starRateRounded,
                    size: 24.w,
                    color: AppIconColors.rating,
                  ),
                  SizedBox(width: 6.w),
                  Text(
                    '商品評價',
                    style: TextStyle(
                      fontSize: 18.sp,
                      fontWeight: FontWeight.w700,
                      color: AppColors.textPrimary,
                      height: 1.23,
                    ),
                  ),
                ],
              ),
              SizedBox(height: 12.h),
              Row(
                children: List.generate(5, (i) {
                  final filled = i < data.rating;
                  return Padding(
                    padding: EdgeInsets.only(right: 4.w),
                    child: Icon(
                      filled
                          ? AppIcons.starRounded
                          : AppIcons.starOutlineRounded,
                      size: 28.w,
                      color: filled
                          ? AppIconColors.rating
                          : AppIconColors.defaultColor.withValues(alpha: 0.3),
                    ),
                  );
                }),
              ),
              if (data.content.isNotEmpty) ...[
                SizedBox(height: 12.h),
                Text(
                  data.content,
                  style: TextStyle(
                    fontSize: 14.sp,
                    color: AppColors.textPrimary,
                    height: 1.5,
                  ),
                ),
              ],
              if (data.images.isNotEmpty) ...[
                SizedBox(height: 12.h),
                SizedBox(
                  height: 80.w,
                  child: ListView.separated(
                    scrollDirection: Axis.horizontal,
                    itemCount: data.images.length > 4 ? 4 : data.images.length,
                    separatorBuilder: (_, __) => SizedBox(width: 8.w),
                    itemBuilder: (context, index) {
                      final url = data.images[index];
                      return ClipRRect(
                        borderRadius: BorderRadius.circular(12.r),
                        child: OptimizedNetworkImage(
                          imageUrl: url,
                          scene: ImageScene.reviewPhoto,
                          width: 80.w,
                          height: 80.w,
                          fit: BoxFit.cover,
                          errorWidget: Container(
                            width: 80.w,
                            height: 80.w,
                            color: AppColors.gray100,
                          ),
                        ),
                      );
                    },
                  ),
                ),
              ],
            ],
          ),
        );
      },
    );
  }
}
