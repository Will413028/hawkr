import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../config/image_config.dart';
import '../../../core/codemeta/code_meta_display.dart';
import '../../../core/theme/app_colors.dart';
import '../../../shared/utils/floating_toast.dart';
import '../../../shared/widgets/hk_glass_app_bar.dart';
import '../../../shared/widgets/hk_icon.dart';
import '../../../shared/widgets/optimized_image.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../data/after_sale_model.dart';
import '../data/after_sale_repository.dart';
import '../data/after_sale_status_notifier.dart';
import '../data/order_repository.dart';
import 'after_sales_confirm_receipt_dialog.dart';
import 'after_sales_resolve_sheet.dart';

class AfterSalesDetailScreen extends HookConsumerWidget {
  const AfterSalesDetailScreen({super.key, required this.caseNo});
  final String caseNo;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final loading = useState(true);
    final detail = useState<AfterSaleCase?>(null);
    final error = useState<String?>(null);
    final actionLoading = useState(false);
    final orderShippingMethod = useState<String?>(null);
    final orderShipmentPhotoUrls = useState<List<String>>(const []);
    final orderReceiptPhotoUrls = useState<List<String>>(const []);

    Future<void> load() async {
      loading.value = true;
      error.value = null;
      try {
        final result = await ref
            .read(afterSaleRepositoryProvider)
            .getAfterSaleDetail(caseNo);
        if (result.orderNo.isNotEmpty) {
          try {
            final order = await ref
                .read(orderRepositoryProvider)
                .fetchMatchedOrderDetails(orderSn: result.orderNo);
            orderShippingMethod.value = order.shippingMethod.isNotEmpty
                ? order.shippingMethod
                : null;
            orderShipmentPhotoUrls.value = order.shipmentPhotoUrls;
            orderReceiptPhotoUrls.value = order.receiptPhotoUrls;
          } catch (_) {
            orderShippingMethod.value = null;
            orderShipmentPhotoUrls.value = const [];
            orderReceiptPhotoUrls.value = const [];
          }
        }
        if (context.mounted) {
          detail.value = result;
          loading.value = false;
        }
      } catch (e) {
        if (context.mounted) {
          error.value = '載入失敗';
          loading.value = false;
        }
      }
    }

    useEffect(() {
      load();
      return null;
    }, [caseNo]);

