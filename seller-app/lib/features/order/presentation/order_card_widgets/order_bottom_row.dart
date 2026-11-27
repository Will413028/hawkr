import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart' show Clipboard, ClipboardData;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';

import '../../../../core/analytics/analytics_providers.dart';
import '../../../../core/codemeta/code_meta.dart';
import '../../../../core/codemeta/shipping_presentation.dart';
import '../../../../core/exceptions/app_exception.dart';
import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_typography.dart';
import '../../../../shared/utils/floating_toast.dart';
import '../../../../shared/widgets/hk_icon.dart';
import '../../data/after_sale_model.dart';
import '../../data/after_sale_repository.dart';
import '../../data/order_model.dart';
import '../../data/order_repository.dart';
import '../../data/seller_after_sale_lookup.dart';
import '../after_sales_confirm_receipt_dialog.dart';
import '../after_sales_resolve_sheet.dart';
import '../call_tcat_dialog.dart';
import '../seven_eleven_ship_dialog.dart';
import '../shipment_error_dispatch.dart';
import '../tcat_reservation_dialog.dart';
import 'order_expanded_section.dart';
import 'pill_button.dart';

typedef OnSetOrderLoading = void Function(String orderId, bool loading);
typedef OnReload = void Function();

/// 卡片底部行：分隔線 + 單號（靠左）+ 操作按鈕群（靠右）。
///
/// 從 [OrderInfoColumn] 拆出後，放在卡片層作為兄弟元素，
/// 使單號能與頭像左對齊，而非與資訊欄對齊。
class OrderBottomRow extends ConsumerWidget {
  const OrderBottomRow({
    super.key,
    required this.order,
    required this.isPending,
    required this.onSetLoading,
    required this.onReload,
    required this.onReloadAndSwitchTab,
  });

  final OrderSummary order;
  final bool isPending;
  final OnSetOrderLoading onSetLoading;
  final OnReload onReload;
  final void Function(int tabIndex) onReloadAndSwitchTab;

  String _orderId() => order.orderSn.isNotEmpty ? order.orderSn : order.id;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final displayId = _orderId();
    final shipping = shippingPresentationOf(order.shippingMethod);

    void onDetailTap() => context.push('/orders/matched/detail', extra: order);

    final lookup = ref.watch(sellerAfterSaleLookupProvider);
    final afterSale = lookup.value?[displayId];
    final canResolve = afterSale != null && afterSale.status == '0';
    final canConfirmReturn =
        afterSale != null && AfterSaleStatus.canConfirmReturnReceipt(afterSale);

    final reference = '訂單 ${displayId.isEmpty ? '—' : displayId}';
    final referenceStyle = AppTypography.meta.copyWith(
      color: AppColors.textSecondary,
    );
    final detail = PillButton(
      label: '細節',
      color: AppColors.primary,
      onTap: onDetailTap,
    );

