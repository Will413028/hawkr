import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:hawkr_buyer/core/codemeta/code_meta.dart'
    hide OrderStatus, OrderItemStatus;

import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_spacing.dart';
import '../../../core/theme/app_typography.dart';
import '../../../config/image_config.dart';
import '../../../shared/icons/app_icon_colors.dart';
import '../../../shared/widgets/hk_icon.dart';
import '../../../shared/widgets/common_snackbar.dart';
import '../../../shared/widgets/hk_tap_target.dart';
import '../../../shared/widgets/oss_image.dart';
import '../../../shared/widgets/product_card_hero.dart';
import '../../member_card/presentation/member_card_controller.dart';
import '../../seller/data/store_card_display.dart';
import '../../seller/data/store_card_provider.dart';
import '../data/after_sale_order_lookup.dart';
import '../data/order_model.dart';
import 'order_detail_controller.dart';
import '../data/order_repository.dart';
import 'confirm_receipt_dialog.dart';
import 'order_card_widgets/order_card_widgets.dart';
import 'order_list_controller.dart';
import 'pay_order.dart';
import 'order_detail_transition.dart';
import 'order_status_presentation.dart';
import 'receipt_photo_controller.dart';

/// 訂單列表卡片。
///
/// 與新版詳情頁共用狀態、色彩與圖示語意：狀態摘要 → 店家／收件資訊 →
/// 商品摘要 → 配送貨態 → 訂單編號與操作。展開後保留完整物流或售後流程。
class OrderCard extends ConsumerStatefulWidget {
  const OrderCard({super.key, required this.order});
  final Order order;

  @override
  ConsumerState<OrderCard> createState() => _OrderCardState();
}

class _OrderCardState extends ConsumerState<OrderCard> {
  bool _expanded = false;
  bool _isPayLoading = false;
  final bool _isConfirmLoading = false;
  Order? _detailOrder;
  Timer? _paymentDeadlineTicker;

  Order get order => widget.order;
  bool get _isPendingPayment => order.status == '0';
  bool get _isDelivered => order.status == '40';
  bool get _canExpand => !_isPendingPayment;
  bool get _needsProductImage =>
      order.orderSn.isNotEmpty &&
      order.items.isNotEmpty &&
      order.items.first.imageUrl.trim().isEmpty;

  @override
  void initState() {
    super.initState();
    if (_isPendingPayment || _needsProductImage) {
      _fetchOrderDetail();
    }
    if (_isPendingPayment) {
      _paymentDeadlineTicker = Timer.periodic(const Duration(seconds: 30), (_) {
        if (!mounted) return;
        setState(() {});
      });
    }
  }

  @override
  void dispose() {
    _paymentDeadlineTicker?.cancel();
    super.dispose();
  }

  void _toggleExpand() {
    if (!_canExpand) return;
    setState(() => _expanded = !_expanded);
  }