    // FCM：本案件狀態變更時自動刷新
    ref.listen(afterSaleStatusStreamProvider, (_, next) {
      next.whenData((event) {
        if (event.caseNo == caseNo) load();
      });
    });

    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      appBar: HkGlassAppBar(title: const Text('售後詳情'), subtitle: caseNo),
      body: loading.value
          ? const TruckLoadingOverlay()
          : error.value != null
          ? Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(
                    error.value!,
                    style: const TextStyle(color: AppColors.danger),
                  ),
                  SizedBox(height: 8.h),
                  OutlinedButton(
                    onPressed: load,
                    child: const HkButtonLabel('重試'),
                  ),
                ],
              ),
            )
          : _buildDetail(
              context,
              ref,
              detail,
              actionLoading,
              load,
              orderShippingMethod.value,
              orderShipmentPhotoUrls.value,
              orderReceiptPhotoUrls.value,
            ),
    );
  }

  Widget _buildDetail(
    BuildContext context,
    WidgetRef ref,
    ValueNotifier<AfterSaleCase?> detail,
    ValueNotifier<bool> actionLoading,
    Future<void> Function() reload,
    String? allowedReturnShippingMethod,
    List<String> orderShipmentPhotoUrls,
    List<String> orderReceiptPhotoUrls,
  ) {
    final c = detail.value!;
    final statusLabel = AfterSaleStatus.label(c.status);
    final reasonLabel = AfterSaleReason.label(c.reason);

    return SingleChildScrollView(
      padding: EdgeInsets.all(16.w),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Status header
          _SectionCard(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Flexible(
                      child: Text(
                        '案件 ${c.caseNo}',
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: TextStyle(
                          fontSize: 16.sp,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                    ),
                    const Spacer(),
                    _StatusBadge(status: c.status, label: statusLabel),
                  ],
                ),
                SizedBox(height: 4.h),
                Text(
                  '訂單 ${c.orderNo}',
                  style: TextStyle(
                    fontSize: 14.sp,
                    color: AppColors.textSecondary,
                  ),
                ),
              ],
            ),
          ),
          SizedBox(height: 12.h),
          _buildProcessNotice(c),
          if (c.autoEscalated || c.processDeadline.isNotEmpty)
            SizedBox(height: 12.h),

          // Reason & description
          _SectionCard(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                _LabelValue(label: '申請原因', value: reasonLabel),
                if (c.description.isNotEmpty) ...[
                  SizedBox(height: 8.h),
                  _LabelValue(label: '詳細說明', value: c.description),
                ],
                SizedBox(height: 8.h),
                _LabelValue(label: '可退金額', value: 'NT\$ ${c.refundableAmount}'),
                SizedBox(height: 8.h),
                _LabelValue(label: '申請時間', value: _formatDate(c.createdAt)),
              ],
            ),
          ),
          SizedBox(height: 12.h),

          // Items
          if (c.items.isNotEmpty) ...[
            _SectionCard(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    '商品項目',
                    style: TextStyle(
                      fontSize: 14.sp,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                  SizedBox(height: 8.h),
                  ...c.items.map(
                    (item) => Padding(
                      padding: EdgeInsets.only(bottom: 8.h),
                      child: Row(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          ClipRRect(
                            borderRadius: BorderRadius.circular(6.r),
                            child: item.imageKey.isNotEmpty
                                ? OptimizedNetworkImage(
                                    imageUrl: item.imageKey,
                                    scene: ImageScene.orderItem,
                                    width: 48.w,
                                    height: 48.w,
                                    fit: BoxFit.cover,
                                    errorWidget: _imagePlaceholder(),
                                  )
                                : _imagePlaceholder(),
                          ),
                          SizedBox(width: 10.w),
                          Expanded(
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Text(
                                  item.skuName,
                                  style: TextStyle(fontSize: 14.sp),
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                ),
                                // PayUni #758: 顯示 effectiveUnitPrice，有 sale 時旁邊加 listPrice 刪除線
                                Row(
                                  crossAxisAlignment: CrossAxisAlignment.end,
                                  children: [
                                    Text(
                                      'x${item.quantity}  NT\$ ${item.effectiveUnitPrice}',
                                      style: TextStyle(
                                        fontSize: 12.sp,
                                        color: AppColors.textSecondary,
                                      ),
                                    ),
                                    if (item.shouldStrikeListPrice) ...[
                                      SizedBox(width: 6.w),
                                      Text(
                                        'NT\$ ${item.effectiveListPrice}',
                                        style: TextStyle(
                                          fontSize: 12.sp,
                                          color: AppColors.textSecondary,
                                          decoration:
                                              TextDecoration.lineThrough,
                                          decorationColor: const Color(
                                            0xFF99A1AF,
                                          ),
                                        ),
                                      ),
                                    ],
                                  ],
                                ),
                              ],
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),
                ],
              ),
            ),
            SizedBox(height: 12.h),
          ],

          // Resolutions history
          if (c.resolutions.isNotEmpty) ...[
            _SectionCard(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    '處理紀錄',
                    style: TextStyle(
                      fontSize: 14.sp,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                  SizedBox(height: 8.h),
                  ...c.resolutions.map((r) => _ResolutionTile(resolution: r)),
                ],
              ),
            ),
            SizedBox(height: 12.h),
          ],

          // Return shipment info
          if (c.returnShipmentNo.isNotEmpty) ...[
            _SectionCard(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  _LabelValue(label: '退貨物流單號', value: c.returnShipmentNo),
                  if (c.returnShippingMethod.isNotEmpty)
                    _LabelValue(
                      label: '退貨方式',
                      value: CodeMetaDisplay.shippingMethod(
                        c.returnShippingMethod,
                      ),
                    ),
                ],
              ),
            ),
            SizedBox(height: 12.h),
          ],

          // Action buttons based on status
          if (c.status == AfterSaleStatus.pending) ...[
            _buildPendingActions(
              context,
              ref,
              c,
              actionLoading,
              reload,
              allowedReturnShippingMethod,
              orderShipmentPhotoUrls,
              orderReceiptPhotoUrls,
            ),
          ],
          // 唯一允許按「確認收貨」的狀態 = 27 RETURN_DELIVERED（與 backend mapper SQL 對齊）
          if (c.status == AfterSaleStatus.returnDelivered) ...[
            _buildConfirmReceiptAction(context, ref, c, actionLoading, reload),
          ],

          SizedBox(height: 32.h),
        ],
      ),
    );
  }

  Widget _buildProcessNotice(AfterSaleCase c) {
    if (c.autoEscalated) {
      return _NoticeCard(
        icon: HkIconName.support,
        title: '平台已接手',
        message: '此案件已超過處理期限並交由平台協助處理。',
        color: AppColors.error,
      );
    }

    final isWaiting = c.status == AfterSaleStatus.pending;
    if (!isWaiting || c.processDeadline.isEmpty) {
      return const SizedBox.shrink();
    }

    final hoursText = c.processDeadlineHours > 0
        ? '${c.processDeadlineHours} 小時內'
        : '期限內';
    return _NoticeCard(
      icon: HkIconName.clock,
      title: '請準時處理售後',
      message: '請於 $hoursText 確認原因，期限：${_formatDeadline(c.processDeadline)}。',
      color: AppColors.warningText,
    );
  }

  Widget _buildPendingActions(
    BuildContext context,
    WidgetRef ref,
    AfterSaleCase c,
    ValueNotifier<bool> actionLoading,
    Future<void> Function() reload,
    String? allowedReturnShippingMethod,
    List<String> orderShipmentPhotoUrls,
    List<String> orderReceiptPhotoUrls,
  ) {
    return Column(
      children: [
        SizedBox(
          width: double.infinity,
          height: 48.h,
          child: ElevatedButton(
            style: ElevatedButton.styleFrom(
              backgroundColor: AppColors.primary,
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(12.r),
              ),
            ),
            onPressed: actionLoading.value
                ? null
                : () => _showResolveSheet(
                    context,
                    ref,
                    c,
                    actionLoading,
                    reload,
                    allowedReturnShippingMethod,
                    orderShipmentPhotoUrls,
                    orderReceiptPhotoUrls,
                  ),
            child: HkButtonLabel(
              '處理售後',
              style: TextStyle(fontSize: 16.sp, color: AppColors.bgCard),
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildConfirmReceiptAction(
    BuildContext context,
    WidgetRef ref,
    AfterSaleCase c,
    ValueNotifier<bool> actionLoading,
    Future<void> Function() reload,
  ) {
    return SizedBox(
      width: double.infinity,
      height: 48.h,
      child: ElevatedButton(
        style: ElevatedButton.styleFrom(
          backgroundColor: AppColors.primary,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(12.r),
          ),
        ),
        onPressed: actionLoading.value
            ? null
            : () => showAfterSaleConfirmReceiptDialog(
                context: context,
                afterSaleCase: c,
                onSubmit: (data) async {
                  actionLoading.value = true;
                  try {
                    await ref
                        .read(afterSaleRepositoryProvider)
                        .confirmReceipt(
                          c.caseNo,
                          actualRefundAmount: data.actualRefundAmount,
                          deductionReason: data.deductionReason,
                          inspectionNote: data.inspectionNote,
                          inspectionPhotoTokens: data.inspectionPhotoTokens,
                        );
                    if (context.mounted) {
                      FloatingToast.show(context, '已確認收貨，退款處理中');
                      await reload();
                    }
                  } catch (e) {
                    if (context.mounted) {
                      FloatingToast.show(context, '確認收貨失敗');
                    }
                    rethrow;
                  } finally {
                    actionLoading.value = false;
                  }
                },
              ),
        child: HkButtonLabel(
          '確認收貨',
          style: TextStyle(fontSize: 16.sp, color: AppColors.bgCard),
        ),
      ),
    );
  }

  void _showResolveSheet(
    BuildContext context,
    WidgetRef ref,
    AfterSaleCase c,
    ValueNotifier<bool> actionLoading,
    Future<void> Function() reload,
    String? allowedReturnShippingMethod,
    List<String> orderShipmentPhotoUrls,
    List<String> orderReceiptPhotoUrls,
  ) {
    showAfterSalesResolveSheet(
      context: context,
      afterSaleCase: c,
      orderNo: c.orderNo,
      refundableAmount: c.refundableAmount,
      allowedReturnShippingMethod:
          allowedReturnShippingMethod ??
          (c.returnShippingMethod.isNotEmpty ? c.returnShippingMethod : null),
      sellerShipmentPhotoUrls: orderShipmentPhotoUrls,
      buyerReceiptPhotoUrls: orderReceiptPhotoUrls,
      onSubmit: (data) async {
        actionLoading.value = true;
        try {
          final result = await ref
              .read(afterSaleRepositoryProvider)
              .resolve(
                c.caseNo,
                resolutionType: data['resolutionType'] as String,
                returnShippingMethod: data['returnShippingMethod'] as String?,
                refundAmount: data['refundAmount'] as String?,
                couponAmount: data['couponAmount'] as String?,
                couponMinSpend: data['couponMinSpend'] as String?,
                couponMessage: data['couponMessage'] as String?,
                rejectReason: data['rejectReason'] as String?,
                rejectMessage: data['rejectMessage'] as String?,
              );
          if (context.mounted) {
            final shipNo = result.returnShipmentNo;
            if (shipNo.isNotEmpty) {
              FloatingToast.show(context, '已提交決策\n退貨單號：$shipNo');
            } else {
              FloatingToast.show(context, '已提交決策');
            }
            await reload();
          }
        } catch (e) {
          if (context.mounted) FloatingToast.show(context, e.toString());
        } finally {
          actionLoading.value = false;
        }
      },
    );
  }

  Widget _imagePlaceholder() => Container(
    width: 48.w,
    height: 48.w,
    color: AppColors.gray100,
    child: Icon(AppIcons.image, size: 24.sp, color: AppIconColors.disabled),
  );

  String _formatDate(String dateStr) {
    if (dateStr.isEmpty) return '';
    final dt = DateTime.tryParse(dateStr);
    if (dt == null) return dateStr;
    final local = dt.toLocal();
    return '${local.year}/${local.month.toString().padLeft(2, '0')}/${local.day.toString().padLeft(2, '0')} ${local.hour.toString().padLeft(2, '0')}:${local.minute.toString().padLeft(2, '0')}';
  }

  String _formatDeadline(String raw) {
    final dt = DateTime.tryParse(raw);
    if (dt == null) return raw;
    final local = dt.toLocal();
    return '${local.month}/${local.day} ${local.hour.toString().padLeft(2, '0')}:${local.minute.toString().padLeft(2, '0')}';
  }
}

// ── Shared widgets ──────────────────────────────────────────────────────────

class _SectionCard extends StatelessWidget {
  const _SectionCard({required this.child});
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: EdgeInsets.all(16.w),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(color: const Color(0x1A42493F)),
      ),
      child: child,
    );
  }
}

