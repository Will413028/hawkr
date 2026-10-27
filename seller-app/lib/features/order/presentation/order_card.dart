import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../../core/analytics/analytics_providers.dart';
import '../../../core/codemeta/code_meta_display.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_typography.dart';
import '../../../shared/utils/floating_toast.dart';
import '../../../shared/widgets/fruit_spinner.dart';
import '../../../shared/widgets/hk_icon.dart';
import '../../shipment_anomaly/data/shipment_anomaly_providers.dart';
import '../data/order_model.dart';
import '../data/order_repository.dart';
import '../data/seller_after_sale_lookup.dart';
import 'order_card_widgets/order_card_widgets.dart';
import 'pdf_preview_screen.dart';
import 'order_status_presentation.dart';

/// 訂單列表卡片（可展開）。
///
/// 自己管理 isExpanded 狀態；isLoading / onSetLoading 由外部傳入（
/// list screen 統一追蹤哪些訂單正在操作中）。
class OrderCard extends ConsumerStatefulWidget {
  const OrderCard({
    super.key,
    required this.order,
    required this.isPending,
    required this.isLoading,
    required this.onSetLoading,
    required this.onReload,
    required this.onReloadAndSwitchTab,
  });

  final OrderSummary order;
  final bool isPending;
  final bool isLoading;
  final void Function(String orderId, bool loading) onSetLoading;
  final void Function() onReload;
  final void Function(int tabIndex) onReloadAndSwitchTab;

  @override
  ConsumerState<OrderCard> createState() => _OrderCardState();
}

class _OrderCardState extends ConsumerState<OrderCard> {
  bool _isExpanded = false;

  String get _orderId =>
      widget.order.orderSn.isNotEmpty ? widget.order.orderSn : widget.order.id;

  bool get _isPendingShipment => widget.isPending;

  void _toggleExpanded() => setState(() => _isExpanded = !_isExpanded);

  void _showCancelShipmentDialog() {
    showDialog<void>(
      context: context,
      barrierColor: AppColors.textPrimary.withValues(alpha: 0.8),
      builder: (ctx) => CancelShipmentDialog(
        orderNo: _orderId,
        onConfirm: (reason) async {
          Navigator.pop(ctx);
          widget.onSetLoading(_orderId, true);
          try {
            final result = await ref
                .read(orderRepositoryProvider)
                .cancelOrder(_orderId, reason: reason);
            if (!mounted) return;
            widget.onSetLoading(_orderId, false);
            unawaited(
              ref
                  .read(analyticsEventsProvider)
                  .cancelOrder(_orderId, cancelReason: reason),
            );
            await showDialog<void>(
              context: context,
              barrierDismissible: true,
              builder: (_) => CancelOrderSuccessDialog(result: result),
            );
            if (mounted) widget.onReload();
          } catch (e) {
            if (!mounted) return;
            widget.onSetLoading(_orderId, false);
            FloatingToast.error(
              context,
              e.toString().replaceFirst('Exception: ', ''),
            );
          }
        },
      ),
    );
  }