  @override
  Widget build(BuildContext context) {
    final cardBorderSide = BorderSide(width: 1, color: AppColors.cardBorder);
    final storeCard = ref.watch(storeCardProvider(order.storeId)).value;
    final storeCode = storeCard != null && storeCard.isOpen
        ? storeCard.storeCode
        : '';

    final VoidCallback cardOnTap = _canExpand
        ? _toggleExpand
        : () => context.push(
            '/orders/detail',
            extra: {
              'orderSn': order.orderSn,
              'heroTag': orderDetailHeroTag(order.orderSn),
            },
          );
    final cardActionLabel = _canExpand
        ? '${_expanded ? '收合' : '展開'}訂單 ${order.orderSn}'
        : '查看訂單 ${order.orderSn}';
    final card = Padding(
      padding: EdgeInsets.only(top: 8.h),
      child: Semantics(
        container: true,
        explicitChildNodes: true,
        button: true,
        label: cardActionLabel,
        hint: _canExpand ? '切換訂單摘要內容' : '開啟訂單詳情',
        expanded: _canExpand ? _expanded : null,
        onTap: cardOnTap,
        child: GestureDetector(
          excludeFromSemantics: true,
          onTap: cardOnTap,
          behavior: HitTestBehavior.opaque,
          child: Container(
            key: const ValueKey('order-card-collapsed'),
            decoration: ShapeDecoration(
              color: AppColors.bgCard,
              shape: RoundedRectangleBorder(
                side: cardBorderSide,
                borderRadius: BorderRadius.circular(16.r),
              ),
              shadows: const [
                BoxShadow(
                  color: AppColors.cardShadow,
                  blurRadius: 2,
                  offset: Offset(0, 1),
                ),
              ],
            ),
            clipBehavior: Clip.antiAlias,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                _buildStatusHeader(),

                Padding(
                  padding: EdgeInsets.fromLTRB(
                    AppSpacing.lg,
                    AppSpacing.md,
                    AppSpacing.lg,
                    0,
                  ),
                  child: Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      HkTapTarget(
                        semanticLabel: '查看商家 ${order.sellerName}',
                        hint: '開啟商家頁面',
                        tooltip: '查看商家',
                        onTap: storeCode.isEmpty
                            ? null
                            : () => context.push('/merchant/$storeCode'),
                        child: OrderSellerColumn(order: order),
                      ),
                      SizedBox(width: AppSpacing.sm),
                      Expanded(
                        child: OrderInfoColumn(
                          order: _isPendingPayment
                              ? (_detailOrder ?? order)
                              : order,
                          isPendingPayment: _isPendingPayment,
                        ),
                      ),
                    ],
                  ),
                ),

                Padding(
                  padding: EdgeInsets.fromLTRB(
                    AppSpacing.lg,
                    AppSpacing.md,
                    AppSpacing.lg,
                    0,
                  ),
                  child: _buildProductSummary(),
                ),
                Padding(
                  padding: EdgeInsets.fromLTRB(
                    AppSpacing.lg,
                    AppSpacing.sm,
                    AppSpacing.lg,
                    AppSpacing.lg,
                  ),
                  child: _buildDeliverySummary(),
                ),

                if (!_canExpand) _buildOrderBottomSection(context),
                if (_canExpand)
                  AnimatedSize(
                    duration: const Duration(milliseconds: 250),
                    curve: Curves.easeOutCubic,
                    alignment: Alignment.topCenter,
                    child: _expanded
                        ? Column(
                            key: const ValueKey('order-card-expanded-content'),
                            children: [
                              _buildExpandedContent(),
                              _buildOrderBottomSection(context),
                            ],
                          )
                        : const SizedBox.shrink(),
                  ),
              ],
            ),
          ),
        ),
      ),
    );
    final heroTag = orderDetailHeroTag(order.orderSn);
    if (heroTag == null) return card;
    return Hero(
      tag: heroTag,
      transitionOnUserGestures: true,
      createRectTween: (begin, end) =>
          MaterialRectArcTween(begin: begin, end: end),
      flightShuttleBuilder: productCardHeroFlightShuttleBuilder,
      child: card,
    );
  }

  Widget _buildOrderBottomSection(BuildContext context) {
    return Column(
      children: [
        Padding(
          padding: EdgeInsets.symmetric(horizontal: AppSpacing.lg),
          child: const Divider(height: 1, color: AppColors.borderSubtle),
        ),
        Padding(
          padding: EdgeInsets.fromLTRB(
            AppSpacing.lg,
            AppSpacing.md,
            AppSpacing.lg,
            AppSpacing.lg,
          ),
          child: OrderBottomRow(
            order: order,
            actionButtons: OrderActionButtons(
              order: order,
              isPendingPayment: _isPendingPayment,
              isDelivered: _isDelivered,
              isPayLoading: _isPayLoading,
              isConfirmLoading: _isConfirmLoading,
              onRetryPayment: () => _executePayment(context),
              onConfirmReceipt: () => _confirmReceipt(context),
              onReview: () => _handleReview(context),
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildStatusHeader() {
    final visual = OrderStatusPresentation.fromOrder(order);
    return ColoredBox(
      color: visual.softColor,
      child: Padding(
        padding: EdgeInsets.fromLTRB(
          AppSpacing.lg,
          AppSpacing.md,
          AppSpacing.lg,
          AppSpacing.md,
        ),
        child: Row(
          children: [
            Container(
              width: 40.w,
              height: 40.w,
              alignment: Alignment.center,
              decoration: BoxDecoration(
                color: AppColors.bgCard,
                shape: BoxShape.circle,
                border: Border.all(color: visual.color.withValues(alpha: 0.2)),
              ),
              child: HkIcon(visual.icon, size: 22.sp, color: visual.color),
            ),
            SizedBox(width: AppSpacing.md),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    visual.label,
                    style: AppTypography.productName.copyWith(
                      color: visual.color,
                    ),
                  ),
                  SizedBox(height: AppSpacing.xs),
                  Text(
                    visual.description,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: AppTypography.metaMuted,
                  ),
                ],
              ),
            ),
            if (_canExpand) ...[
              SizedBox(width: AppSpacing.sm),
              AnimatedRotation(
                turns: _expanded ? 0.5 : 0,
                duration: const Duration(milliseconds: 250),
                child: HkIcon(
                  HkIconName.chevronDown,
                  size: 20.sp,
                  color: visual.color,
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildProductSummary() {
    final listFirst = order.items.isEmpty ? null : order.items.first;
    final detailFirst = _detailOrder?.items.isEmpty ?? true
        ? null
        : _detailOrder!.items.first;
    final first = listFirst?.imageUrl.trim().isNotEmpty == true
        ? listFirst
        : (detailFirst?.imageUrl.trim().isNotEmpty == true
              ? detailFirst
              : listFirst ?? detailFirst);
    final itemCount = int.tryParse(order.itemCount) ?? order.items.length;
    final count = itemCount > 0 ? itemCount : order.items.length;
    final amount = order.totalActualAmount.trim();
    final productImageFallback = ColoredBox(
      color: AppColors.primarySoft,
      child: Center(
        child: HkIcon(
          HkIconName.goods,
          size: 22.sp,
          color: AppIconColors.active,
        ),
      ),
    );
    return Row(
      crossAxisAlignment: CrossAxisAlignment.center,
      children: [
        ClipOval(
          key: const ValueKey('order-card-product-image'),
          child: SizedBox(
            width: 44.w,
            height: 44.w,
            child: first?.imageUrl.trim().isNotEmpty == true
                ? OssImage(
                    imageSource: first!.imageUrl,
                    scene: ImageScene.orderItem,
                    width: 44.w,
                    height: 44.w,
                    fit: BoxFit.cover,
                    errorWidget: productImageFallback,
                  )
                : productImageFallback,
          ),
        ),
        SizedBox(width: AppSpacing.md),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                first?.productName.trim().isNotEmpty == true
                    ? first!.productName
                    : '訂單商品',
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: AppTypography.bodyBold,
              ),
              if (first?.skuName.trim().isNotEmpty == true || count > 0)
                Text(
                  [
                    if (first?.skuName.trim().isNotEmpty == true)
                      first!.skuName,
                    if (count > 0) '共 $count 件',
                  ].join(' · '),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: AppTypography.metaMuted,
                ),
            ],
          ),
        ),
        if (amount.isNotEmpty) ...[
          SizedBox(width: AppSpacing.sm),
          Text(
            'NT\$ $amount',
            style: AppTypography.priceTotal.copyWith(
              color: AppColors.brandPrimaryStrong,
              fontFeatures: const [FontFeature.tabularFigures()],
            ),
          ),
        ],
      ],
    );
  }

  Widget _buildDeliverySummary() {
    final delivery = DeliveryStatusPresentation.fromOrder(order);
    final method =
        ShippingMethod.fromValue(order.shippingMethod)?.label ??
        (order.shippingMethod.isEmpty ? '配送方式未指定' : order.shippingMethod);
    return Container(
      width: double.infinity,
      padding: EdgeInsets.symmetric(
        horizontal: AppSpacing.md,
        vertical: AppSpacing.sm,
      ),
      decoration: BoxDecoration(
        color: delivery.softColor,
        borderRadius: BorderRadius.circular(12.r),
      ),
      child: Row(
        children: [
          _buildDeliveryBrandIcon(delivery),
          SizedBox(width: AppSpacing.sm),
          Expanded(
            child: Text(
              '$method · ${delivery.label}',
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: AppTypography.meta.copyWith(
                color: delivery.color,
                fontWeight: FontWeight.w700,
              ),
            ),
          ),
          SizedBox(width: AppSpacing.sm),
          Text(
            '${delivery.progressStep + 1}/4',
            style: AppTypography.caption.copyWith(color: delivery.color),
          ),
        ],
      ),
    );
  }

  Widget _buildDeliveryBrandIcon(DeliveryStatusPresentation delivery) {
    return Container(
      width: 28.w,
      height: 28.w,
      padding: EdgeInsets.all(AppSpacing.xs),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        shape: BoxShape.circle,
        border: Border.all(color: delivery.color.withValues(alpha: 0.16)),
      ),
      child: HkIcon(HkIconName.truck, size: 18.sp, color: delivery.color),
    );
  }


  Widget _buildExpandedContent() {
    final snapshot = ref
        .watch(afterSaleOrderLookupProvider)
        .value?[order.orderSn];
    if (snapshot != null && _isDelivered) {
      return AfterSaleExpandedSection(snapshot: snapshot, order: order);
    }
    return OrderExpandedSection(order: order);
  }


  Future<void> _fetchOrderDetail() async {
    try {
      final detail = await ref
          .read(orderRepositoryProvider)
          .getOrderDetail(order.orderSn);
      if (mounted) setState(() => _detailOrder = detail);
    } catch (_) {}
  }

  Future<void> _executePayment(BuildContext context) async {
    if (_isPayLoading) return;
    setState(() => _isPayLoading = true);
    try {
      await payOrder(context, ref, order.orderSn);
      if (!mounted) return;
      await _fetchOrderDetail();
      ref.read(orderListControllerProvider.notifier).refresh();
    } finally {
      if (mounted) setState(() => _isPayLoading = false);
    }
  }

  Future<void> _confirmReceipt(BuildContext context) async {
    if (_isConfirmLoading) return;
    final result = await ConfirmReceiptDialog.show(
      context,
      orderSn: order.orderSn,
    );
    if (result == null || !context.mounted) return;

    final url = (result.photoUrl ?? '').trim();
    if (url.isNotEmpty) {
      await ref
          .read(
            receiptPhotoControllerProvider(
              orderSn: order.orderSn,
              backendUrls: const [],
            ).notifier,
          )
          .cachePreview(url);
    }
    if (!context.mounted) return;

    showFloatingToast(context, '已確認收貨', isSuccess: true);
    ref.read(orderListControllerProvider.notifier).refresh();
    ref.invalidate(orderDetailControllerProvider(order.orderSn));
    ref.invalidate(memberCardsControllerProvider);
  }

  Future<void> _handleReview(BuildContext context) async {
    final result = await context.push<dynamic>(
      '/review/order',
      extra: {
        'orderSn': order.orderSn,
        'items': order.items
            .map(
              (item) => {
                'id': item.id,
                'skuId': item.skuId,
                'skuName': item.skuName,
                'productName': item.productName,
                'actualPayAmount': item.actualPayAmount,
                'imageUrl': item.imageUrl,
              },
            )
            .toList(),
      },
    );
    if (result == true && mounted) {
      await ref.read(orderListControllerProvider.notifier).refresh();
    }
  }
}
