import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:hawkr_buyer/shared/widgets/hk_button_label.dart';
import 'package:hawkr_api/gen/hawkr/content/v1alpha1/content.pb.dart' as api;
import 'package:hawkr_api/images.dart' as api_image;
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:url_launcher/url_launcher.dart';

import '../../../../config/image_config.dart';
import '../../../../core/motion/hk_motion_transitions.dart';
import '../../../../shared/widgets/app_cached_image.dart';
import '../../../product/presentation/merchant_item_detail_screen.dart';

class BannerAction {
  const BannerAction({this.type, this.url, this.id, this.buttonText});

  final String? type; // 'popup' | 'url' | 'product' | 'post'
  final String? url;
  final String? id;
  final String? buttonText;

  /// ContentPublicService 的 `BannerAction`（oneof）。沒有設定時沒有行為。
  factory BannerAction.fromProto(
    api.BannerAction action, {
    String? buttonText,
  }) {
    return switch (action.whichKind()) {
      api.BannerAction_Kind.popup => BannerAction(
        type: 'popup',
        buttonText: buttonText,
      ),
      api.BannerAction_Kind.url => BannerAction(
        type: 'url',
        url: action.url,
        buttonText: buttonText,
      ),
      api.BannerAction_Kind.productId => BannerAction(
        type: 'product',
        id: action.productId,
        buttonText: buttonText,
      ),
      api.BannerAction_Kind.postId => BannerAction(
        type: 'post',
        id: action.postId,
        buttonText: buttonText,
      ),
      api.BannerAction_Kind.notSet => BannerAction(buttonText: buttonText),
    };
  }

  bool get hasCta {
    switch (type) {
      case 'url':
        return (url ?? '').isNotEmpty;
      case 'product':
      case 'post':
        return (id ?? '').isNotEmpty;
      default:
        return false;
    }
  }

  String get defaultLabel {
    switch (type) {
      case 'url':
        return '查看詳情';
      case 'product':
        return '立即購買';
      case 'post':
        return '查看貼文';
      default:
        return '';
    }
  }

  String get label {
    final custom = (buttonText ?? '').trim();
    return custom.isNotEmpty ? custom : defaultLabel;
  }
}

Future<void> showBannerPopupDialog(
  BuildContext context, {
  required api_image.Image popupImage,
  BannerAction action = const BannerAction(),
}) {
  return showGeneralDialog(
    context: context,
    barrierDismissible: true,
    barrierLabel: 'Banner Popup',
    barrierColor: AppColors.textEmphasis.withValues(alpha: 0.5),
    transitionDuration: HkMotionTransitions.durationOf(context),
    pageBuilder: (_, __, ___) =>
        _BannerPopupDialog(popupImage: popupImage, action: action),
    transitionBuilder: (transitionContext, animation, __, child) {
      return HkMotionTransitions.modal(
        context: transitionContext,
        animation: animation,
        presentation: HkModalPresentation.popup,
        child: child,
      );
    },
  );
}

class _BannerPopupDialog extends StatelessWidget {
  const _BannerPopupDialog({required this.popupImage, required this.action});

  final api_image.Image popupImage;
  final BannerAction action;

  void _handleCta(BuildContext context) {
    Navigator.of(context).pop();
    switch (action.type) {
      case 'url':
        final url = action.url;
        if (url != null && url.isNotEmpty) {
          final uri = Uri.tryParse(url);
          if (uri != null && (uri.scheme == 'https' || uri.scheme == 'http')) {
            launchUrl(uri, mode: LaunchMode.externalApplication);
          }
        }
        return;
      case 'product':
        final id = action.id;
        if (id != null && id.isNotEmpty) {
          context.push(
            '/merchant-product',
            extra: MerchantItemDetailScreen(skuId: id),
          );
        }
        return;
      case 'post':
        final id = action.id;
        if (id != null && id.isNotEmpty) {
          context.push('/social/post/$id');
        }
        return;
    }
  }

  @override
  Widget build(BuildContext context) {
    final showCta = action.hasCta;

    return SafeArea(
      minimum: const EdgeInsets.all(HkSpace.lg),
      child: LayoutBuilder(
        builder: (context, constraints) {
          final dialogWidth = constraints.maxWidth > 360
              ? 360.0
              : constraints.maxWidth;
          // Figma: 360 × 404 (ratio 360/404 ≈ 0.891).
          final dialogHeight = dialogWidth * 404 / 360;

          return SingleChildScrollView(
            key: const ValueKey('banner-popup-scroll'),
            child: ConstrainedBox(
              constraints: BoxConstraints(minHeight: constraints.maxHeight),
              child: Center(
                child: SizedBox(
                  width: dialogWidth,
                  child: Material(
                    color: Colors.transparent,
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Align(
                          alignment: Alignment.centerRight,
                          child: IconButton(
                            key: const ValueKey('banner-popup-close'),
                            constraints: const BoxConstraints.tightFor(
                              width: HkControl.minTap,
                              height: HkControl.minTap,
                            ),
                            onPressed: () => Navigator.of(context).pop(),
                            icon: const Icon(
                              AppIcons.close,
                              color: AppIconColors.onBrand,
                              size: 28,
                            ),
                            tooltip: '關閉',
                          ),
                        ),
                        const SizedBox(height: HkSpace.xs),
                        ClipRRect(
                          borderRadius: BorderRadius.circular(HkRadius.lg),
                          child: SizedBox(
                            key: const ValueKey('banner-popup-image'),
                            width: dialogWidth,
                            height: dialogHeight,
                            child: AppCachedImage.api(
                              image: popupImage,
                              scene: ImageScene.homeBannerPopup,
                              fit: BoxFit.cover,
                              errorWidget: Container(
                                color: AppColors.primarySoft,
                                child: const Icon(
                                  AppIcons.imageNotSupported,
                                  size: HkControl.minTap,
                                  color: AppIconColors.muted,
                                ),
                              ),
                            ),
                          ),
                        ),
                        if (showCta) ...[
                          const SizedBox(height: HkSpace.lg),
                          SizedBox(
                            width: dialogWidth,
                            height: HkControl.minTap,
                            child: ElevatedButton(
                              key: const ValueKey('banner-popup-cta'),
                              onPressed: () => _handleCta(context),
                              style: ElevatedButton.styleFrom(
                                minimumSize: const Size.fromHeight(
                                  HkControl.minTap,
                                ),
                                padding: const EdgeInsets.symmetric(
                                  horizontal: HkSpace.lg,
                                ),
                                backgroundColor: AppColors.bgCard,
                                foregroundColor: AppColors.brandPrimaryStrong,
                                shape: const StadiumBorder(),
                                elevation: 0,
                              ),
                              child: HkButtonLabel(
                                action.label,
                                style: AppTypography.buttonMedium.copyWith(
                                  color: AppColors.brandPrimaryStrong,
                                ),
                              ),
                            ),
                          ),
                        ],
                      ],
                    ),
                  ),
                ),
              ),
            ),
          );
        },
      ),
    );
  }
}
