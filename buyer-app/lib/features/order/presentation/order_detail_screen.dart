import 'package:hawkr_buyer/core/l10n/ui_locale.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../shared/widgets/common_top_bar.dart';
import '../../../shared/widgets/common_snackbar.dart';
import '../../../shared/widgets/hk_empty_state.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../../shared/widgets/hk_tap_target.dart';
import '../../../shared/widgets/live_countdown_line.dart';

import 'package:hawkr_buyer/core/codemeta/code_meta.dart'
    hide OrderStatus, OrderItemStatus;
import 'package:hawkr_buyer/core/codemeta/shipping_presentation.dart';

import '../../seller/data/store_card_display.dart';
import '../../seller/data/store_card_provider.dart';
import 'order_detail_controller.dart';
import '../data/after_sale_model.dart';
import '../data/after_sale_repository.dart';
import '../data/after_sale_status_notifier.dart';
import '../data/order_model.dart';
import '../order_color_tokens.dart';
import '../../../core/analytics/analytics_providers.dart';
import '../../../core/analytics/purchase_dedup.dart';
import '../data/order_status_notifier.dart';
import 'package:hawkr_api/images.dart' as pb_image;
import 'package:hawkr_api/gen/hawkr/review/v1alpha1/review.pb.dart'
    as pb_review;
import '../../../shared/widgets/app_cached_image.dart';
import '../../review/data/review_repository.dart';
import '../../../config/image_config.dart';
import '../../../shared/widgets/oss_image.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../../../shared/widgets/product_card_hero.dart';
import 'confirm_receipt_dialog.dart';
import 'pay_order.dart';
import 'order_list_controller.dart';
import 'order_status_presentation.dart';
import 'widgets/order_logistics_section.dart';
import 'widgets/order_receipt_photos_section.dart';
import '../../cart/data/cart_repository.dart';
import '../../cart/data/cart_error_message.dart';
import '../../cart/presentation/cart_controller.dart';
import '../../../shared/time/taipei_display.dart';

import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hawkr_buyer/shared/widgets/hk_brand_loader.dart';

final _orderAfterSaleCaseProvider = FutureProvider.autoDispose
    .family<AfterSaleCase?, String>((ref, orderNo) async {
      if (orderNo.isEmpty) return null;
      final repository = ref.read(afterSaleRepositoryProvider);
      final result = await repository.getAfterSaleList(pageSize: 100);
      for (final c in result.records) {
        if (c.orderNo != orderNo) continue;
        if (c.caseNo.isEmpty) return c;
        try {
          return await repository.getAfterSaleDetail(c.caseNo);
        } catch (_) {
          return c;
        }
      }
      return null;
    });

class OrderDetailScreen extends HookConsumerWidget {
  const OrderDetailScreen({
    super.key,
    this.orderSn,
    this.fromCheckout = false,
    this.heroTag,
  });
  final String? orderSn;

  /// When true, the screen was opened directly after a successful payment.
  /// Used to fire the GA4 `purchase` event exactly once for this order.
  final bool fromCheckout;
  final Object? heroTag;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    Widget body;
    if (orderSn != null) {
      final orderAsync = ref.watch(orderDetailControllerProvider(orderSn!));

      ref.listen(orderStatusStreamProvider, (_, next) {
        next.whenData((_) {
          ref.read(orderDetailControllerProvider(orderSn!).notifier).refresh();
        });
      });

      ref.listen(afterSaleStatusStreamProvider, (_, next) {
        next.whenData((_) {
          ref.read(orderDetailControllerProvider(orderSn!).notifier).refresh();
          ref.invalidate(_orderAfterSaleCaseProvider(orderSn!));
        });
      });

      body = orderAsync.when(
        loading: () => const TruckLoadingOverlay(),
        error: (err, _) => Center(
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              HkIcon(
                HkIconName.receipt,
                size: 48.sp,
                color: AppIconColors.disabled,
              ),
              SizedBox(height: 12.h),
              Text('載入訂單失敗', style: AppTypography.metaMuted),
              SizedBox(height: 12.h),
              Semantics(
                button: true,
                label: '重新載入訂單',
                child: GestureDetector(
                  onTap: () => ref
                      .read(orderDetailControllerProvider(orderSn!).notifier)
                      .refresh(),
                  child: Container(
                    padding: EdgeInsets.symmetric(
                      horizontal: 20.w,
                      vertical: 8.h,
                    ),
                    decoration: BoxDecoration(
                      color: AppColors.primary,
                      borderRadius: BorderRadius.circular(999),
                    ),
                    child: Text(
                      '重試',
                      style: AppTypography.buttonSmall.copyWith(
                        color: AppColors.bgCard,
                      ),
                    ),
                  ),
                ),
              ),
            ],
          ),
        ),
        data: (order) =>
            _OrderDetailBody(order: order, fromCheckout: fromCheckout),
      );
    } else {
      body = const HkEmptyState(
        key: ValueKey('order-detail-empty-state'),
        iconKey: ValueKey('order-detail-empty-state-icon'),
        icon: HkIconName.emptyOrders,
        message: '無訂單資料',
      );
    }

    return _OrderFloatingDetailShell(
      child: MediaQuery.removePadding(
        context: context,
        removeTop: true,
        child: Builder(
          builder: (detailContext) => Scaffold(
            backgroundColor: AppColors.bgPrimary,
            appBar: buildCommonTitleAppBar(
              detailContext,
              title: '訂單細節',
              backIcon: AppIcons.close,
              onBack: () {
                if (Navigator.of(context).canPop()) {
                  context.pop();
                } else {
                  context.go('/?tab=3');
                }
              },
              actions: [
                if (orderSn != null)
                  Consumer(
                    builder: (context, ref, _) {
                      final orderAsync = ref.watch(
                        orderDetailControllerProvider(orderSn!),
                      );
                      final isDelivered = orderAsync.value?.status == '40';
                      final afterSaleCase = ref
                          .watch(_orderAfterSaleCaseProvider(orderSn!))
                          .value;
                      final hasOpenAfterSale =
                          afterSaleCase != null &&
                          afterSaleCase.status != AfterSaleStatus.cancelled &&
                          afterSaleCase.status !=
                              AfterSaleStatus.cancelledByConfirmReceipt;
                      if (!isDelivered || hasOpenAfterSale) {
                        return const SizedBox.shrink();
                      }
                      return GestureDetector(
                        behavior: HitTestBehavior.opaque,
                        onTap: () {
                          final order = orderAsync.value;
                          if (order == null) return;
                          context.push(
                            '/orders/after-sale',
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
                        },
                        child: SizedBox(
                          width: 24.w,
                          height: 24.w,
                          child: HkIcon(
                            HkIconName.afterSales,
                            size: 24.w,
                            color: AppIconColors.onBrand,
                          ),
                        ),
                      );
                    },
                  ),
              ],
            ),
            body: _OrderDetailHero(tag: heroTag, child: body),
            bottomNavigationBar: orderSn != null
                ? Consumer(
                    builder: (context, ref, _) {
                      final order = ref
                          .watch(orderDetailControllerProvider(orderSn!))
                          .value;
                      if (order == null) return const SizedBox.shrink();
                      return _OrderDetailBottomBar(order: order);
                    },
                  )
                : null,
          ),
        ),
      ),
    );
  }
}