class _NoticeCard extends StatelessWidget {
  const _NoticeCard({
    required this.icon,
    required this.title,
    required this.message,
    required this.color,
  });

  final HkIconName icon;
  final String title;
  final String message;
  final Color color;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: EdgeInsets.all(12.w),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.10),
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(color: color.withValues(alpha: 0.28)),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          HkIcon(icon, color: color, size: 20.sp),
          SizedBox(width: 8.w),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  title,
                  style: TextStyle(
                    fontSize: 14.sp,
                    fontWeight: FontWeight.w700,
                    color: color,
                  ),
                ),
                SizedBox(height: 2.h),
                Text(
                  message,
                  style: TextStyle(
                    fontSize: 12.sp,
                    color: AppColors.textSecondary,
                    height: 1.4,
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

class _StatusBadge extends StatelessWidget {
  const _StatusBadge({required this.status, required this.label});
  final String status;
  final String label;

  @override
  Widget build(BuildContext context) {
    final color = switch (status) {
      '0' => AppColors.warningText,
      '20' || '25' => AppColors.info,
      '40' => AppColors.primary,
      '33' || '50' || '71' => AppColors.textSecondary,
      '60' => AppColors.error,
      _ => AppColors.textSecondary,
    };
    return Container(
      padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 4.h),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(12.r),
      ),
      child: Text(
        label,
        style: TextStyle(
          fontSize: 12.sp,
          color: color,
          fontWeight: FontWeight.w700,
        ),
      ),
    );
  }
}

