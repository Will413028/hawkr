import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/widgets/hk_button_label.dart';
import 'package:flutter/material.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../../config/image_config.dart';
import '../../../../shared/widgets/common_snackbar.dart';
import '../../../../shared/widgets/common_top_bar.dart';
import '../../../../shared/widgets/oss_image.dart';
import '../../../../shared/widgets/truck_loading_overlay.dart';
import '../../data/after_sale_model.dart';
import '../../data/after_sale_order_lookup.dart';
import '../../data/after_sale_repository.dart';
import 'package:hawkr_buyer/shared/widgets/hk_brand_loader.dart';

/// 買家查看售後 — Figma 9860:103515
/// 案件編號 / 訂單編號 / 退貨原因 / 詳細說明 / 上傳照片 / 補充說明輸入框 / 取消售後
class AfterSaleViewScreen extends HookConsumerWidget {
  const AfterSaleViewScreen({super.key, required this.orderSn});
  final String orderSn;

  static final _titleStyle = TextStyle(
    fontSize: 16,
    fontWeight: FontWeight.w700,
    color: AppColors.textPrimary,
  );
  static final _valueStyle = TextStyle(
    fontSize: 14,
    color: AppColors.textPrimary,
  );

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final loading = useState(true);
    final afterSale = useState<AfterSaleCase?>(null);
    final error = useState<String?>(null);
    final cancelling = useState(false);

    Future<void> load() async {
      loading.value = true;
      error.value = null;
      try {
        final result = await ref
            .read(afterSaleRepositoryProvider)
            .getAfterSaleList(pageSize: 100);
        final match = result.records
            .where((c) => c.orderNo == orderSn)
            .toList();
        if (context.mounted && match.isNotEmpty) {
          afterSale.value = match.first;
        }
      } catch (_) {
        if (context.mounted) error.value = '載入失敗';
      } finally {
        if (context.mounted) loading.value = false;
      }
    }

