import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:hawkr_buyer/core/codemeta/code_meta.dart'
    hide OrderStatus, OrderItemStatus;

import '../../../core/payment/pay_token_service.dart';
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
import '../data/after_sale_order_lookup.dart';
import '../data/after_sale_repository.dart';
import '../data/order_model.dart';
import 'order_detail_controller.dart';
import '../data/order_repository.dart';
import '../data/payment_repository.dart';
import 'confirm_receipt_dialog.dart';
import 'order_card_widgets/order_card_widgets.dart';
import 'order_list_controller.dart';
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
  bool _detailLoading = false;
  Order? _detailOrder;
  // B-185: 待付款倒數每 30 秒刷新一次（不需要每秒 tick，省電 + 視覺穩定）
  Timer? _paymentDeadlineTicker;

  Order get order => widget.order;
  bool get _isPendingPayment => order.status == '0';
  bool get _isDelivered => order.status == '40';
  bool get _isAtmPayment =>
      (_isPendingPayment ? _detailOrder?.paymentMethod : order.paymentMethod) ==
      PaymentMethod.atm.value;
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
        // 重 build 讓 OrderInfoColumn 重算 paymentCountdownLabel。
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

  // ─── Build ───────────────────────────────────────────────

  @override
  Widget build(BuildContext context) {
    // 邊框 / 陰影 / 圓角 — 對齊 Figma node 10632:149278
    // 用 strokeAlignInside（預設）：邊框畫在 clip path 內側，搭配 Clip.antiAlias
    // 圓角不會出現不連續。strokeAlignOutside 在 antialias clip 下角落會被切。
    final cardBorderSide = BorderSide(width: 1, color: AppColors.cardBorder);

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

                // ── 店家與收件摘要 ──
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
                        onTap: order.storeId.isEmpty
                            ? null
                            : () => context.push('/merchant/${order.storeId}'),
                        child: OrderSellerColumn(order: order),
                      ),
                      SizedBox(width: AppSpacing.sm),
                      Expanded(
                        child: OrderInfoColumn(
                          // 待付款卡片優先吃 detail（含 paymentDeadline）；
                          // detail 還沒載完先用列表級 order 撐住基本資訊。
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

                // 待付款必須直接看到操作；其他訂單預設收合，訂單編號與按鈕
                // 僅在展開後顯示。
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
              isAtmPayment: _isAtmPayment,
              detailLoading: _detailLoading,
              isPayLoading: _isPayLoading,
              isConfirmLoading: _isConfirmLoading,
              onShowAtmInfo: () => showAtmInfoSheet(
                context,
                order: order,
                detailOrder: _detailOrder,
              ),
              onRetryPayment: () => showRetryPaymentSheet(
                context,
                order: _detailOrder ?? order,
                currentPayType:
                    _detailOrder?.paymentMethod ?? order.paymentMethod,
                onPaymentSelected: (payType) =>
                    _executePayment(context, payType),
              ),
              onOpenRefundPage: () => _openRefundPage(context, ref),
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
    // 列表契約未帶圖片時，使用同一筆訂單詳情的商品快照補齊；
    // 其他文字仍優先採列表資料，避免等待詳情時整列跳動。
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
                    // 圖片失效時維持乾淨的商品圖示，不顯示破圖符號。
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

  // ─── 展開內容切換 ───────────────────────────────────────

  Widget _buildExpandedContent() {
    // 僅 watch 最新 snapshot；lookup 的刷新由 [OrderListScreen._refreshCurrentTab]
    // 統一觸發（下拉刷新 / tab 切換 / FCM / lifecycle resume / 首次挂载）
    final snapshot = ref
        .watch(afterSaleOrderLookupProvider)
        .value?[order.orderSn];
    if (snapshot != null && _isDelivered) {
      return AfterSaleExpandedSection(snapshot: snapshot, order: order);
    }
    return OrderExpandedSection(order: order);
  }

  // ─── Actions ────────────────────────────────────────────

  Future<void> _fetchOrderDetail() async {
    setState(() => _detailLoading = true);
    try {
      final detail = await ref
          .read(orderRepositoryProvider)
          .getOrderDetail(order.orderSn);
      if (mounted) setState(() => _detailOrder = detail);
    } catch (_) {
    } finally {
      if (mounted) setState(() => _detailLoading = false);
    }
  }

  Future<void> _executePayment(BuildContext context, String payType) async {
    if (_isPayLoading) return;
    setState(() => _isPayLoading = true);
    try {
      String? applePayToken;
      if (payType == PaymentMethod.applePay.value) {
        applePayToken = await _requestApplePayToken(context);
        if (applePayToken == null) return;
      }

      final payRepo = ref.read(paymentRepositoryProvider);
      debugPrint('[RetryPay] orderNo=${order.orderSn} payType=$payType');
      final result = await payRepo.createPayment(
        orderNo: order.orderSn,
        paymentType: payType,
        applePayToken: applePayToken,
      );
      if (!context.mounted) return;

      if (payType == PaymentMethod.atm.value) {
        await _fetchOrderDetail();
        if (!context.mounted) return;
        ref.read(orderListControllerProvider.notifier).refresh();
        showFloatingToast(context, '已切換為 ATM 轉帳，請查看匯款資訊', isSuccess: true);
        return;
      }

      final responsePaymentType =
          result['paymentType']?.toString() ??
          result['paymentMethod']?.toString() ??
          '';
      final paidWithoutRedirect =
          _paymentResultIsPaid(result['status']) &&
          (responsePaymentType == PaymentMethod.zeroAmount.value ||
              _amountIsZero(
                result['amount']?.toString() ?? order.totalActualAmount,
              ));
      if (paidWithoutRedirect) {
        await _fetchOrderDetail();
        if (!context.mounted) return;
        ref.read(orderListControllerProvider.notifier).refresh();
        showFloatingToast(context, '折抵後無需付款，訂單已完成付款', isSuccess: true);
        return;
      }

      final redirectUrl = result['redirectUrl']?.toString() ?? '';
      final tradeNo = result['tradeNo']?.toString() ?? '';
      final amount = result['amount']?.toString() ?? order.totalActualAmount;
      if (redirectUrl.isNotEmpty) {
        if (!context.mounted) return;
        await context.push(
          '/payment/webview',
          extra: {
            'redirectUrl': redirectUrl,
            'tradeNo': tradeNo,
            'orderNo': order.orderSn,
            'amount': amount,
          },
        );
        if (mounted) {
          ref.read(orderListControllerProvider.notifier).refresh();
        }
      } else {
        if (!context.mounted) return;
        showFloatingToast(context, '無法取得付款頁面', isSuccess: false);
      }
    } catch (e) {
      if (context.mounted) {
        showFloatingToast(
          context,
          '付款失敗：${e.toString().replaceAll('Exception: ', '')}',
          isSuccess: false,
        );
      }
    } finally {
      if (mounted) setState(() => _isPayLoading = false);
    }
  }

  Future<void> _openRefundPage(BuildContext context, WidgetRef ref) async {
    if (_isPayLoading) return;
    setState(() => _isPayLoading = true);
    try {
      final url = await ref
          .read(afterSaleRepositoryProvider)
          .getRefundPageUrl(order.orderSn);
      if (!context.mounted) return;
      await context.push(
        '/refund/webview',
        extra: {'redirectUrl': url, 'orderNo': order.orderSn},
      );
      if (context.mounted) {
        ref.invalidate(afterSaleOrderLookupProvider);
      }
    } catch (e) {
      if (context.mounted) {
        showFloatingToast(context, '載入退款頁面失敗：$e', isSuccess: false);
      }
    } finally {
      if (mounted) setState(() => _isPayLoading = false);
    }
  }

  Future<String?> _requestApplePayToken(BuildContext context) async {
    final payTokenService = PayTokenService();
    try {
      await payTokenService.init();
      if (!context.mounted) return null;
      if (!payTokenService.isApplePayAvailable) {
        showFloatingToast(
          context,
          '此裝置尚未啟用 Apple Pay，請選擇其他付款方式',
          isSuccess: false,
        );
        return null;
      }
      return await payTokenService.requestToken(
        amount: _retryPaymentAmount,
        label: 'Hawkr 訂單',
      );
    } on PaymentCancelledException {
      if (context.mounted) {
        showFloatingToast(context, 'Apple Pay 付款已取消', isSuccess: false);
      }
      return null;
    } on PaymentTokenUnavailableException catch (e) {
      if (context.mounted) {
        showFloatingToast(context, '付款失敗：$e', isSuccess: false);
      }
      return null;
    } catch (e, stackTrace) {
      debugPrint('[OrderCard] Apple Pay request failed: $e');
      debugPrintStack(stackTrace: stackTrace);
      if (context.mounted) {
        showFloatingToast(context, 'Apple Pay 授權失敗，請稍後再試', isSuccess: false);
      }
      return null;
    } finally {
      payTokenService.dispose();
    }
  }

  String get _retryPaymentAmount {
    final detailAmount = _detailOrder?.totalActualAmount ?? '';
    if (detailAmount.isNotEmpty) return detailAmount;
    if (order.totalActualAmount.isNotEmpty) return order.totalActualAmount;
    return '0';
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
    // 讓詳情頁也重抓資料，新拍的照片才會被 _PhotosCard 顯示出來
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

bool _paymentResultIsPaid(dynamic value) {
  final status = value?.toString().trim().toUpperCase() ?? '';
  return status == '1' ||
      status == 'PAID' ||
      status == 'SUCCESS' ||
      status == '1102';
}

bool _amountIsZero(String value) {
  final cleaned = value.replaceAll(RegExp(r'[^0-9.-]'), '');
  if (cleaned.isEmpty) return false;
  return (num.tryParse(cleaned) ?? -1) == 0;
}