class _LabelValue extends StatelessWidget {
  const _LabelValue({required this.label, required this.value});
  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.only(bottom: 4.h),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 80.w,
            child: Text(
              label,
              style: TextStyle(fontSize: 14.sp, color: AppColors.textSecondary),
            ),
          ),
          Expanded(
            child: Text(
              value,
              style: TextStyle(fontSize: 14.sp, color: AppColors.textEmphasis),
            ),
          ),
        ],
      ),
    );
  }
}

class _ResolutionTile extends StatelessWidget {
  const _ResolutionTile({required this.resolution});
  final AfterSaleResolution resolution;

  @override
  Widget build(BuildContext context) {
    final typeLabel = CodeMetaDisplay.afterSaleResolutionType(
      resolution.resolutionType,
    );
    return Container(
      margin: EdgeInsets.only(bottom: 8.h),
      padding: EdgeInsets.all(12.w),
      decoration: BoxDecoration(
        color: AppColors.bgPrimary,
        borderRadius: BorderRadius.circular(8.r),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            typeLabel,
            style: TextStyle(fontSize: 14.sp, fontWeight: FontWeight.w700),
          ),
          if (resolution.refundAmount.isNotEmpty &&
              resolution.refundAmount != '0')
            Text(
              '退款金額 NT\$ ${resolution.refundAmount}',
              style: TextStyle(fontSize: 12.sp, color: AppColors.textSecondary),
            ),
          if (resolution.couponAmount.isNotEmpty &&
              resolution.couponAmount != '0')
            Text(
              '優惠券 NT\$ ${resolution.couponAmount} (滿 ${resolution.couponMinSpend} 可用)',
              style: TextStyle(fontSize: 12.sp, color: AppColors.textSecondary),
            ),
          if (resolution.rejectReason.isNotEmpty)
            Text(
              '拒絕原因: ${resolution.rejectMessage.isNotEmpty ? resolution.rejectMessage : CodeMetaDisplay.afterSaleRejectReason(resolution.rejectReason)}',
              style: TextStyle(fontSize: 12.sp, color: AppColors.error),
            ),
        ],
      ),
    );
  }
}

// Old _ResolveSheet removed — replaced by after_sales_resolve_sheet.dart