class _OrderFloatingDetailShell extends StatelessWidget {
  const _OrderFloatingDetailShell({required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      key: const ValueKey('order-detail-floating-shell'),
      backgroundColor: Colors.transparent,
      body: SafeArea(
        minimum: const EdgeInsets.all(HkSpace.sm),
        child: DecoratedBox(
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(HkRadius.xxl),
            boxShadow: HkShadows.modal,
          ),
          child: ClipRRect(
            key: const ValueKey('order-detail-preview-window'),
            borderRadius: BorderRadius.circular(HkRadius.xxl),
            child: child,
          ),
        ),
      ),
    );
  }
}

class _OrderDetailHero extends StatelessWidget {
  const _OrderDetailHero({required this.tag, required this.child});

  final Object? tag;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    if (tag == null) return child;
    return Hero(
      tag: tag!,
      transitionOnUserGestures: true,
      createRectTween: (begin, end) =>
          MaterialRectArcTween(begin: begin, end: end),
      flightShuttleBuilder: productCardHeroFlightShuttleBuilder,
      child: Material(color: AppColors.bgPrimary, child: child),
    );
  }
}

class _OrderDetailBody extends ConsumerWidget {
  const _OrderDetailBody({required this.order, this.fromCheckout = false});
  final Order order;
  final bool fromCheckout;

  String get _remark =>
      order.orderRemark.isNotEmpty ? order.orderRemark : order.shippingNote;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    if (fromCheckout && order.orderSn.isNotEmpty) {
      if (markPurchaseTracked(order.orderSn)) {
        ref.read(analyticsEventsProvider).trackPurchase(order);
      }
    }

    final afterSaleAsync = ref.watch(
      _orderAfterSaleCaseProvider(order.orderSn),
    );
    return SafeArea(
      top: false,
      bottom: false,
      child: LayoutBuilder(
        builder: (context, constraints) {
          final gutter = HkBreakpoints.pageGutter(constraints.maxWidth);
          return Center(
            child: ConstrainedBox(
              constraints: const BoxConstraints(
                maxWidth: HkControl.bottomNavMaxWidth,
              ),
              child: ListView(
                key: const ValueKey('order-detail-content'),
                padding: EdgeInsets.fromLTRB(
                  gutter,
                  HkSpace.lg,
                  gutter,
                  HkSpace.section,
                ),
                children: [
                  _TransactionOverviewCard(order: order),
                  if (order.sellerName.isNotEmpty) ...[
                    const SizedBox(height: HkSpace.lg),
                    _SellerInfoCard(order: order),
                  ],
                  const SizedBox(height: HkSpace.lg),
                  _ItemsCard(order: order),
                  const SizedBox(height: HkSpace.lg),
                  _OrderInfoCard(order: order),
                  OrderLogisticsSection(order: order),
                  const SizedBox(height: HkSpace.lg),
                  _PaymentDetailsCard(order: order),
                  _RefundTaskSummaryCard(
                    order: order,
                    afterSaleAsync: afterSaleAsync,
                  ),
                  _AfterSaleCaseSection(afterSaleAsync: afterSaleAsync),
                  if (_remark.isNotEmpty &&
                      order.status != '80' &&
                      order.status != '60' &&
                      order.status != '70') ...[
                    const SizedBox(height: HkSpace.lg),
                    _NoteCard(remark: _remark),
                  ],
                  OrderReceiptPhotosSection(order: order),
                  if (order.isReviewed) _OrderReviewCard(order: order),
                ],
              ),
            ),
          );
        },
      ),
    );
  }
}

/// 訂單詳情頁底部 sticky CTA bar — 依 status 切換主要動作。
/// status=0 → 立即付款 (re-open retry payment sheet)
/// status=40 → 確認收貨 (open ConfirmReceiptDialog)
/// status=50 + !isReviewed + canReview → 去評價 (push /review/order)
/// status=50 + isReviewed → 再次購買 (reorder placeholder)
/// 其他狀態 → 不顯示底部 bar。
class _OrderDetailBottomBar extends ConsumerStatefulWidget {
  const _OrderDetailBottomBar({required this.order});
  final Order order;

  @override
  ConsumerState<_OrderDetailBottomBar> createState() =>
      _OrderDetailBottomBarState();
}

class _OrderDetailBottomBarState extends ConsumerState<_OrderDetailBottomBar> {
  bool _busy = false;

  Order get order => widget.order;

