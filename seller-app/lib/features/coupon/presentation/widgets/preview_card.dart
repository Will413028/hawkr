import 'dart:typed_data';

import 'package:hawkr_seller/config/image_config.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/core/theme/hk_tokens.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/widgets/hk_frosted_card_surface.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:hawkr_seller/shared/widgets/optimized_image.dart';
import 'package:flutter/material.dart';

/// 買家實際收到優惠券前的即時預覽。
///
/// 預覽只呈現買家需要理解的內容：優惠、使用門檻與期限。編輯狀態則由
/// 上方「即時預覽 / 草稿」標記承接，避免舊版懸掛徽章搶走表單焦點。
class CouponPreviewCard extends StatelessWidget {
  const CouponPreviewCard({
    super.key,
    required this.nameCtrl,
    required this.discountCtrl,
    required this.minPurchaseCtrl,
    required this.discountRule,
    required this.noExpiry,
    required this.hasExpiry,
    required this.validUntil,
    this.photoBytes,
    this.photoUrl,
    this.onPickPhoto,
  });

  final TextEditingController nameCtrl;
  final TextEditingController discountCtrl;
  final TextEditingController minPurchaseCtrl;
  final String discountRule;
  final bool noExpiry;
  final bool hasExpiry;
  final DateTime validUntil;
  final Uint8List? photoBytes;
  final String? photoUrl;
  final VoidCallback? onPickPhoto;

  @override
  Widget build(BuildContext context) {
    final title = nameCtrl.text.trim().isEmpty
        ? '為優惠券取一個好記的名稱'
        : nameCtrl.text.trim();
    final discount = discountCtrl.text.trim();
    final minPurchase = minPurchaseCtrl.text.trim();
    final isPercent = discountRule == '折數折抵';

    final discountValue = discount.isEmpty
        ? (isPercent ? '—%' : 'NT\$ —')
        : (isPercent ? '$discount% OFF' : '折 NT\$ $discount');
    final threshold = minPurchase.isEmpty || minPurchase == '0'
        ? '不限最低消費'
        : '滿 NT\$ $minPurchase 可用';
    final expiry = noExpiry
        ? '無使用期限'
        : hasExpiry
        ? '有效至 ${_formatDate(validUntil)}'
        : '尚未設定期限';

    return Semantics(
      container: true,
      label: '優惠券即時預覽，$title，$discountValue，$threshold，$expiry',
      child: HkFrostedCardSurface(
        child: Padding(
          padding: const EdgeInsets.all(HkSpace.lg),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Row(children: [_PreviewLabel(), Spacer(), _DraftBadge()]),
              const SizedBox(height: HkSpace.md),
              Container(
                width: double.infinity,
                padding: const EdgeInsets.all(HkSpace.lg),
                decoration: BoxDecoration(
                  gradient: const LinearGradient(
                    begin: Alignment.topLeft,
                    end: Alignment.bottomRight,
                    colors: [
                      AppColors.brandPrimaryStrong,
                      AppColors.primaryDark,
                    ],
                  ),
                  borderRadius: BorderRadius.circular(HkRadius.lg),
                  border: Border.all(
                    color: AppColors.neutralWhite.withValues(alpha: 0.14),
                  ),
                  boxShadow: HkShadows.sm,
                ),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        _PhotoPicker(
                          photoBytes: photoBytes,
                          photoUrl: photoUrl,
                          onTap: onPickPhoto,
                        ),
                        const SizedBox(width: HkSpace.md),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(
                                title,
                                maxLines: 2,
                                overflow: TextOverflow.ellipsis,
                                style: const TextStyle(
                                  color: AppColors.textInverse,
                                  fontSize: 18,
                                  height: 1.35,
                                  fontWeight: FontWeight.w700,
                                ),
                              ),
                              const SizedBox(height: HkSpace.sm),
                              Text(
                                discountValue,
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                                style: const TextStyle(
                                  color: AppColors.brandSecondarySoft,
                                  fontSize: 24,
                                  height: 1.15,
                                  fontWeight: FontWeight.w900,
                                ),
                              ),
                            ],
                          ),
                        ),
                      ],
                    ),
                    const SizedBox(height: HkSpace.lg),
                    Divider(
                      height: 1,
                      color: AppColors.neutralWhite.withValues(alpha: 0.18),
                    ),
                    const SizedBox(height: HkSpace.md),
                    Wrap(
                      spacing: HkSpace.sm,
                      runSpacing: HkSpace.sm,
                      children: [
                        _PreviewMeta(icon: HkIconName.wallet, label: threshold),
                        _PreviewMeta(icon: HkIconName.calendar, label: expiry),
                      ],
                    ),
                  ],
                ),
              ),
              const SizedBox(height: HkSpace.md),
              const Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  HkIcon(HkIconName.info, size: 18, color: AppIconColors.muted),
                  SizedBox(width: HkSpace.sm),
                  Expanded(
                    child: Text(
                      '內容會隨下方設定即時更新；儲存前不會發送給顧客。',
                      style: TextStyle(
                        color: AppColors.textMuted,
                        fontSize: 14,
                        height: 1.45,
                      ),
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }

  static String _formatDate(DateTime date) =>
      '${date.year}/${date.month.toString().padLeft(2, '0')}/${date.day.toString().padLeft(2, '0')}';
}