    useEffect(() {
      load();
      return null;
    }, [orderSn]);

    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      appBar: buildCommonTitleAppBar(context, title: '查看售後'),
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
                    style: OutlinedButton.styleFrom(
                      minimumSize: const Size(0, HkControl.minTap),
                    ),
                    child: const HkButtonLabel('重試'),
                  ),
                ],
              ),
            )
          : afterSale.value == null
          ? Center(
              child: Text(
                '尚未提出售後申請',
                style: TextStyle(fontSize: 14, color: AppColors.textSecondary),
              ),
            )
          : _buildContent(context, ref, afterSale.value!, cancelling),
    );
  }

  Widget _buildContent(
    BuildContext context,
    WidgetRef ref,
    AfterSaleCase c,
    ValueNotifier<bool> cancelling,
  ) {
    final reasonLabel = AfterSaleReason.label(c.reason);
    // 優先使用 evidence（新契約）中買家舉證的 imageUrl，
    // fallback 至舊欄位 shipmentPhotoUrls 保持向後相容
    final buyerPhotos = c.evidence
        .where(
          (e) =>
              e.evidenceType == EvidenceType.buyerProof &&
              e.imageUrl.isNotEmpty,
        )
        .map((e) => e.imageUrl)
        .toList();
    final photos = buyerPhotos.isNotEmpty ? buyerPhotos : c.shipmentPhotoUrls;
    // 可取消售後的狀態：待處理(0)
    final canCancel = c.status == AfterSaleStatus.pending;

    return Column(
      children: [
        Expanded(
          child: SingleChildScrollView(
            padding: EdgeInsets.fromLTRB(20.w, 16.h, 20.w, 20.h),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                // 案件編號
                Text('案件編號', style: _titleStyle),
                SizedBox(height: 4.h),
                Text(c.caseNo, style: _valueStyle),
                SizedBox(height: 16.h),

                // 訂單編號
                Text('訂單編號', style: _titleStyle),
                SizedBox(height: 4.h),
                Text(c.orderNo, style: _valueStyle),
                SizedBox(height: 16.h),

                // 退貨原因
                Text('退貨原因', style: _titleStyle),
                SizedBox(height: 4.h),
                Text(
                  reasonLabel.isNotEmpty ? reasonLabel : '-',
                  style: _valueStyle,
                ),
                SizedBox(height: 16.h),

                // 詳細說明（有資料才顯示）
                if (c.description.isNotEmpty) ...[
                  Text('詳細說明', style: _titleStyle),
                  SizedBox(height: 4.h),
                  Text(c.description, style: _valueStyle),
                  SizedBox(height: 16.h),
                ],

                // 上傳照片（有資料才顯示）
                if (photos.isNotEmpty) ...[
                  Text('上傳照片', style: _titleStyle),
                  SizedBox(height: 8.h),
                  Row(
                    children: photos.take(3).toList().asMap().entries.map((e) {
                      return Padding(
                        padding: EdgeInsets.only(
                          right: e.key < photos.take(4).length - 1 ? 12.w : 0,
                        ),
                        child: ClipRRect(
                          borderRadius: BorderRadius.circular(12.r),
                          child: OssImage(
                            imageSource: e.value,
                            scene: ImageScene.reviewPhoto,
                            width: 100.w,
                            height: 100.w,
                            fit: BoxFit.cover,
                          ),
                        ),
                      );
                    }).toList(),
                  ),
                  SizedBox(height: 16.h),
                ],

                // 詳細說明（唯讀灰底框，帶入原本提交的文字，有資料才顯示）
                if (c.description.isNotEmpty) ...[
                  RichText(
                    text: TextSpan(
                      children: [
                        TextSpan(text: '詳細說明 ', style: _titleStyle),
                        TextSpan(
                          text: '*',
                          style: TextStyle(
                            fontSize: 16,
                            fontWeight: FontWeight.w700,
                            color: AppColors.danger,
                          ),
                        ),
                      ],
                    ),
                  ),
                  SizedBox(height: 8.h),
                  Container(
                    width: double.infinity,
                    padding: EdgeInsets.all(16.w),
                    decoration: BoxDecoration(
                      color: AppColors.gray100.withValues(alpha: 0.5),
                      borderRadius: BorderRadius.circular(12.r),
                    ),
                    constraints: BoxConstraints(minHeight: 100.h),
                    child: Text(
                      c.description,
                      style: TextStyle(
                        fontSize: 14,
                        color: AppColors.textEmphasis,
                      ),
                    ),
                  ),
                ],
              ],
            ),
          ),
        ),

        // 取消售後按鈕（僅待處理狀態）
        if (canCancel)
          SafeArea(
            top: false,
            child: Padding(
              padding: EdgeInsets.fromLTRB(20.w, 8.h, 20.w, 16.h),
              child: SizedBox(
                width: double.infinity,
                height: HkControl.minTap,
                child: ElevatedButton(
                  onPressed: cancelling.value
                      ? null
                      : () => _cancelAfterSale(
                          context,
                          ref,
                          c.caseNo,
                          cancelling,
                        ),
                  style: ElevatedButton.styleFrom(
                    backgroundColor: AppColors.primary,
                    disabledBackgroundColor: AppColors.primaryLight,
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(999.r),
                    ),
                  ),
                  child: cancelling.value
                      ? SizedBox(
                          width: 20.sp,
                          height: 20.sp,
                          child: const HkBrandLoader(
                            tone: HkBrandLoaderTone.mono,
                            color: AppColors.bgCard,
                          ),
                        )
                      : HkButtonLabel(
                          '取消售後',
                          style: TextStyle(
                            fontSize: 16,
                            fontWeight: FontWeight.w700,
                            color: AppColors.bgCard,
                          ),
                        ),
                ),
              ),
            ),
          ),
      ],
    );
  }

  Future<void> _cancelAfterSale(
    BuildContext context,
    WidgetRef ref,
    String caseNo,
    ValueNotifier<bool> cancelling,
  ) async {
    cancelling.value = true;
    try {
      await ref.read(afterSaleRepositoryProvider).cancelAfterSale(caseNo);
      if (context.mounted) {
        ref.read(afterSaleOrderLookupProvider.notifier).removeOrderNo(orderSn);
        showFloatingToast(context, '已取消售後');
        context.pop();
      }
    } catch (e) {
      if (context.mounted) {
        showFloatingToast(context, '取消失敗', isSuccess: false);
      }
    } finally {
      cancelling.value = false;
    }
  }
}