  Future<void> _handleRetryPayment(BuildContext context) async {
    if (_busy) return;
    setState(() => _busy = true);
    try {
      await payOrder(context, ref, order.orderSn);
      if (!mounted) return;
      ref.read(orderDetailControllerProvider(order.orderSn).notifier).refresh();
      ref.read(orderListControllerProvider.notifier).refresh();
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _handleConfirmReceipt(BuildContext context) async {
    final result = await ConfirmReceiptDialog.show(
      context,
      orderSn: order.orderSn,
    );
    if (result == null) return;
    if (!context.mounted) return;
    ref.read(orderDetailControllerProvider(order.orderSn).notifier).refresh();
    ref.read(orderListControllerProvider.notifier).refresh();
    showFloatingToast(context, '已確認收貨', isSuccess: true);
  }

  Future<void> _handleReorder(BuildContext context) async {
    if (_busy) return;
    setState(() => _busy = true);
    try {
      final cartRepo = ref.read(cartRepositoryProvider);
      int added = 0;
      int skipped = 0;
      Object? firstAddError;
      for (final item in order.items) {
        final skuId = item.skuId.trim();
        final qty = int.tryParse(item.quantity) ?? 0;
        if (skuId.isEmpty || qty <= 0) {
          skipped++;
          continue;
        }
        try {
          await cartRepo.addToCart(skuId, qty);
          added++;
        } catch (error) {
          firstAddError ??= error;
          skipped++;
        }
      }
      if (!context.mounted) return;
      ref.invalidate(cartCountProvider);
      if (added == 0) {
        showFloatingToast(
          context,
          firstAddError == null
              ? '加入購物車失敗，請稍後再試'
              : addToCartErrorMessage(firstAddError),
          isSuccess: false,
        );
        return;
      }
      final msg = skipped > 0
          ? '已加入 $added 件商品（$skipped 件無法加入）'
          : '已加入 $added 件商品到購物車';
      showFloatingToast(context, msg, isSuccess: true);
      await context.push('/cart');
    } catch (e) {
      if (context.mounted) {
        showFloatingToast(context, addToCartErrorMessage(e), isSuccess: false);
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
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
    if (result == true && context.mounted) {
      ref.read(orderDetailControllerProvider(order.orderSn).notifier).refresh();
      ref.read(orderListControllerProvider.notifier).refresh();
    }
  }

  ({String label, HkIconName icon, VoidCallback onTap})? _action(
    BuildContext context,
  ) {
    switch (order.status) {
      case '0':
        return (
          label: order.paymentDeadlineExpired ? '訂單已逾期' : '立即付款',
          icon: HkIconName.payment,
          onTap: order.paymentDeadlineExpired
              ? () {}
              : () => _handleRetryPayment(context),
        );
      case '40':
        return (
          label: '確認收貨',
          icon: HkIconName.check,
          onTap: () => _handleConfirmReceipt(context),
        );
      case '50':
        if (!order.isReviewed && order.canReview) {
          return (
            label: '去評價',
            icon: HkIconName.star,
            onTap: () => _handleReview(context),
          );
        }
        if (order.items.isNotEmpty) {
          return (
            label: '再次購買',
            icon: HkIconName.refresh,
            onTap: () => _handleReorder(context),
          );
        }
        return null;
      case '60':
      case '70':
      case '80':
        if (order.items.isNotEmpty) {
          return (
            label: '再次購買',
            icon: HkIconName.refresh,
            onTap: () => _handleReorder(context),
          );
        }
        return null;
      default:
        return null;
    }
  }

  @override
  Widget build(BuildContext context) {
    final action = _action(context);
    if (action == null) return const SizedBox.shrink();
    final disabled = order.status == '0' && order.paymentDeadlineExpired;

    return SafeArea(
      top: false,
      child: Container(
        padding: EdgeInsets.fromLTRB(16.w, 12.h, 16.w, 12.h),
        decoration: BoxDecoration(
          color: AppColors.bgCard,
          boxShadow: [
            BoxShadow(
              color: AppColors.cardShadow,
              blurRadius: 12,
              offset: const Offset(0, -2),
            ),
          ],
        ),
        child: SizedBox(
          height: 52.h,
          width: double.infinity,
          child: ElevatedButton.icon(
            onPressed: _busy || disabled ? null : action.onTap,
            icon: _busy
                ? SizedBox(
                    width: 18.w,
                    height: 18.w,
                    child: const HkBrandLoader(
                      tone: HkBrandLoaderTone.mono,
                      color: AppColors.bgCard,
                    ),
                  )
                : HkIcon(action.icon, size: 20.sp),
            label: HkButtonLabel(
              _busy ? '處理中…' : action.label,
              style: AppTypography.buttonLarge.copyWith(letterSpacing: 0.5),
            ),
            style: ElevatedButton.styleFrom(
              backgroundColor: AppColors.primary,
              disabledBackgroundColor: AppColors.gray300,
              foregroundColor: AppColors.bgCard,
              elevation: 0,
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(24.r),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

/// 與商品詳情一致的商家資訊卡：店家身份、聯絡與進店操作各自清楚分工。
class _SellerInfoCard extends ConsumerWidget {
  const _SellerInfoCard({required this.order});

  final Order order;

  void _goToChat(BuildContext context) {
    final id = order.storeId.trim();
    if (id.isEmpty) return;
    context.push(
      '/chat/room',
      extra: {'merchantId': id, 'merchantName': order.sellerName},
    );
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final hasStore = order.storeId.trim().isNotEmpty;
    final card = ref.watch(storeCardProvider(order.storeId)).value;
    final storeCode = card != null && card.isOpen ? card.storeCode : '';
    return Container(
      decoration: _cardDecoration(),
      padding: const EdgeInsets.all(HkSpace.lg),
      child: Row(
        children: [
          ClipOval(
            child: order.sellerAvatarUrl.isNotEmpty
                ? OssImage(
                    imageSource: order.sellerAvatarUrl,
                    scene: ImageScene.avatarSmall,
                    width: 44.w,
                    height: 44.w,
                    fit: BoxFit.cover,
                  )
                : Container(
                    width: 44.w,
                    height: 44.w,
                    alignment: Alignment.center,
                    color: AppColors.primarySoft,
                    child: HkIcon(
                      HkIconName.store,
                      size: 22.sp,
                      color: AppIconColors.active,
                    ),
                  ),
          ),
          const SizedBox(width: HkSpace.md),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  order.sellerName,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: AppTypography.sectionTitle,
                ),
                Text('訂單賣家', style: AppTypography.metaMuted),
              ],
            ),
          ),
          if (hasStore) ...[
            HkTapTarget(
              semanticLabel: '聯絡賣家',
              onTap: () => _goToChat(context),
              child: Container(
                width: 40,
                height: 40,
                alignment: Alignment.center,
                decoration: BoxDecoration(
                  color: AppColors.primarySoft,
                  borderRadius: BorderRadius.circular(HkRadius.md),
                ),
                child: HkIcon(
                  HkIconName.chat,
                  size: 20,
                  color: AppIconColors.active,
                ),
              ),
            ),
            const SizedBox(width: HkSpace.sm),
            HkTapTarget(
              semanticLabel: '進入店舖',
              onTap: storeCode.isEmpty
                  ? null
                  : () => context.push('/merchant/$storeCode'),
              child: Container(
                height: 40,
                padding: const EdgeInsets.symmetric(horizontal: HkSpace.md),
                decoration: BoxDecoration(
                  color: AppColors.actionPrimary,
                  borderRadius: BorderRadius.circular(HkRadius.md),
                  boxShadow: HkShadows.sm,
                ),
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    HkIcon(
                      HkIconName.store,
                      size: 18,
                      color: AppIconColors.onBrand,
                    ),
                    const SizedBox(width: HkSpace.xs),
                    Text(
                      card?.statusLabel(ref.watch(uiLocaleProvider)) ?? '店舖',
                      style: AppTypography.buttonMedium.copyWith(
                        color: AppColors.textInverse,
                        height: 1,
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ],
        ],
      ),
    );
  }
}

class _TransactionOverviewCard extends StatelessWidget {
  const _TransactionOverviewCard({required this.order});

  final Order order;

  @override
  Widget build(BuildContext context) {
    final visual = OrderStatusPresentation.fromOrder(order);
    final rows = <Widget>[];

    if (order.status == '0' && order.paymentDeadline.isNotEmpty) {
      rows.add(
        LiveCountdownLine(
          label: '付款倒數',
          deadlineIso: order.paymentDeadline,
          expiredText: '付款已逾時',
        ),
      );
    }
    if (order.processingDeadline.isNotEmpty &&
        (order.status == '10' || order.status == '20')) {
      rows.add(
        _InfoLine(
          label: '出貨處理期限',
          value: _formatDateTime(
            order.processingDeadline,
            context.taipeiDisplay,
          ),
        ),
      );
    }
    if (order.status == '40' && order.autoCompleteAt.isNotEmpty) {
      rows.add(
        LiveCountdownLine(
          label: '自動完成倒數',
          deadlineIso: order.autoCompleteAt,
          expiredText: '已自動完成',
          urgentThreshold: Duration.zero,
        ),
      );
    }
    final statusColor = visual.color;
    final subtitle = order.statusSummary.trim().isNotEmpty
        ? order.statusSummary.trim()
        : visual.description;

    return Container(
      decoration: _cardDecoration(),
      padding: EdgeInsets.all(16.w),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.center,
            children: [
              Container(
                width: 44.w,
                height: 44.w,
                alignment: Alignment.center,
                decoration: BoxDecoration(
                  color: statusColor.withValues(alpha: 0.12),
                  shape: BoxShape.circle,
                ),
                child: HkIcon(visual.icon, color: statusColor, size: 24.sp),
              ),
              SizedBox(width: 14.w),
              Expanded(
                child: Text(
                  visual.label,
                  style: AppTypography.display.copyWith(
                    height: 1.1,
                    color: statusColor,
                    letterSpacing: 0.3,
                  ),
                ),
              ),
            ],
          ),
          if (subtitle.isNotEmpty) ...[
            SizedBox(height: 10.h),
            Text(
              subtitle,
              style: AppTypography.metaMuted.copyWith(height: 1.5),
            ),
          ],
          if (rows.isNotEmpty) ...[
            const SizedBox(height: HkSpace.lg),
            Divider(
              height: 1,
              color: AppColors.textPrimary.withValues(alpha: 0.12),
            ),
            const SizedBox(height: HkSpace.md),
            ..._withDividers(rows),
          ],
          const SizedBox(height: HkSpace.lg),
          Container(
            key: const ValueKey('order-detail-order-meta'),
            width: double.infinity,
            padding: const EdgeInsets.all(HkSpace.md),
            decoration: BoxDecoration(
              color: AppColors.neutralCream.withValues(alpha: 0.72),
              borderRadius: BorderRadius.circular(HkRadius.md),
            ),
            child: Row(
              children: [
                Expanded(
                  child: Text(
                    '下單 ${_formatDateTime(order.createTime, context.taipeiDisplay).ifEmpty('時間未提供')}',
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: AppTypography.metaMuted,
                  ),
                ),
                const SizedBox(width: HkSpace.sm),
                HkTapTarget(
                  semanticLabel: '複製訂單編號',
                  onTap: order.orderSn.trim().isEmpty
                      ? null
                      : () async {
                          await Clipboard.setData(
                            ClipboardData(text: order.orderSn.trim()),
                          );
                          if (context.mounted) {
                            showFloatingToast(context, '已複製訂單編號');
                          }
                        },
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      HkIcon(
                        HkIconName.copy,
                        size: 16,
                        color: AppIconColors.active,
                      ),
                      const SizedBox(width: HkSpace.xs),
                      ConstrainedBox(
                        constraints: const BoxConstraints(maxWidth: 112),
                        child: Text(
                          order.orderSn.ifEmpty('—'),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: AppTypography.meta.copyWith(
                            color: AppColors.brandPrimaryStrong,
                            fontWeight: FontWeight.w700,
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _RefundTaskSummaryCard extends StatelessWidget {
  const _RefundTaskSummaryCard({
    required this.order,
    required this.afterSaleAsync,
  });

  final Order order;
  final AsyncValue<AfterSaleCase?> afterSaleAsync;

  @override
  Widget build(BuildContext context) {
    final afterSale = afterSaleAsync.value;
    if (afterSale == null ||
        !RefundTaskStatus.isFailed(afterSale.refundTaskStatus)) {
      return const SizedBox.shrink();
    }

    return Padding(
      padding: const EdgeInsets.only(top: HkSpace.lg),
      child: _SimpleInfoCard(
        icon: HkIconName.warning,
        title: '需要你的處理',
        message: afterSale.refundFailureMessage.ifEmpty('退款處理失敗，請聯繫客服協助。'),
        color: OrderColorTokens.attention,
      ),
    );
  }
}

class _ReturnLogisticsCard extends StatelessWidget {
  const _ReturnLogisticsCard({required this.afterSale});

  final AfterSaleCase afterSale;

  @override
  Widget build(BuildContext context) {
    final abnormal = AfterSaleReturnProgress.isAbnormal(afterSale);
    final rows = <Widget>[
      _InfoLine(
        label: '退貨方式',
        value: _returnShippingMethodText(afterSale.returnShippingMethod),
      ),
      _InfoLine(
        label: '退貨貨態',
        value: afterSale.providerStatus.isNotEmpty
            ? _shippingStatusText(afterSale.providerStatus)
            : _returnProgressText(afterSale),
        valueColor: abnormal ? OrderColorTokens.attention : null,
      ),
      if (afterSale.providerMessage.isNotEmpty)
        _InfoLine(
          label: '物流訊息',
          value: afterSale.providerMessage,
          valueColor: abnormal ? OrderColorTokens.attention : null,
        ),
      _CopyableInfoRow(label: '退貨物流號', value: afterSale.returnShipmentNo),
      if (afterSale.shipTradeNo.isNotEmpty)
        _CopyableInfoRow(label: '寄件交易單', value: afterSale.shipTradeNo),
      if (afterSale.shouldShow711ReturnCode)
        _CopyableInfoRow(label: '7-11 退貨碼', value: afterSale.formatted711Code),
    ];

    return Container(
      decoration: _cardDecoration(),
      padding: EdgeInsets.all(16.w),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _CardSectionTitle(
            icon: abnormal ? HkIconName.warning : HkIconName.returnOrder,
            title: '退貨物流',
            iconColor: abnormal ? OrderColorTokens.attention : null,
          ),
          SizedBox(height: 12.h),
          ..._withDividers(rows),
        ],
      ),
    );
  }
}

class _AfterSaleCaseSection extends StatelessWidget {
  const _AfterSaleCaseSection({required this.afterSaleAsync});

  final AsyncValue<AfterSaleCase?> afterSaleAsync;

  @override
  Widget build(BuildContext context) {
    return afterSaleAsync.when(
      loading: () => const SizedBox.shrink(),
      error: (_, _) => const SizedBox.shrink(),
      data: (afterSale) {
        if (afterSale == null) return const SizedBox.shrink();
        final refundLabel = RefundTaskStatus.buyerProgressLabel(
          taskStatus: afterSale.refundTaskStatus,
          failureMessage: afterSale.refundFailureMessage,
        );
        final rows = <Widget>[
          if (afterSale.caseNo.isNotEmpty)
            _CopyableInfoRow(label: '案件編號', value: afterSale.caseNo),
          _InfoLine(
            label: '處理狀態',
            value: _afterSaleStatusText(afterSale.status),
          ),
          if (afterSale.reason.isNotEmpty)
            _InfoLine(
              label: '申請原因',
              value: AfterSaleReason.label(afterSale.reason),
            ),
          if (afterSale.processDeadline.isNotEmpty)
            _InfoLine(
              label: '處理期限',
              value: _formatDateTime(
                afterSale.processDeadline,
                context.taipeiDisplay,
              ),
              valueColor: OrderColorTokens.attention,
            ),
          if (afterSale.resolutions.isNotEmpty)
            _InfoLine(label: '處理結果', value: _resolutionText(afterSale)),
          if (refundLabel.isNotEmpty)
            _InfoLine(
              label: '退款進度',
              value: refundLabel,
              valueColor: RefundTaskStatus.isFailed(afterSale.refundTaskStatus)
                  ? OrderColorTokens.attention
                  : null,
            ),
        ];
        final hasReturnLogistics =
            afterSale.returnShippingMethod.isNotEmpty ||
            afterSale.returnShipmentNo.isNotEmpty ||
            afterSale.providerStatus.isNotEmpty ||
            afterSale.providerMessage.isNotEmpty ||
            afterSale.shipTradeNo.isNotEmpty ||
            afterSale.shouldShow711ReturnCode;

        return Padding(
          padding: const EdgeInsets.only(top: HkSpace.lg),
          child: Column(
            children: [
              Container(
                decoration: _cardDecoration(),
                padding: const EdgeInsets.all(HkSpace.lg),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const _CardSectionTitle(
                      icon: HkIconName.returnOrder,
                      title: '售後處理',
                    ),
                    const SizedBox(height: HkSpace.md),
                    ..._withDividers(rows),
                  ],
                ),
              ),
              if (hasReturnLogistics) ...[
                const SizedBox(height: HkSpace.lg),
                _ReturnLogisticsCard(afterSale: afterSale),
              ],
            ],
          ),
        );
      },
    );
  }
}

class _InfoLine extends StatelessWidget {
  const _InfoLine({required this.label, required this.value, this.valueColor});

  final String label;
  final String value;
  final Color? valueColor;

  @override
  Widget build(BuildContext context) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(
          width: 96.w,
          child: Text(
            label,
            style: _smallStyle().copyWith(color: AppColors.textSecondary),
          ),
        ),
        SizedBox(width: 10.w),
        Expanded(
          child: Text(
            value.ifEmpty('—'),
            textAlign: TextAlign.end,
            style: _valueStyle().copyWith(
              color: valueColor ?? AppColors.textEmphasis,
            ),
          ),
        ),
      ],
    );
  }
}

class _CopyableInfoRow extends StatelessWidget {
  const _CopyableInfoRow({required this.label, required this.value});

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    final normalized = value.trim();
    final canCopy = normalized.isNotEmpty;
    return Row(
      crossAxisAlignment: CrossAxisAlignment.center,
      children: [
        SizedBox(
          width: 88.w,
          child: Text(
            label,
            style: _smallStyle().copyWith(color: AppColors.textSecondary),
          ),
        ),
        SizedBox(width: 10.w),
        Expanded(
          child: SelectableText(
            normalized.ifEmpty('—'),
            textAlign: TextAlign.end,
            style: _valueStyle().copyWith(
              color: canCopy ? AppColors.textEmphasis : AppColors.textSecondary,
            ),
          ),
        ),
        SizedBox(width: 8.w),
        IconButton(
          onPressed: canCopy
              ? () async {
                  await Clipboard.setData(ClipboardData(text: normalized));
                  if (context.mounted) {
                    showFloatingToast(
                      context,
                      '已複製$label',
                      icon: AppIcons.contentCopy,
                    );
                  }
                }
              : null,
          icon: HkIcon(
            HkIconName.copy,
            size: 18.sp,
            color: AppIconColors.active,
          ),
          tooltip: '複製$label',
        ),
      ],
    );
  }
}

class _SimpleInfoCard extends StatelessWidget {
  const _SimpleInfoCard({
    required this.icon,
    required this.title,
    required this.message,
    this.color = AppColors.textPrimary,
  });

  final HkIconName icon;
  final String title;
  final String message;
  final Color color;

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: _cardDecoration(),
      padding: EdgeInsets.all(16.w),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          HkIcon(icon, size: 22.sp, color: color),
          SizedBox(width: 10.w),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(title, style: _sectionTitleStyle()),
                SizedBox(height: 6.h),
                Text(message, style: _smallStyle().copyWith(color: color)),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

BoxDecoration _cardDecoration() => BoxDecoration(
  color: AppColors.bgCard.withValues(alpha: 0.94),
  borderRadius: BorderRadius.circular(HkRadius.lg),
  border: Border.all(color: AppColors.cardBorder),
  boxShadow: HkShadows.card,
);

TextStyle _sectionTitleStyle() => AppTypography.sectionTitle;

TextStyle _valueStyle() =>
    AppTypography.body.copyWith(color: AppColors.textEmphasis);

TextStyle _smallStyle() => AppTypography.metaMuted;

/// 12sp w500 — badge / chip 用，color 由 caller 傳。
TextStyle _captionStyle({Color? color}) => AppTypography.caption.copyWith(
  fontWeight: FontWeight.w500,
  height: 1.3,
  color: color ?? AppColors.textPrimary,
);

/// 卡片內部標題列（如「出貨物流」「付款方式」）— 統一 20sp icon + Title。
class _CardSectionTitle extends StatelessWidget {
  const _CardSectionTitle({
    required this.icon,
    required this.title,
    this.iconColor,
  });
  final HkIconName icon;
  final String title;
  final Color? iconColor;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        HkIcon(
          icon,
          size: 20.sp,
          color: iconColor ?? AppIconColors.defaultColor,
        ),
        const SizedBox(width: HkSpace.sm),
        Expanded(
          child: Text(
            title,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: _sectionTitleStyle(),
          ),
        ),
      ],
    );
  }
}

List<Widget> _withDividers(List<Widget> rows) {
  final widgets = <Widget>[];
  for (var i = 0; i < rows.length; i++) {
    if (i > 0) {
      widgets
        ..add(SizedBox(height: 10.h))
        ..add(
          Divider(
            height: 1,
            color: AppColors.textPrimary.withValues(alpha: 0.12),
          ),
        )
        ..add(SizedBox(height: 10.h));
    }
    widgets.add(rows[i]);
  }
  return widgets;
}

String _shippingMethodText(String value) =>
    ShippingMethod.fromValue(value)?.label ?? (value.isEmpty ? '未指定' : value);

String _shippingStatusText(String value) =>
    ShippingStatus.fromValue(value)?.label ?? (value.isEmpty ? '尚未提供' : value);

String _afterSaleStatusText(String value) =>
    AfterSaleCaseStatus.fromValue(value)?.label ??
    (value.isEmpty ? '未知' : value);

String _returnShippingMethodText(String value) => switch (value) {
  ReturnShippingMethod.kuroneko => '黑貓宅配退貨',
  ReturnShippingMethod.sevenEleven => '7-11 店到店退貨',
  '' => '尚未指定',
  _ => value,
};

String _resolutionText(AfterSaleCase c) {
  if (c.resolutions.isEmpty) {
    return switch (c.status) {
      AfterSaleStatus.pending => '等待賣家處理',
      AfterSaleStatus.rejected => '賣家已拒絕',
      AfterSaleStatus.cancelled => '買家已取消',
      AfterSaleStatus.cancelledByConfirmReceipt => '買家確認收貨，自動取消',
      _ => '尚未產生決策紀錄',
    };
  }
  final r = c.resolutions.last;
  return switch (r.resolutionType) {
    ResolutionType.approved =>
      r.refundAmount.isNotEmpty ? '同意退貨退款，退款 NT\$ ${r.refundAmount}' : '同意退貨退款',
    ResolutionType.coupon =>
      r.couponAmount.isNotEmpty ? '改發優惠券 NT\$ ${r.couponAmount}' : '改發優惠券',
    ResolutionType.reject => r.rejectMessage.ifEmpty(
      r.rejectReason.ifEmpty('賣家拒絕'),
    ),
    ResolutionType.negotiation => r.negotiationReason.ifEmpty('協商中'),
    ResolutionType.inspectionDeduction => [
      '驗貨扣款',
      if (r.refundAmount.isNotEmpty) '退款 NT\$ ${r.refundAmount}',
      if (r.deductionReason.isNotEmpty) r.deductionReason,
    ].join('｜'),
    _ => r.resolutionType.ifEmpty('尚未產生決策紀錄'),
  };
}

String _returnProgressText(AfterSaleCase c) {
  if (AfterSaleReturnProgress.isAbnormal(c)) return '退貨物流異常，請聯繫客服';
  if (AfterSaleReturnProgress.isDelivered(c)) return '退貨已送達賣家端';
  if (AfterSaleReturnProgress.hasShipped(c)) return '買家已寄出，物流運送中';
  if (c.status == AfterSaleStatus.awaitingReturn) return '等待買家寄回商品';
  if (c.status == AfterSaleStatus.pending) return '等待賣家處理';
  return _afterSaleStatusText(c.status);
}

String _formatDateTime(String raw, TaipeiDisplay t) {
  return t.formatIso(raw);
}

extension _OrderDetailStringX on String {
  String ifEmpty(String fallback) => trim().isEmpty ? fallback : this;
}

class _OrderInfoCard extends StatelessWidget {
  const _OrderInfoCard({required this.order});
  final Order order;

  @override
  Widget build(BuildContext context) {
    final (addressLabel, addressValue) = switch (shippingPresentationOf(
      order.shippingMethod,
    )) {
      ShippingPresentation.cvsStorePickup => (
        '取貨門市',
        order.pickupStoreName.isNotEmpty ? order.pickupStoreName : '未指定',
      ),
      ShippingPresentation.homeDelivery => (
        '配送地址',
        order.shippingAddress.isNotEmpty ? order.shippingAddress : '未指定',
      ),
      ShippingPresentation.unsupported => (
        '配送資訊',
        order.pickupStoreName.isNotEmpty
            ? order.pickupStoreName
            : (order.shippingAddress.isNotEmpty
                  ? order.shippingAddress
                  : '未指定'),
      ),
    };
    final deliveryText = [
      _shippingMethodText(order.shippingMethod),
    ].where((value) => value.isNotEmpty && value != '未指定').join(' · ');
    final rows = <Widget>[
      if (order.recipientName.isNotEmpty)
        _InfoLine(label: '收件人', value: order.recipientName),
      _InfoLine(label: addressLabel, value: addressValue),
      if (order.recipientPhone.isNotEmpty)
        _InfoLine(label: '聯絡電話', value: order.recipientPhone),
      if (deliveryText.isNotEmpty)
        _InfoLine(label: '配送方式', value: deliveryText),
      if (order.shipmentNo.isNotEmpty)
        _CopyableInfoRow(label: '物流單號', value: order.shipmentNo),
    ];

    return Container(
      decoration: _cardDecoration(),
      padding: const EdgeInsets.all(HkSpace.lg),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const _CardSectionTitle(icon: HkIconName.location, title: '收件與配送'),
          const SizedBox(height: HkSpace.md),
          ..._withDividers(rows),
        ],
      ),
    );
  }
}

class _ItemsCard extends StatelessWidget {
  const _ItemsCard({required this.order});
  final Order order;

  @override
  Widget build(BuildContext context) {
    final totalQuantity = order.items.fold<int>(
      0,
      (sum, item) => sum + (int.tryParse(item.quantity) ?? 0),
    );
    return Container(
      decoration: _cardDecoration(),
      padding: const EdgeInsets.all(HkSpace.lg),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              const Expanded(
                child: _CardSectionTitle(icon: HkIconName.goods, title: '商品'),
              ),
              if (order.items.isNotEmpty)
                Text(
                  '${order.items.length} 項 · $totalQuantity 件',
                  style: AppTypography.metaMuted,
                ),
            ],
          ),
          const SizedBox(height: HkSpace.md),
          if (order.items.isEmpty)
            Row(
              children: [
                HkIcon(
                  HkIconName.goods,
                  size: 24.sp,
                  color: AppIconColors.muted,
                ),
                SizedBox(width: 10.w),
                Expanded(
                  child: Text(
                    '商品資料尚未同步，訂單金額與付款紀錄仍可正常查看。',
                    style: _smallStyle().copyWith(
                      color: AppColors.textSecondary,
                    ),
                  ),
                ),
              ],
            )
          else ...[
            ...List.generate(order.items.length, (i) {
              final item = order.items[i];
              return Column(
                children: [
                  if (i > 0) ...[
                    Divider(
                      color: AppColors.textPrimary.withValues(alpha: 0.2),
                      height: 24.h,
                    ),
                  ],
                  _OrderItemRow(item: item),
                ],
              );
            }),
          ],
        ],
      ),
    );
  }
}

class _OrderItemRow extends StatelessWidget {
  const _OrderItemRow({required this.item});
  final OrderItem item;

  @override
  Widget build(BuildContext context) {
    final quantity = item.quantity.trim().isEmpty ? '1' : item.quantity.trim();
    final unitPrice = item.effectiveUnitPrice.trim().isEmpty
        ? '—'
        : item.effectiveUnitPrice.trim();
    final actualAmount = item.actualPayAmount.trim().isNotEmpty
        ? item.actualPayAmount.trim()
        : unitPrice;
    final refundLabel = _itemRefundStatusLabel(item.refundStatus);
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        ClipRRect(
          borderRadius: BorderRadius.circular(12.r),
          child: item.imageUrl.isNotEmpty
              ? OssImage(
                  imageSource: item.imageUrl,
                  scene: ImageScene.orderItem,
                  width: 80.w,
                  height: 80.w,
                  fit: BoxFit.cover,
                )
              : Container(
                  width: 80.w,
                  height: 80.w,
                  color: AppColors.primarySoft,
                  child: Icon(
                    AppIcons.image,
                    size: 32.sp,
                    color: AppIconColors.secondary,
                  ),
                ),
        ),
        SizedBox(width: 12.w),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                item.productName.isNotEmpty ? item.productName : '商品',
                style: _valueStyle().copyWith(color: AppColors.textPrimary),
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
              ),
              if (item.skuName.trim().isNotEmpty) ...[
                SizedBox(height: 2.h),
                Text(
                  item.skuName.trim(),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: _smallStyle().copyWith(color: AppColors.textSecondary),
                ),
              ],
              SizedBox(height: 8.h),
              Row(
                crossAxisAlignment: CrossAxisAlignment.end,
                children: [
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          '單價 NT\$ $unitPrice × $quantity',
                          style: _smallStyle().copyWith(
                            color: AppColors.textSecondary,
                          ),
                        ),
                        if (item.shouldStrikeListPrice) ...[
                          SizedBox(height: 2.h),
                          Text(
                            '原價 NT\$ ${item.effectiveListPrice}',
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: _smallStyle().copyWith(
                              color: AppColors.gray400,
                              decoration: TextDecoration.lineThrough,
                              decorationColor: AppColors.gray400,
                            ),
                          ),
                        ],
                      ],
                    ),
                  ),
                  SizedBox(width: 12.w),
                  Text(
                    'NT\$ $actualAmount',
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    textAlign: TextAlign.right,
                    style: _sectionTitleStyle().copyWith(
                      color: AppColors.brandPrimaryStrong,
                    ),
                  ),
                ],
              ),
              if (item.shareDiscountValue > 0) ...[
                SizedBox(height: 4.h),
                Text(
                  '品項折抵 -NT\$ ${item.shareDiscount}',
                  style: _captionStyle(color: OrderColorTokens.attention),
                ),
              ],
              if (refundLabel.isNotEmpty) ...[
                SizedBox(height: 6.h),
                Container(
                  padding: EdgeInsets.symmetric(horizontal: 8.w, vertical: 4.h),
                  decoration: BoxDecoration(
                    color: OrderColorTokens.attentionSurface,
                    borderRadius: BorderRadius.circular(999.r),
                  ),
                  child: Text(
                    refundLabel,
                    style: _captionStyle(color: OrderColorTokens.attention),
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

String _itemRefundStatusLabel(String raw) {
  final value = raw.trim().toUpperCase();
  return switch (value) {
    '' || '0' || 'NONE' => '',
    '1' || 'PENDING' || 'PROCESSING' => '退款處理中',
    '2' || 'PARTIAL' || 'PARTIAL_REFUNDED' => '此商品部分退款',
    '3' || 'REFUNDED' || 'FULL_REFUNDED' => '此商品已退款',
    '4' || 'REJECTED' => '退款申請未通過',
    _ => '退款狀態更新中',
  };
}

class _PaymentDetailsCard extends StatelessWidget {
  const _PaymentDetailsCard({required this.order});
  final Order order;

  @override
  Widget build(BuildContext context) {
    final originalAmount = order.totalOriginalAmount.isNotEmpty
        ? order.totalOriginalAmount
        : '0';
    final shippingFee = order.shippingFee.isNotEmpty ? order.shippingFee : '0';
    final discount = order.discountAmount.isNotEmpty
        ? order.discountAmount
        : '0';
    final totalAmount = order.totalActualAmount.isNotEmpty
        ? order.totalActualAmount
        : '0';
    final refundAmountText = _refundAmountText(order, totalAmount);
    final badge = _paymentStatusBadge(order);
    final method = _paymentMethodText(order.paymentMethod);
    final showBreakdown =
        _amountIsNonZero(shippingFee) ||
        _amountIsNonZero(discount) ||
        originalAmount != totalAmount;

    return Container(
      decoration: _cardDecoration(),
      padding: const EdgeInsets.all(HkSpace.lg),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              const Expanded(
                child: _CardSectionTitle(icon: HkIconName.payment, title: '付款'),
              ),
              Container(
                padding: const EdgeInsets.symmetric(
                  horizontal: HkSpace.sm,
                  vertical: HkSpace.xs,
                ),
                decoration: BoxDecoration(
                  color: badge.color.withValues(alpha: 0.14),
                  borderRadius: BorderRadius.circular(HkRadius.pill),
                ),
                child: Text(
                  badge.label,
                  style: AppTypography.caption.copyWith(
                    color: badge.color,
                    fontWeight: FontWeight.w700,
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: HkSpace.lg),
          _PaymentTotalLine(method: method, totalAmount: totalAmount),
          if (showBreakdown) ...[
            const SizedBox(height: HkSpace.md),
            Divider(color: AppColors.textPrimary.withValues(alpha: 0.12)),
            const SizedBox(height: HkSpace.sm),
            _PaymentRow(label: '商品小計', amount: 'NT\$ $originalAmount'),
            if (_amountIsNonZero(shippingFee))
              _PaymentRow(label: '運費', amount: 'NT\$ $shippingFee'),
            if (_amountIsNonZero(discount))
              _PaymentRow(label: '優惠', amount: '− NT\$ $discount'),
          ],
          if (order.status == '60' ||
              order.status == '70' ||
              order.status == '80') ...[
            const SizedBox(height: HkSpace.md),
            Divider(color: AppColors.textPrimary.withValues(alpha: 0.12)),
            const SizedBox(height: HkSpace.sm),
            Row(
              children: [
                Text('退款金額', style: AppTypography.metaMuted),
                const Spacer(),
                Text(
                  refundAmountText,
                  style: AppTypography.bodyBold.copyWith(
                    color: OrderColorTokens.attention,
                  ),
                ),
              ],
            ),
          ],
        ],
      ),
    );
  }
}

class _PaymentTotalLine extends StatelessWidget {
  const _PaymentTotalLine({required this.method, required this.totalAmount});

  final String method;
  final String totalAmount;

  @override
  Widget build(BuildContext context) {
    final methodStyle = AppTypography.metaMuted;
    final unitStyle = AppTypography.priceUnit.copyWith(
      color: AppColors.brandPrimaryStrong,
      height: 1.9,
    );
    final amountStyle = AppTypography.display.copyWith(
      color: AppColors.brandPrimaryStrong,
      height: 1.1,
      letterSpacing: -0.3,
      fontFeatures: const [FontFeature.tabularFigures()],
    );
    final textScaler = MediaQuery.textScalerOf(context);
    final textDirection = Directionality.of(context);

    Widget inlineAmount() => Row(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.end,
      children: [
        Text('NT\$ ', maxLines: 1, softWrap: false, style: unitStyle),
        Text(
          totalAmount,
          key: const ValueKey('order-detail-payment-amount'),
          maxLines: 1,
          softWrap: false,
          style: amountStyle,
        ),
      ],
    );

    Widget stackedAmount() => Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Align(
          alignment: Alignment.centerRight,
          child: Text('NT\$', maxLines: 1, softWrap: false, style: unitStyle),
        ),
        Text(
          totalAmount,
          key: const ValueKey('order-detail-payment-amount'),
          maxLines: 2,
          softWrap: true,
          textAlign: TextAlign.right,
          style: amountStyle,
        ),
      ],
    );

    return Semantics(
      key: const ValueKey('order-detail-payment-total'),
      container: true,
      label: '付款金額 NT\$ $totalAmount',
      excludeSemantics: true,
      child: LayoutBuilder(
        builder: (context, constraints) {
          final methodWidth = _singleLineTextWidth(
            method,
            methodStyle,
            textScaler,
            textDirection,
          );
          final amountWidth =
              _singleLineTextWidth(
                'NT\$ ',
                unitStyle,
                textScaler,
                textDirection,
              ) +
              _singleLineTextWidth(
                totalAmount,
                amountStyle,
                textScaler,
                textDirection,
              );
          final fitsInline =
              methodWidth + HkSpace.md + amountWidth <= constraints.maxWidth;

          if (fitsInline) {
            return Row(
              key: const ValueKey('order-detail-payment-total-inline'),
              crossAxisAlignment: CrossAxisAlignment.end,
              children: [
                Text(method, maxLines: 1, softWrap: false, style: methodStyle),
                const Spacer(),
                inlineAmount(),
              ],
            );
          }

          return Column(
            key: const ValueKey('order-detail-payment-total-stacked'),
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(method, maxLines: 1, softWrap: false, style: methodStyle),
              const SizedBox(height: HkSpace.xs),
              stackedAmount(),
            ],
          );
        },
      ),
    );
  }
}

double _singleLineTextWidth(
  String text,
  TextStyle style,
  TextScaler textScaler,
  TextDirection textDirection,
) {
  final painter = TextPainter(
    text: TextSpan(text: text, style: style),
    maxLines: 1,
    textScaler: textScaler,
    textDirection: textDirection,
  )..layout();
  return painter.width.ceilToDouble();
}

bool _amountIsNonZero(String value) {
  final normalized = value.replaceAll(',', '').trim();
  return (num.tryParse(normalized) ?? 0) != 0;
}

({String label, Color color}) _paymentStatusBadge(Order order) {
  switch (order.status) {
    case '0':
      return (label: '尚未付款', color: OrderColorTokens.attentionFill);
    case '80':
      return (label: '付款已關閉', color: OrderColorTokens.neutral);
    case '60':
      return (label: '退款調整', color: OrderColorTokens.attentionFill);
    case '70':
      return (label: '退款完成', color: OrderColorTokens.neutral);
    default:
      return (label: '付款完成', color: OrderColorTokens.primary);
  }
}

String _refundAmountText(Order order, String fallbackTotalAmount) {
  num sum = 0;
  for (final record in order.refundHistory) {
    sum += num.tryParse(record.refundAmount.replaceAll(',', '')) ?? 0;
  }
  if (sum > 0) {
    return 'NT\$ ${sum.toStringAsFixed(sum.truncateToDouble() == sum ? 0 : 2)}';
  }
  if (order.status == '60') return '核算中';
  return 'NT\$ $fallbackTotalAmount';
}

class _PaymentRow extends StatelessWidget {
  const _PaymentRow({required this.label, required this.amount});
  final String label;
  final String amount;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: HkSpace.xs),
      child: Row(
        children: [
          Text(label, style: AppTypography.metaMuted),
          const Spacer(),
          Text(
            amount,
            style: AppTypography.meta.copyWith(
              fontFeatures: const [FontFeature.tabularFigures()],
            ),
          ),
        ],
      ),
    );
  }
}