class _PreviewLabel extends StatelessWidget {
  const _PreviewLabel();

  @override
  Widget build(BuildContext context) {
    return const Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        HkIcon(HkIconName.coupon, size: 20, color: AppIconColors.active),
        SizedBox(width: HkSpace.sm),
        Text(
          '即時票券預覽',
          style: TextStyle(
            color: AppColors.textEmphasis,
            fontSize: 16,
            fontWeight: FontWeight.w700,
          ),
        ),
      ],
    );
  }
}

class _DraftBadge extends StatelessWidget {
  const _DraftBadge();

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(
        horizontal: HkSpace.md,
        vertical: HkSpace.xs,
      ),
      decoration: BoxDecoration(
        color: AppColors.warningSoft,
        borderRadius: BorderRadius.circular(HkRadius.pill),
        border: Border.all(
          color: AppColors.brandSecondary.withValues(alpha: 0.42),
        ),
      ),
      child: const Text(
        '草稿',
        style: TextStyle(
          color: AppColors.onWarning,
          fontSize: 12,
          fontWeight: FontWeight.w700,
        ),
      ),
    );
  }
}

class _PreviewMeta extends StatelessWidget {
  const _PreviewMeta({required this.icon, required this.label});

  final HkIconName icon;
  final String label;

  @override
  Widget build(BuildContext context) {
    return Container(
      constraints: const BoxConstraints(minHeight: 32),
      padding: const EdgeInsets.symmetric(
        horizontal: HkSpace.md,
        vertical: HkSpace.sm,
      ),
      decoration: BoxDecoration(
        color: AppColors.neutralWhite.withValues(alpha: 0.10),
        borderRadius: BorderRadius.circular(HkRadius.pill),
        border: Border.all(
          color: AppColors.neutralWhite.withValues(alpha: 0.12),
        ),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          HkIcon(icon, size: 16, color: AppIconColors.onBrand),
          const SizedBox(width: HkSpace.xs),
          Flexible(
            child: Text(
              label,
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(
                color: AppColors.textInverse,
                fontSize: 12,
                fontWeight: FontWeight.w500,
                height: 1.3,
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _PhotoPicker extends StatelessWidget {
  const _PhotoPicker({this.photoBytes, this.photoUrl, this.onTap});

  final Uint8List? photoBytes;
  final String? photoUrl;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    final hasBytes = photoBytes != null && photoBytes!.isNotEmpty;
    final hasUrl = !hasBytes && (photoUrl?.isNotEmpty ?? false);

    final Widget content;
    if (hasBytes) {
      content = Image.memory(
        photoBytes!,
        fit: BoxFit.cover,
        gaplessPlayback: true,
      );
    } else if (hasUrl) {
      content = OptimizedNetworkImage(
        imageUrl: photoUrl!,
        scene: ImageScene.couponImage,
        fit: BoxFit.cover,
        errorWidget: const Center(
          child: HkIcon(HkIconName.photoAdd, color: AppIconColors.active),
        ),
      );
    } else {
      content = const Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            HkIcon(HkIconName.photoAdd, size: 24, color: AppIconColors.active),
            SizedBox(height: HkSpace.xs),
            Text(
              '封面',
              style: TextStyle(
                color: AppColors.brandPrimaryStrong,
                fontSize: 12,
                fontWeight: FontWeight.w700,
              ),
            ),
          ],
        ),
      );
    }

    return Semantics(
      button: true,
      label: hasBytes || hasUrl ? '更換優惠券封面' : '新增優惠券封面',
      child: Tooltip(
        message: hasBytes || hasUrl ? '更換優惠券封面' : '新增優惠券封面',
        child: Material(
          color: Colors.transparent,
          child: InkWell(
            onTap: onTap,
            borderRadius: BorderRadius.circular(HkRadius.lg),
            child: SizedBox.square(
              dimension: 72,
              child: Stack(
                clipBehavior: Clip.none,
                children: [
                  Positioned.fill(
                    child: DecoratedBox(
                      decoration: BoxDecoration(
                        color: AppColors.surfaceCream,
                        borderRadius: BorderRadius.circular(HkRadius.lg),
                        border: Border.all(
                          color: AppColors.neutralWhite.withValues(alpha: 0.84),
                        ),
                      ),
                      child: ClipRRect(
                        borderRadius: BorderRadius.circular(HkRadius.lg - 1),
                        child: content,
                      ),
                    ),
                  ),
                  Positioned(
                    right: -4,
                    bottom: -4,
                    child: DecoratedBox(
                      decoration: BoxDecoration(
                        color: AppColors.brandSecondary,
                        shape: BoxShape.circle,
                        border: Border.all(
                          color: AppColors.brandPrimaryStrong,
                          width: 2,
                        ),
                      ),
                      child: const SizedBox.square(
                        dimension: 28,
                        child: Center(
                          child: HkIcon(
                            HkIconName.edit,
                            size: 15,
                            color: AppIconColors.active,
                          ),
                        ),
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}