  Future<void> _downloadPdf() async {
    final labelUrl = widget.order.labelUrl;
    if (labelUrl.isEmpty) {
      FloatingToast.error(context, '無法取得託運單 PDF');
      return;
    }
    // 開啟內嵌 PDF 預覽頁（iOS 的 WKWebView 直接渲染）。
    await Navigator.of(context).push(
      MaterialPageRoute(
        builder: (_) => PdfPreviewScreen(pdfUrl: labelUrl),
        fullscreenDialog: true,
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Stack(
      clipBehavior: Clip.none,
      children: [
        // ── margin-top 8h 为 Flag banner 留空间 ────────────────────────────
        Container(
          margin: EdgeInsets.only(top: 8.h),
          child: Stack(
            clipBehavior: Clip.none,
            children: [
              // ── 拖拉欄位：高 25h, bottom:0 對齊內層 Stack 底
              // 上 13h 被主卡片覆蓋（z-order 靠前），僅下 12h 露出呈現「抽屜把手」
              Positioned(
                left: 0,
                right: 0,
                bottom: 0,
                height: 25.h,
                child: _buildDragHandle(),
              ),
              // ── 主卡片：padding-bottom 12h 撐出空間讓拖拉欄位露出 ─────────
              // 外層 GestureDetector：點擊卡片空白區也能展開／收起。
              // 內部具名按鈕（細節 / 售後處理 / 單號複製 / info_outline 等）
              // 本身是 GestureDetector，依 Flutter 手勢競爭機制自動贏得各自區域。
              Padding(
                padding: EdgeInsets.only(bottom: 12.h),
                child: Semantics(
                  key: ValueKey('order-card-expand-$_orderId'),
                  container: true,
                  explicitChildNodes: true,
                  button: true,
                  expanded: _isExpanded,
                  label: _isExpanded ? '收合訂單 $_orderId' : '展開訂單 $_orderId',
                  onTap: _toggleExpanded,
                  child: GestureDetector(
                    onTap: _toggleExpanded,
                    excludeFromSemantics: true,
                    behavior: HitTestBehavior.opaque,
                    child: Container(
                      decoration: ShapeDecoration(
                        color: AppColors.bgCard,
                        shape: RoundedRectangleBorder(
                          // 對齊 buyer：用預設 strokeAlignInside，避免圓角邊框
                          // 在 Clip.antiAlias 下被裁切不連續。
                          side: BorderSide(
                            width: 1,
                            color: AppColors.cardBorder,
                          ),
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

                          // ── 買家與收件摘要 ──
                          Padding(
                            padding: EdgeInsets.fromLTRB(16.w, 12.h, 16.w, 0),
                            child: Row(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                OrderAvatarColumn(order: widget.order),
                                SizedBox(width: 8.w),
                                Expanded(
                                  child: OrderInfoColumn(order: widget.order),
                                ),
                              ],
                            ),
                          ),

                          Padding(
                            padding: EdgeInsets.fromLTRB(16.w, 12.h, 16.w, 0),
                            child: _buildProductSummary(),
                          ),
                          Padding(
                            padding: EdgeInsets.fromLTRB(16.w, 8.h, 16.w, 0),
                            child: _buildDeliverySummary(),
                          ),
                          Padding(
                            padding: EdgeInsets.fromLTRB(16.w, 8.h, 16.w, 0),
                            child: const Divider(
                              height: 1,
                              color: AppColors.borderSubtle,
                            ),
                          ),

                          // ── 下區：單號（靠左）+ 按鈕群（靠右） ──
                          Padding(
                            padding: EdgeInsets.fromLTRB(16.w, 4.h, 16.w, 8.h),
                            child: OrderBottomRow(
                              order: widget.order,
                              isPending: _isPendingShipment,
                              onSetLoading: widget.onSetLoading,
                              onReload: widget.onReload,
                              onReloadAndSwitchTab: widget.onReloadAndSwitchTab,
                            ),
                          ),

                          // ── 展開區：有售後 case 時優先顯示售後進度 ──
                          ClipRect(
                            child: AnimatedAlign(
                              duration: const Duration(milliseconds: 250),
                              curve: Curves.easeInOut,
                              alignment: Alignment.topCenter,
                              heightFactor: _isExpanded ? 1.0 : 0.0,
                              child: _buildExpandedContent(),
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),
                ),
              ),
            ],
          ),
        ),
        // ── 配送異常紅角徽章（top-left） ─────────────────────────────────
        // 該訂單對應 shipment 有未 ack 的 anomaly 時顯示。
        Positioned(
          top: 0,
          left: 16.w,
          child: _ShipmentAnomalyBadge(orderNo: _orderId),
        ),
        // ── Per-card loading overlay ─────────────────────────────────────
        if (widget.isLoading)
          Positioned.fill(
            top: 8.h,
            child: Container(
              decoration: BoxDecoration(
                color: AppColors.bgCard.withValues(alpha: 0.7),
                borderRadius: BorderRadius.circular(16.r),
              ),
              child: Center(child: FruitSpinner(size: 40.w)),
            ),
          ),
      ],
    );
  }

  Widget _buildStatusHeader() {
    final visual = OrderStatusPresentation.fromOrder(widget.order);
    return ColoredBox(
      color: visual.softColor,
      child: Padding(
        padding: EdgeInsets.fromLTRB(16.w, 12.h, 16.w, 12.h),
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
            SizedBox(width: 12.w),
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
                  SizedBox(height: 4.h),
                  Text(
                    visual.description,
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                    style: AppTypography.metaMuted,
                  ),
                ],
              ),
            ),
            SizedBox(width: 8.w),
            AnimatedRotation(
              turns: _isExpanded ? 0.5 : 0,
              duration: const Duration(milliseconds: 250),
              child: HkIcon(
                HkIconName.chevronDown,
                size: 20.sp,
                color: visual.color,
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildProductSummary() {
    final first = widget.order.items.isEmpty ? null : widget.order.items.first;
    final count = widget.order.items.isEmpty ? 1 : widget.order.items.length;
    final amount = 'NT\$ ${widget.order.totalAmount}';
    final amountStyle = AppTypography.priceTotal.copyWith(
      color: AppColors.brandPrimaryStrong,
    );
    final price = Text(
      amount,
      key: ValueKey('order-card-amount-$_orderId'),
      style: amountStyle,
      textAlign: TextAlign.end,
    );
    return LayoutBuilder(
      builder: (context, constraints) {
        final painter = TextPainter(
          text: TextSpan(text: amount, style: amountStyle),
          textDirection: Directionality.of(context),
          textScaler: MediaQuery.textScalerOf(context),
        )..layout();
        // Reserve a useful product-name column; a large amount gets a separate
        // full-width line rather than shrinking or truncating financial data.
        final stackPrice =
            painter.width + 40.w + 12.w + 8.w + 112.w > constraints.maxWidth;
        painter.dispose();
        return Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Row(
              children: [
                Container(
                  width: 40.w,
                  height: 40.w,
                  alignment: Alignment.center,
                  decoration: const BoxDecoration(
                    color: AppColors.primarySoft,
                    shape: BoxShape.circle,
                  ),
                  child: HkIcon(
                    HkIconName.goods,
                    size: 22.sp,
                    color: AppIconColors.active,
                  ),
                ),
                SizedBox(width: 12.w),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        first?.name.trim().isNotEmpty == true
                            ? first!.name
                            : widget.order.productName.trim().isNotEmpty
                            ? widget.order.productName
                            : '訂單商品',
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                        style: AppTypography.bodyBold,
                      ),
                      Text('共 $count 件', style: AppTypography.metaMuted),
                    ],
                  ),
                ),
                if (!stackPrice) ...[SizedBox(width: 8.w), price],
              ],
            ),
            if (stackPrice)
              Padding(
                padding: EdgeInsets.only(top: 4.h),
                child: price,
              ),
          ],
        );
      },
    );
  }

  Widget _buildDeliverySummary() {
    final delivery = DeliveryStatusPresentation.fromOrder(widget.order);
    final method = CodeMetaDisplay.shippingMethod(
      widget.order.shippingMethod,
      fallback: '配送方式未指定',
    );
    return Container(
      width: double.infinity,
      padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 8.h),
      decoration: BoxDecoration(
        color: delivery.softColor,
        borderRadius: BorderRadius.circular(12.r),
      ),
      child: Row(
        children: [
          HkIcon(delivery.icon, size: 20.sp, color: delivery.color),
          SizedBox(width: 8.w),
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
          Text(
            '${delivery.progressStep + 1}/4',
            style: AppTypography.caption.copyWith(color: delivery.color),
          ),
        ],
      ),
    );
  }

  /// 展開區內容分派：
  /// - 該訂單有 active 售後 case → 渲染 [AfterSaleExpandedSection]（售後進度）
  /// - 否則 → 渲染 [OrderExpandedSection]（正向物流進度）
  Widget _buildExpandedContent() {
    final lookup = ref.watch(sellerAfterSaleLookupProvider);
    final afterSale = lookup.value?[_orderId];
    if (afterSale != null) {
      return AfterSaleExpandedSection(summary: afterSale);
    }
    return OrderExpandedSection(
      order: widget.order,
      onCancelShipment: _showCancelShipmentDialog,
      onDownloadPdf: _downloadPdf,
    );
  }

  /// 拖拉欄位 — 總高 25h（上半 13h 藏於主卡片背後，下半 12h 露出於卡片下緣）
  /// 圓角 radius 15 配合總高 25 才能完整呈現；下方阴影 blur:4 offset(0,4) 強化"抽屜"感
  Widget _buildDragHandle() {
    return GestureDetector(
      onTap: _toggleExpanded,
      // The complete card already exposes the same expand/collapse action as a
      // large semantics target. Avoid advertising this 25dp visual affordance
      // as a second, undersized accessibility control.
      excludeFromSemantics: true,
      behavior: HitTestBehavior.opaque,
      child: Container(
        padding: EdgeInsets.only(top: 16.h),
        decoration: ShapeDecoration(
          color: AppColors.bgApp,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.only(
              bottomLeft: Radius.circular(16.r),
              bottomRight: Radius.circular(16.r),
            ),
          ),
          shadows: const [
            BoxShadow(
              color: AppColors.cardShadow,
              blurRadius: 4,
              offset: Offset(0, 4),
            ),
          ],
        ),
        child: Center(
          child: AnimatedRotation(
            turns: _isExpanded ? 0.5 : 0.0,
            duration: const Duration(milliseconds: 250),
            curve: Curves.easeInOut,
            child: HkIcon(
              HkIconName.chevronDown,
              size: 12.w,
              color: AppIconColors.disabled,
            ),
          ),
        ),
      ),
    );
  }
}

/// 配送異常紅角徽章 — 該訂單對應 shipment 有未 ack 的 anomaly 時顯示。
///
/// 沿用既有「紅色 dot + drop shadow」徽章 pattern（與 dashboard chat unread 風格一致）。
class _ShipmentAnomalyBadge extends ConsumerWidget {
  const _ShipmentAnomalyBadge({required this.orderNo});

  final String orderNo;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final map = ref.watch(shipmentAnomalyByOrderNoProvider).value;
    final hasAnomaly = map != null && map.containsKey(orderNo);
    if (!hasAnomaly) return const SizedBox.shrink();

    return Container(
      width: 32.w,
      height: 34.w,
      decoration: BoxDecoration(
        color: AppColors.error,
        borderRadius: BorderRadius.only(
          bottomLeft: Radius.circular(24.r),
          bottomRight: Radius.circular(24.r),
        ),
        boxShadow: [
          BoxShadow(
            color: AppColors.cardShadow,
            blurRadius: 1.5,
            offset: const Offset(0, 1),
          ),
        ],
      ),
      child: Align(
        alignment: const Alignment(0, -0.1),
        child: HkIcon(
          HkIconName.warning,
          size: 18.w,
          color: AppIconColors.onBrand,
        ),
      ),
    );
  }
}