class _NoteCard extends StatelessWidget {
  const _NoteCard({required this.remark});
  final String remark;

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: _cardDecoration(),
      padding: const EdgeInsets.all(HkSpace.lg),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const _CardSectionTitle(icon: HkIconName.edit, title: '訂單備註'),
          const SizedBox(height: HkSpace.sm),
          Text(
            remark,
            style: AppTypography.body.copyWith(color: AppColors.textEmphasis),
          ),
        ],
      ),
    );
  }
}

String _paymentMethodText(String value) =>
    PaymentMethod.fromValue(value)?.label ??
    PaymentMethod.fromCode(value.toUpperCase())?.label ??
    (value.isEmpty ? '付款方式未指定' : value);

class _OrderReviewCard extends ConsumerStatefulWidget {
  const _OrderReviewCard({required this.order});
  final Order order;

  @override
  ConsumerState<_OrderReviewCard> createState() => _OrderReviewCardState();
}

class _OrderReviewCardState extends ConsumerState<_OrderReviewCard> {
  late Future<_ReviewData?> _reviewFuture;

  @override
  void initState() {
    super.initState();
    _reviewFuture = _loadReview();
  }

  Future<_ReviewData?> _loadReview() async {
    if (widget.order.reviewRating > 0 &&
        widget.order.reviewContent.isNotEmpty) {}
    try {
      final orderItemIds = widget.order.items
          .map((i) => i.id)
          .where((id) => id.isNotEmpty)
          .toSet();
      if (orderItemIds.isEmpty) return null;
      final review = await ref
          .read(reviewRepositoryProvider)
          .findMyReviewForOrderItems(orderItemIds);
      if (review == null) {
        if (widget.order.reviewRating > 0) {
          return _ReviewData(
            rating: widget.order.reviewRating,
            content: widget.order.reviewContent,
            images: const [],
          );
        }
        return null;
      }
      return _ReviewData.fromReview(review);
    } catch (_) {
      if (widget.order.reviewRating > 0) {
        return _ReviewData(
          rating: widget.order.reviewRating,
          content: widget.order.reviewContent,
          images: const [],
        );
      }
      return null;
    }
  }