    return LayoutBuilder(
      builder: (context, constraints) {
        double textWidth(String text, TextStyle style) {
          final painter = TextPainter(
            text: TextSpan(text: text, style: style),
            textDirection: Directionality.of(context),
            textScaler: MediaQuery.textScalerOf(context),
          )..layout();
          final width = painter.width;
          painter.dispose();
          return width;
        }

        final detailWidth =
            (textWidth(
                      '細節',
                      AppTypography.meta.copyWith(fontWeight: FontWeight.w500),
                    ) +
                    24.w)
                .clamp(48.0, double.infinity);
        final inlineDetail =
            textWidth(reference, referenceStyle) + 48 + 8.w + detailWidth <=
            constraints.maxWidth;
        final actions = <Widget>[
          if (!inlineDetail) detail,
          if (canResolve)
            PillButton(
              label: '售後申請',
              color: AppColors.error,
              onTap: () => _showResolve(context, ref, afterSale),
            ),
          if (canConfirmReturn)
            PillButton(
              label: '確認收貨',
              color: AppColors.warning,
              onTap: () => _showConfirmReturnReceipt(context, ref, afterSale),
            ),
          if (isPending) ...[
            ..._buildShippingButtons(context, ref, shipping),
            if (shipping.isSupported)
              IconButton(
                onPressed: () => _showShippingInfoDialog(context, shipping),
                tooltip: '查看出貨說明',
                constraints: const BoxConstraints.tightFor(
                  width: 48,
                  height: 48,
                ),
                padding: EdgeInsets.zero,
                icon: HkIcon(
                  HkIconName.info,
                  size: 22.sp,
                  color: AppIconColors.defaultColor,
                ),
              ),
          ],
        ];

        return Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Row(
              children: [
                Expanded(
                  child: Text(
                    reference,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: referenceStyle,
                  ),
                ),
                IconButton(
                  onPressed: displayId.isEmpty
                      ? null
                      : () {
                          Clipboard.setData(ClipboardData(text: displayId));
                          FloatingToast.show(
                            context,
                            '已複製單號',
                            icon: AppIcons.check,
                          );
                        },
                  tooltip: '複製訂單編號',
                  constraints: const BoxConstraints.tightFor(
                    width: 48,
                    height: 48,
                  ),
                  padding: EdgeInsets.zero,
                  icon: HkIcon(
                    HkIconName.copy,
                    size: 18.sp,
                    color: AppIconColors.active,
                  ),
                ),
                if (inlineDetail) ...[SizedBox(width: 8.w), detail],
              ],
            ),
            if (actions.isNotEmpty)
              Align(
                alignment: Alignment.centerRight,
                child: Wrap(
                  spacing: 8.w,
                  runSpacing: 4.h,
                  alignment: WrapAlignment.end,
                  crossAxisAlignment: WrapCrossAlignment.center,
                  children: actions,
                ),
              ),
          ],
        );
      },
    );
  }

  List<Widget> _buildShippingButtons(
    BuildContext context,
    WidgetRef ref,
    ShippingPresentation shipping,
  ) {
    final ss = order.shipmentStatus;

    if (!shipping.isSupported) {
      return [
        Text(
          '此配送方式暫不支援 App 出貨',
          style: AppTypography.meta.copyWith(color: AppColors.textSecondary),
        ),
      ];
    }

    if (shipping == ShippingPresentation.homeDelivery) {
      if (ss == ShippingStatus.tcatCalled.value) {
        if (order.labelUrl.isNotEmpty) {
          return [
            PillButton(
              label: '下載 PDF',
              color: AppColors.warning,
              onTap: () => _downloadPdf(context),
            ),
          ];
        }
        return [];
      }
      if (order.labelUrl.isNotEmpty) {
        return [
          PillButton(
            label: '呼叫黑貓取件',
            color: AppColors.warning,
            onTap: () => _handleCallTcat(context, ref),
          ),
        ];
      }
      return [
        PillButton(
          label: '預約出貨',
          color: AppColors.warning,
          onTap: () => _handleReserveShipment(context, ref),
        ),
      ];
    }

    final statusCode = order.statusCode ?? int.tryParse(order.subtitle) ?? -1;
    if (statusCode >= 30) return [];
    final activeOdno = effectiveOdno(_orderId(), order.odno);
    if (activeOdno.isNotEmpty) {
      return [
        PillButton(
          label: '出貨確認',
          color: AppColors.warning,
          onTap: () => _handleSevenElevenConfirm(context, ref),
        ),
      ];
    }
    return [
      PillButton(
        label: '預約取號',
        color: AppColors.warning,
        onTap: () => _handleSevenElevenPrintLabel(context, ref),
      ),
    ];
  }

  /// 售後申請 — 內聯彈 resolve sheet，提交決策後刷新列表與 lookup 快取。
  /// 複製自 after_sales_screen._AfterSaleCard._showResolve，改用 [onReload]
  /// 作為刷新鉤子並 invalidate [sellerAfterSaleLookupProvider] 讓
  /// 訂單卡按鈕狀態即時同步（避免決策後仍顯示「售後申請」）。
  Future<void> _showResolve(
    BuildContext context,
    WidgetRef ref,
    AfterSaleCase afterSale,
  ) async {
    final orderId = _orderId();
    onSetLoading(orderId, true);
    late final AfterSaleCase detail;
    var sellerShipmentPhotoUrls = order.shipmentPhotoUrls;
    var buyerReceiptPhotoUrls = order.receiptPhotoUrls;
    try {
      detail = await ref
          .read(afterSaleRepositoryProvider)
          .getAfterSaleDetail(afterSale.caseNo);
      if (sellerShipmentPhotoUrls.isEmpty || buyerReceiptPhotoUrls.isEmpty) {
        try {
          final orderDetail = await ref
              .read(orderRepositoryProvider)
              .fetchMatchedOrderDetails(
                orderSn: detail.orderNo.isNotEmpty ? detail.orderNo : orderId,
              );
          if (sellerShipmentPhotoUrls.isEmpty) {
            sellerShipmentPhotoUrls = orderDetail.shipmentPhotoUrls;
          }
          if (buyerReceiptPhotoUrls.isEmpty) {
            buyerReceiptPhotoUrls = orderDetail.receiptPhotoUrls;
          }
        } catch (_) {}
      }
    } catch (_) {
      if (context.mounted) {
        FloatingToast.show(context, '載入售後詳情失敗', isSuccess: false);
      }
      onSetLoading(orderId, false);
      return;
    }
    onSetLoading(orderId, false);
    if (!context.mounted) return;

    showAfterSalesResolveSheet(
      context: context,
      afterSaleCase: detail,
      orderNo: detail.orderNo,
      refundableAmount: detail.refundableAmount,
      allowedReturnShippingMethod: order.shippingMethod,
      sellerShipmentPhotoUrls: sellerShipmentPhotoUrls,
      buyerReceiptPhotoUrls: buyerReceiptPhotoUrls,
      onSubmit: (data) async {
        try {
          final result = await ref
              .read(afterSaleRepositoryProvider)
              .resolve(
                detail.caseNo,
                resolutionType: data['resolutionType'] as String,
                returnShippingMethod: data['returnShippingMethod'] as String?,
                refundAmount: data['refundAmount'] as String?,
                couponAmount: data['couponAmount'] as String?,
                couponMinSpend: data['couponMinSpend'] as String?,
                couponMessage: data['couponMessage'] as String?,
                rejectReason: data['rejectReason'] as String?,
                rejectMessage: data['rejectMessage'] as String?,
              );
          ref.invalidate(sellerAfterSaleLookupProvider);
          onReload();
          if (context.mounted) {
            final resType = data['resolutionType'] as String;
            final shipMethod = data['returnShippingMethod'] as String?;
            final isTaqbin = shipMethod == ShippingMethod.taqbin.value;
            final is711 =
                shipMethod == ShippingMethod.sevenElevenStorePickup.value;
            final shipNo = result.returnShipmentNo;

            if (resType == AfterSaleResolutionType.returnRefund.value) {
              if (isTaqbin) {
                FloatingToast.show(
                  context,
                  '已預約黑貓上門取件${shipNo.isNotEmpty ? '\n取貨住址已建立' : ''}',
                );
              } else if (is711) {
                FloatingToast.show(
                  context,
                  '已產生 7-11 退貨代碼${shipNo.isNotEmpty ? '\n代碼：$shipNo' : ''}',
                );
              } else {
                FloatingToast.show(context, '已建立退貨物流單');
              }
            } else {
              FloatingToast.show(context, '已提交處理決策，款項將原路退還');
            }
          }
        } catch (e) {
          if (context.mounted) {
            FloatingToast.show(context, '提交失敗：$e');
          }
        }
      },
    );
  }

  Future<void> _showConfirmReturnReceipt(
    BuildContext context,
    WidgetRef ref,
    AfterSaleCase afterSale,
  ) async {
    final orderId = _orderId();
    onSetLoading(orderId, true);
    late final AfterSaleCase detail;
    try {
      detail = await ref
          .read(afterSaleRepositoryProvider)
          .getAfterSaleDetail(afterSale.caseNo);
    } catch (_) {
      if (context.mounted) {
        FloatingToast.show(context, '載入售後詳情失敗', isSuccess: false);
      }
      onSetLoading(orderId, false);
      return;
    }
    onSetLoading(orderId, false);
    if (!context.mounted) return;

    await showAfterSaleConfirmReceiptDialog(
      context: context,
      afterSaleCase: detail,
      onSubmit: (data) async {
        onSetLoading(orderId, true);
        try {
          await ref
              .read(afterSaleRepositoryProvider)
              .confirmReceipt(
                detail.caseNo,
                actualRefundAmount: data.actualRefundAmount,
                deductionReason: data.deductionReason,
                inspectionNote: data.inspectionNote,
                inspectionPhotoTokens: data.inspectionPhotoTokens,
              );
          ref.invalidate(sellerAfterSaleLookupProvider);
          onReload();
          if (context.mounted) {
            FloatingToast.show(context, '已確認收貨，退款處理中');
          }
        } catch (_) {
          if (context.mounted) {
            FloatingToast.show(context, '確認收貨失敗', isSuccess: false);
          }
          rethrow;
        } finally {
          onSetLoading(orderId, false);
        }
      },
    );
  }

  Future<void> _handleReserveShipment(
    BuildContext context,
    WidgetRef ref,
  ) async {
    final orderNo = _orderId();
    if (guardShipmentCooldown(context, orderNo)) return;

    final result = await showDialog<TcatReservationResult>(
      context: context,
      barrierColor: AppColors.textPrimary.withValues(alpha: 0.8),
      builder: (_) => const TcatReservationDialog(),
    );
    if (result == null || !context.mounted) return;

    onSetLoading(orderNo, true);
    try {
      List<String>? stagedTokens;
      if (result.photo != null) {
        final token = await uploadShipmentPhoto(
          ref,
          context,
          result.photo!,
          orderNo,
        );
        if (!context.mounted) return;
        if (token != null) stagedTokens = [token];
      }
      await ref
          .read(orderRepositoryProvider)
          .generateHomeLabel(
            orderNo,
            shipDate: result.shipDate,
            stagedTokens: stagedTokens,
          );
      if (!context.mounted) return;
      markShipmentCreated(orderNo);
      resetShipmentRetryCount(orderNo);
      onSetLoading(orderNo, false);
      FloatingToast.success(context, '預約出貨成功，記得下載列印「寄件單.PDF」，並預約黑貓司機上門取件');
      onReload();
    } catch (e) {
      if (!context.mounted) return;
      onSetLoading(orderNo, false);
      if (handleShipmentApiError(context, e, orderNo: orderNo)) {
        if (e is ApiException && shipmentBusinessCode(e) == 5047) onReload();
        return;
      }
      final msg = e.toString();
      if (msg.contains('已取號')) {
        FloatingToast.show(
          context,
          '此訂單已取號，正在更新狀態...',
          icon: AppIcons.infoOutline,
        );
        onReload();
      } else {
        FloatingToast.error(context, '預約出貨失敗：${friendlyShipmentMessage(e)}');
      }
    }
  }

  Future<void> _handleCallTcat(BuildContext context, WidgetRef ref) async {
    final orderNo = _orderId();
    final params = await showDialog<CallTcatParams>(
      context: context,
      barrierColor: AppColors.textPrimary.withValues(alpha: 0.8),
      builder: (_) => const CallTcatDialog(),
    );
    if (params == null || !context.mounted) return;

    onSetLoading(orderNo, true);
    try {
      await ref
          .read(orderRepositoryProvider)
          .callTcat(
            orderNo,
            isContact: params.isContact,
            isTrolley: params.isTrolley,
            memo: params.memo,
          );
      if (!context.mounted) return;
      onSetLoading(orderNo, false);
      FloatingToast.success(context, '已呼叫黑貓取件，司機將於兩日內到府收件');
      unawaited(ref.read(analyticsEventsProvider).requestLogistics(orderNo));
      onReload();
    } catch (e) {
      if (!context.mounted) return;
      onSetLoading(orderNo, false);
      if (handleShipmentApiError(context, e, orderNo: orderNo)) {
        if (e is ApiException && shipmentBusinessCode(e) == 5047) onReload();
        return;
      }
      final msg = e.toString().replaceFirst('Exception: ', '');
      if (msg.contains('假日') ||
          msg.contains('週日') ||
          msg.contains('工作天') ||
          msg.contains('E072')) {
        _showTcatHolidayDialog(context);
      } else {
        FloatingToast.error(context, '呼叫黑貓取件失敗，請稍後再試');
      }
    }
  }

  Future<void> _handleSevenElevenPrintLabel(
    BuildContext context,
    WidgetRef ref,
  ) async {
    final orderNo = _orderId();
    if (guardShipmentCooldown(context, orderNo)) return;

    onSetLoading(orderNo, true);
    try {
      final label = await ref
          .read(orderRepositoryProvider)
          .printCvsLabel(orderNo);
      if (!context.mounted) return;
      onSetLoading(orderNo, false);
      if (!label.webhookReceived) {
        FloatingToast.show(
          context,
          '系統繁忙中，交貨便代碼即將生成，請稍後刷新',
          icon: AppIcons.sync,
        );
        onReload();
        return;
      }
      if (label.odno.isEmpty) {
        FloatingToast.error(context, '無法取得交貨便代碼');
        return;
      }
      markShipmentCreated(orderNo);
      resetShipmentRetryCount(orderNo);
      setLocalOdnoOverride(
        orderNo,
        odno: label.odno,
        validationNo: label.validationNo,
      );
      FloatingToast.success(context, '取號成功：${label.odno}${label.validationNo}');
      unawaited(ref.read(analyticsEventsProvider).printCvsLabel(orderNo));
      onReload();
    } catch (e) {
      if (!context.mounted) return;
      onSetLoading(orderNo, false);
      if (handleShipmentApiError(context, e, orderNo: orderNo)) {
        if (e is ApiException && shipmentBusinessCode(e) == 5047) onReload();
        return;
      }
      FloatingToast.error(context, '預約取號失敗：${friendlyShipmentMessage(e)}');
    }
  }

  Future<void> _handleSevenElevenConfirm(
    BuildContext context,
    WidgetRef ref,
  ) async {
    final orderNo = _orderId();
    final result = await showDialog<SevenElevenShipResult>(
      context: context,
      barrierColor: AppColors.textPrimary.withValues(alpha: 0.8),
      builder: (_) => SevenElevenShipDialog(
        odno: effectiveOdno(orderNo, order.odno),
        validationNo: effectiveValidationNo(orderNo, order.validationNo),
      ),
    );
    if (result == null || !context.mounted) return;

    onSetLoading(orderNo, true);
    try {
      List<String>? stagedTokens;
      if (result.photo != null) {
        final token = await uploadShipmentPhoto(
          ref,
          context,
          result.photo!,
          orderNo,
        );
        if (!context.mounted) return;
        if (token != null) stagedTokens = [token];
      }
      await ref
          .read(orderRepositoryProvider)
          .confirmShipment(orderNo, stagedTokens: stagedTokens);
      if (!context.mounted) return;
      onSetLoading(orderNo, false);
      FloatingToast.success(context, '出貨確認成功');
      unawaited(
        ref
            .read(analyticsEventsProvider)
            .shipOrder(orderNo, shippingMethod: order.shippingMethod),
      );
      onReloadAndSwitchTab(1);
    } catch (e) {
      if (!context.mounted) return;
      onSetLoading(orderNo, false);
      FloatingToast.error(context, '出貨確認失敗：$e');
    }
  }

  Future<void> _downloadPdf(BuildContext context) async {
    final labelUrl = order.labelUrl;
    if (labelUrl.isEmpty) {
      FloatingToast.error(context, '無法取得託運單 PDF');
      return;
    }
    FloatingToast.show(context, '下載托運單完成，請自行列印貼在商品上', icon: AppIcons.download);
  }

  void _showTcatHolidayDialog(BuildContext context) {
    showDialog(
      context: context,
      builder: (dCtx) => Dialog(
        backgroundColor: Colors.transparent,
        insetPadding: const EdgeInsets.symmetric(horizontal: 48),
        child: Container(
          decoration: BoxDecoration(
            color: AppColors.bgCard,
            borderRadius: BorderRadius.circular(12),
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Padding(
                padding: EdgeInsets.fromLTRB(20.w, 24.h, 20.w, 16.h),
                child: Column(
                  children: [
                    Text(
                      '無法預約取件',
                      style: TextStyle(
                        fontSize: 16.sp,
                        fontWeight: FontWeight.w700,
                        color: AppColors.textPrimary,
                      ),
                    ),
                    SizedBox(height: 8.h),
                    Text(
                      '黑貓宅急便於週日及國定假日不受理預約取件，請於工作天再操作。',
                      textAlign: TextAlign.center,
                      style: TextStyle(
                        fontSize: 14.sp,
                        color: AppColors.textPrimary.withValues(alpha: 0.8),
                        height: 1.5,
                      ),
                    ),
                  ],
                ),
              ),
              Container(height: 1, color: AppColors.textSecondary),
              InkWell(
                onTap: () => Navigator.of(dCtx).pop(),
                child: SizedBox(
                  height: 44.h,
                  child: Center(
                    child: Text(
                      '我知道了',
                      style: TextStyle(
                        fontSize: 16.sp,
                        fontWeight: FontWeight.w700,
                        color: AppColors.textPrimary,
                      ),
                    ),
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  void _showShippingInfoDialog(
    BuildContext context,
    ShippingPresentation shipping,
  ) {
    if (!shipping.isSupported) return;
    final isHomeDelivery = shipping == ShippingPresentation.homeDelivery;
    showDialog(
      context: context,
      barrierColor: AppColors.textPrimary.withValues(alpha: 0.8),
      builder: (ctx) => Stack(
        children: [
          Center(
            child: Container(
              width: 370.w,
              margin: EdgeInsets.symmetric(horizontal: 24.w),
              padding: EdgeInsets.symmetric(horizontal: 16.w, vertical: 20.h),
              decoration: BoxDecoration(
                color: AppColors.bgCard,
                borderRadius: BorderRadius.circular(16.r),
                border: Border.all(
                  color: AppColors.textPrimary.withValues(alpha: 0.2),
                ),
                boxShadow: const [
                  BoxShadow(
                    color: AppColors.cardShadow,
                    offset: Offset(0, 1),
                    blurRadius: 2,
                  ),
                ],
              ),
              child: SingleChildScrollView(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Container(
                      width: 45.w,
                      height: 45.w,
                      decoration: BoxDecoration(
                        color: AppColors.bgCard,
                        borderRadius: BorderRadius.circular(12.r),
                        border: Border.all(color: AppColors.textPrimary),
                      ),
                      child: ClipRRect(
                        borderRadius: BorderRadius.circular(12.r),
                        child: Center(
                          child: HkIcon(
                            isHomeDelivery
                                ? HkIconName.truck
                                : HkIconName.store,
                            size: 28.sp,
                            color: AppIconColors.defaultColor,
                          ),
                        ),
                      ),
                    ),
                    SizedBox(height: 6.h),
                    Text(
                      isHomeDelivery ? '如何使用黑貓宅配寄件？' : '如何使用賣貨便寄件？',
                      style: TextStyle(
                        fontSize: 16.sp,
                        fontWeight: FontWeight.w700,
                        color: AppColors.textEmphasis,
                      ),
                    ),
                    SizedBox(height: 10.h),
                    Divider(
                      color: AppColors.textPrimary.withValues(alpha: 0.2),
                    ),
                    SizedBox(height: 10.h),
                    if (isHomeDelivery)
                      ..._buildSteps(_taqbinSteps())
                    else
                      ..._buildSteps(_sevenElevenSteps()),
                    SizedBox(height: 10.h),
                    Divider(
                      color: AppColors.textPrimary.withValues(alpha: 0.2),
                    ),
                    SizedBox(height: 10.h),
                    Row(
                      children: [
                        HkIcon(
                          HkIconName.info,
                          size: 24.sp,
                          color: AppIconColors.defaultColor,
                        ),
                        SizedBox(width: 10.w),
                        Text(
                          '注意事項',
                          style: TextStyle(
                            fontSize: 16.sp,
                            fontWeight: FontWeight.w700,
                            color: AppColors.textEmphasis,
                          ),
                        ),
                      ],
                    ),
                    SizedBox(height: 12.h),
                    if (isHomeDelivery)
                      ..._taqbinNotes()
                    else
                      ..._sevenElevenNotes(),
                  ],
                ),
              ),
            ),
          ),
          Positioned(
            top: MediaQuery.of(ctx).padding.top + 12,
            right: 12,
            child: GestureDetector(
              onTap: () => Navigator.pop(ctx),
              child: Container(
                width: 32.w,
                height: 32.w,
                decoration: const BoxDecoration(
                  color: AppColors.bgCard,
                  shape: BoxShape.circle,
                ),
                child: HkIcon(
                  HkIconName.close,
                  size: 20.sp,
                  color: AppIconColors.defaultColor,
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }

  List<(String, String)> _taqbinSteps() => [
    ('買家建立訂單', ''),
    ('列印寄件單', '至統一金流後台【寄件管理】頁，選取出貨訂單並列印或在 Hawkr【我的訂單－按下載pdf按鈕】並且列印檔案'),
    ('黏貼寄件單', '請平貼於包裹左上角'),
    ('商品包裝商品', '冷藏(0°C~7°C)包裹需預冷 6 小時以上\n冷凍(-15˚C以下)包裹需預冷 12 小時以上'),
    ('預約收件', '點選預約收件並依照欄位填寫本次出貨資訊，預約成功則黑貓司機約於兩日內上門收貨'),
    ('轉運中', ''),
    ('配送至收件地址', ''),
  ];

  List<(String, String)> _sevenElevenSteps() => [
    ('買家建立訂單', ''),
    ('列印寄件單', '至統一金流後台【寄件管理】頁，選取出貨訂單並列印寄件單或在 Hawkr【我的訂單】記下交貨便編號至門市ibon機取得寄件單'),
    ('或至門市取得寄件單', '到ibon點選【購物/寄貨】'),
    ('門市交貨', '門市櫃台辦理'),
    ('配送至至取件門市', '店到店常溫服務支援配送至各離島門市，店到店冷凍離島目前僅能配送至部份澎湖門市'),
    ('到貨通知', '簡訊通知買家至指定門市取貨'),
  ];

  List<Widget> _buildSteps(List<(String, String)> steps) {
    return steps.asMap().entries.map((e) {
      final number = e.key + 1;
      final title = e.value.$1;
      final desc = e.value.$2;
      return Padding(
        padding: EdgeInsets.only(bottom: 8.h),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Container(
              width: 24.w,
              height: 24.w,
              decoration: const BoxDecoration(
                color: AppColors.primary,
                shape: BoxShape.circle,
              ),
              child: Center(
                child: Text(
                  '$number',
                  style: TextStyle(
                    fontSize: 16.sp,
                    fontWeight: FontWeight.w700,
                    color: AppColors.bgCard,
                  ),
                ),
              ),
            ),
            SizedBox(width: 16.w),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    title,
                    style: TextStyle(
                      fontSize: 14.sp,
                      fontWeight: FontWeight.w700,
                      color: AppColors.textEmphasis,
                    ),
                  ),
                  if (desc.isNotEmpty) ...[
                    SizedBox(height: 2.h),
                    Text(
                      desc,
                      style: TextStyle(
                        fontSize: 14.sp,
                        color: AppColors.textPrimary,
                        height: 22.75 / 14,
                      ),
                    ),
                  ],
                ],
              ),
            ),
          ],
        ),
      );
    }).toList();
  }

  List<Widget> _taqbinNotes() => [
    _note('常溫尺寸限制：', '長+寬+高 ≦ 150cm；重量 ≦ 20kg。'),
    SizedBox(height: 12.h),
    _note('冷藏/冷凍尺寸限制：', '長+寬+高 ≦ 120cm，重量 ≦ 20kg。'),
    SizedBox(height: 12.h),
    _note('寄件代碼效期：', '請於代碼產生後 4 天內完成寄件，逾期代碼將失效。'),
  ];

  List<Widget> _sevenElevenNotes() => [
    _note('常溫尺寸限制：', '材積最長邊 ≦ 45cm；長+寬+高 合計 ≦ 105cm；重量 ≦ 5kg。'),
    SizedBox(height: 12.h),
    _note('冷凍尺寸限制：', '冷凍交貨專用箱 60 / 80 / 90cm，可依需求選購；重量 ≦ 10kg。'),
    SizedBox(height: 12.h),
    _note('寄件代碼效期：', '請於代碼產生後 4 天內完成寄件，逾期代碼將失效。'),
  ];

  Widget _note(String label, String content) {
    return Text.rich(
      TextSpan(
        children: [
          TextSpan(
            text: label,
            style: TextStyle(
              fontSize: 14.sp,
              fontWeight: FontWeight.w700,
              color: AppColors.textEmphasis,
            ),
          ),
          TextSpan(
            text: content,
            style: TextStyle(fontSize: 14.sp, color: AppColors.textPrimary),
          ),
        ],
      ),
    );
  }
}
