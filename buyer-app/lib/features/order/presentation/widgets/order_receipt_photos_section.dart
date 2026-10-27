import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../../config/image_config.dart';
import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_typography.dart';
import '../../../../core/theme/hk_tokens.dart';
import '../../../../shared/icons/app_icon_colors.dart';
import '../../../../shared/widgets/hk_icon.dart';
import '../../../../shared/widgets/oss_image.dart';
import '../../data/order_model.dart';
import '../receipt_photo_controller.dart';

class OrderReceiptPhotosSection extends ConsumerWidget {
  const OrderReceiptPhotosSection({super.key, required this.order});

  final Order order;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final receiptState = ref.watch(
      receiptPhotoControllerProvider(
        orderSn: order.orderSn,
        backendUrls: order.receiptPhotoUrls,
      ),
    );
    final receiptPhotos = receiptState.value?.photos ?? const <String>[];
    final shipmentPhotos = order.shipmentPhotoUrls;
    final hasShipment = shipmentPhotos.isNotEmpty;
    final hasReceipt = receiptPhotos.isNotEmpty;
    if (!hasShipment && !hasReceipt) return const SizedBox.shrink();

    return Padding(
      key: const ValueKey('order-receipt-photos-section'),
      padding: const EdgeInsets.only(top: HkSpace.lg),
      child: Container(
        decoration: BoxDecoration(
          color: AppColors.bgCard.withValues(alpha: 0.94),
          borderRadius: BorderRadius.circular(HkRadius.lg),
          border: Border.all(color: AppColors.cardBorder),
          boxShadow: HkShadows.card,
        ),
        padding: EdgeInsets.all(16.w),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const _SectionTitle(),
            SizedBox(height: 12.h),
            Row(
              mainAxisAlignment: hasShipment && hasReceipt
                  ? MainAxisAlignment.center
                  : MainAxisAlignment.start,
              crossAxisAlignment: CrossAxisAlignment.center,
              children: [
                if (hasShipment)
                  _PhotoColumn(label: '賣家出貨照片', imageUrls: shipmentPhotos),
                if (hasShipment && hasReceipt) ...[
                  SizedBox(width: 10.w),
                  const _VsBadge(),
                  SizedBox(width: 10.w),
                ],
                if (hasReceipt)
                  _PhotoColumn(label: '買家收貨照片', imageUrls: receiptPhotos),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _SectionTitle extends StatelessWidget {
  const _SectionTitle();

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        HkIcon(
          HkIconName.photoAdd,
          size: 20.sp,
          color: AppIconColors.defaultColor,
        ),
        const SizedBox(width: HkSpace.sm),
        Expanded(
          child: Text(
            '買賣家上傳照片',
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: AppTypography.sectionTitle,
          ),
        ),
      ],
    );
  }
}

class _PhotoColumn extends StatelessWidget {
  const _PhotoColumn({required this.label, required this.imageUrls});

  final String label;
  final List<String> imageUrls;

  @override
  Widget build(BuildContext context) {
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(
          label,
          style: AppTypography.meta.copyWith(color: AppColors.textEmphasis),
        ),
        SizedBox(height: 8.h),
        Container(
          width: 80.w,
          height: 80.w,
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(12.r),
            border: Border.all(
              color: AppColors.textPrimary.withValues(alpha: 0.5),
            ),
          ),
          clipBehavior: Clip.antiAlias,
          child: OssImage(
            imageSource: imageUrls.first,
            scene: ImageScene.reviewPhoto,
            width: 80.w,
            height: 80.w,
            fit: BoxFit.cover,
          ),
        ),
      ],
    );
  }
}

class _VsBadge extends StatelessWidget {
  const _VsBadge();

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 36.w,
      height: 36.w,
      alignment: Alignment.center,
      decoration: const BoxDecoration(
        color: AppColors.primary,
        shape: BoxShape.circle,
      ),
      child: Text(
        'vs',
        style: AppTypography.sectionTitle.copyWith(
          height: 1.23,
          color: AppColors.bgCard,
        ),
      ),
    );
  }
}