  @override
  Widget build(BuildContext context) {
    return FutureBuilder<_ReviewData?>(
      future: _reviewFuture,
      builder: (context, snapshot) {
        final data = snapshot.data;
        if (snapshot.connectionState != ConnectionState.done) {
          return const SizedBox.shrink();
        }
        if (data == null || data.rating <= 0) {
          return const SizedBox.shrink();
        }
        return Padding(
          padding: const EdgeInsets.only(top: HkSpace.lg),
          child: Container(
            decoration: _cardDecoration(),
            padding: EdgeInsets.symmetric(horizontal: 16.w, vertical: 16.h),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const _CardSectionTitle(
                  icon: HkIconName.star,
                  title: '我的評價',
                  iconColor: OrderColorTokens.attention,
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
                    style: AppTypography.meta.copyWith(
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
                      itemCount: data.images.length > 4
                          ? 4
                          : data.images.length,
                      separatorBuilder: (_, __) => SizedBox(width: 8.w),
                      itemBuilder: (context, index) {
                        final image = data.images[index];
                        return ClipRRect(
                          borderRadius: BorderRadius.circular(12.r),
                          child: SizedBox(
                            width: 80.w,
                            height: 80.w,
                            child: AppCachedImage.api(
                              image: image,
                              scene: ImageScene.reviewPhoto,
                              width: 80.w,
                              height: 80.w,
                              fit: BoxFit.cover,
                            ),
                          ),
                        );
                      },
                    ),
                  ),
                ],
              ],
            ),
          ),
        );
      },
    );
  }
}

/// 評價卡片資料來源整合（`ReviewAppService.ListMyReviews`，失敗時用 Order 自帶的值）
class _ReviewData {
  const _ReviewData({
    required this.rating,
    required this.content,
    required this.images,
  });

  final int rating;
  final String content;
  final List<pb_image.Image> images;

  factory _ReviewData.fromReview(pb_review.Review review) => _ReviewData(
    rating: review.rating.clamp(0, 5),
    content: review.content.trim(),
    images: review.images,
  );
}
