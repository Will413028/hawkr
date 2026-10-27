import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';

import '../../../config/image_config.dart';
import '../../../shared/widgets/optimized_image.dart';

/// App 內前台通知橫幅
///
/// 當 App 在前台收到推播時，以此 Widget 替代系統通知，
/// 避免 iOS 出現雙 Banner。
///
/// 由 [PushNotificationManager] 透過 [OverlayEntry] 插入至 Navigator overlay，
/// 5 秒後自動消失，也可點擊關閉或點擊正文跳轉。
class NotificationBanner extends StatefulWidget {
  /// 通知標題
  final String title;

  /// 通知內文（可為空，為空時不顯示內文行）
  final String body;

  /// FCM type，用於選擇 icon 與主題色（可選）
  final String? type;

  /// 右側大頭照（買家 / 使用者頭像），來源為 FCM payload 的 imageUrl，
  /// 為空時僅顯示左側 type icon。
  final String? imageUrl;

  /// 點擊橫幅正文時觸發（通常執行頁面跳轉）
  final VoidCallback onTap;

  /// 點擊右上角關閉按鈕時觸發
  final VoidCallback onClose;

  const NotificationBanner({
    super.key,
    required this.title,
    required this.body,
    this.type,
    this.imageUrl,
    required this.onTap,
    required this.onClose,
  });

  @override
  State<NotificationBanner> createState() => _NotificationBannerState();
}

class _NotificationBannerState extends State<NotificationBanner>
    with SingleTickerProviderStateMixin {
  late final AnimationController _enterCtrl;
  late final Animation<double> _opacity;
  late final Animation<double> _slideY;
  late final Animation<double> _scale;

  static const _autoDismissDuration = Duration(seconds: 5);

  @override
  void initState() {
    super.initState();
    _enterCtrl = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 380),
    );

    _opacity = CurvedAnimation(
      parent: _enterCtrl,
      curve: const Interval(0.0, 0.55, curve: Curves.easeOut),
    );
    _slideY = Tween<double>(
      begin: -18,
      end: 0,
    ).animate(CurvedAnimation(parent: _enterCtrl, curve: Curves.easeOutCubic));
    _scale = Tween<double>(
      begin: 0.96,
      end: 1.0,
    ).animate(CurvedAnimation(parent: _enterCtrl, curve: Curves.easeOutCubic));

    _enterCtrl.forward();
  }

  @override
  void dispose() {
    _enterCtrl.dispose();
    super.dispose();
  }

  // ── 根據 type 選擇 icon ───────────────────────────────────────

  IconData get _icon {
    switch (widget.type?.toUpperCase()) {
      case 'CHAT_MESSAGE':
        return AppIcons.chatBubbleOutline;
      case 'ORDER':
      case 'ORDER_PAID':
      case 'ORDER_CREATED':
      case 'ORDER_STATUS':
      case 'ORDER_DELIVERED':
        return AppIcons.receiptLongRounded;
      case 'MERCHANT_VERIFICATION':
      case 'SELLER_REVIEW_STATUS':
      case 'CERTIFICATION_APPROVED':
      case 'CERTIFICATION_REJECTED':
      case 'CERTIFICATION_PENDING_INFO':
      case 'MERCHANT.VERIFICATION.APPROVED':
      case 'MERCHANT.VERIFICATION.REJECTED':
      case 'MERCHANT.VERIFICATION.PENDING_INFO':
        return AppIcons.verifiedRounded;
      case 'COUPON_RECEIVED':
        return AppIcons.localOfferRounded;
      case 'PAYUNI_STATUS':
        return AppIcons.accountBalanceWalletRounded;
      default:
        return AppIcons.notificationsRounded;
    }
  }

  Color get _accentColor {
    switch (widget.type?.toUpperCase()) {
      case 'CHAT_MESSAGE':
        return AppColors.primary;
      case 'MERCHANT_VERIFICATION':
      case 'SELLER_REVIEW_STATUS':
      case 'CERTIFICATION_APPROVED':
      case 'CERTIFICATION_REJECTED':
      case 'CERTIFICATION_PENDING_INFO':
      case 'MERCHANT.VERIFICATION.APPROVED':
      case 'MERCHANT.VERIFICATION.REJECTED':
      case 'MERCHANT.VERIFICATION.PENDING_INFO':
        return AppColors.warning;
      case 'COUPON_RECEIVED':
        return AppColors.primary;
      default:
        return AppColors.primary;
    }
  }

  @override
  Widget build(BuildContext context) {
    final top = MediaQuery.of(context).padding.top;

    return Positioned(
      top: top + 8,
      left: 12,
      right: 12,
      child: AnimatedBuilder(
        animation: _enterCtrl,
        builder: (context, child) => Opacity(
          opacity: _opacity.value,
          child: Transform.translate(
            offset: Offset(0, _slideY.value),
            child: Transform.scale(
              scale: _scale.value,
              alignment: Alignment.topCenter,
              child: child,
            ),
          ),
        ),
        child: Material(
          color: Colors.transparent,
          child: GestureDetector(
            onTap: widget.onTap,
            child: Container(
              decoration: BoxDecoration(
                color: AppColors.bgCard,
                borderRadius: BorderRadius.circular(16),
                boxShadow: [
                  BoxShadow(
                    color: AppColors.cardShadow,
                    blurRadius: 28,
                    spreadRadius: 0,
                    offset: const Offset(0, 8),
                  ),
                  BoxShadow(
                    color: _accentColor.withValues(alpha: 0.14),
                    blurRadius: 18,
                    spreadRadius: -2,
                    offset: const Offset(0, 4),
                  ),
                ],
              ),
              child: ClipRRect(
                borderRadius: BorderRadius.circular(16),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    // ── 主內容 ─────────────────────────────────
                    Padding(
                      padding: const EdgeInsets.fromLTRB(16, 16, 12, 12),
                      child: Row(
                        crossAxisAlignment: CrossAxisAlignment.center,
                        children: [
                          // 左側 icon
                          Container(
                            width: 42,
                            height: 42,
                            decoration: BoxDecoration(
                              color: _accentColor.withValues(alpha: 0.12),
                              borderRadius: BorderRadius.circular(12),
                            ),
                            child: Icon(_icon, color: _accentColor, size: 21),
                          ),
                          const SizedBox(width: 11),
                          // 標題 + 內文
                          Expanded(
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              mainAxisSize: MainAxisSize.min,
                              children: [
                                Text(
                                  widget.title,
                                  style: const TextStyle(
                                    fontSize: 14,
                                    fontWeight: FontWeight.w700,
                                    color: AppColors.textEmphasis,
                                    height: 1.25,
                                  ),
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                ),
                                if (widget.body.isNotEmpty) ...[
                                  const SizedBox(height: 3),
                                  Text(
                                    widget.body,
                                    style: const TextStyle(
                                      fontSize: 14,
                                      color: AppColors.textSecondary,
                                      height: 1.35,
                                    ),
                                    maxLines: 2,
                                    overflow: TextOverflow.ellipsis,
                                  ),
                                ],
                              ],
                            ),
                          ),
                          // 右側大頭照（選填）
                          if (widget.imageUrl != null &&
                              widget.imageUrl!.isNotEmpty) ...[
                            ClipRRect(
                              borderRadius: BorderRadius.circular(8),
                              child: OptimizedNetworkImage(
                                imageUrl: widget.imageUrl!,
                                scene: ImageScene.avatarSmall,
                                width: 36,
                                height: 36,
                                fit: BoxFit.cover,
                                errorWidget: const SizedBox.shrink(),
                              ),
                            ),
                            const SizedBox(width: 6),
                          ],
                          // 關閉按鈕
                          GestureDetector(
                            onTap: widget.onClose,
                            behavior: HitTestBehavior.opaque,
                            child: Padding(
                              padding: const EdgeInsets.fromLTRB(8, 4, 4, 4),
                              child: Icon(
                                AppIcons.closeRounded,
                                size: 18,
                                color: AppIconColors.strong.withValues(
                                  alpha: 0.28,
                                ),
                              ),
                            ),
                          ),
                        ],
                      ),
                    ),
                    // ── 倒數進度條 ─────────────────────────────
                    _CountdownBar(
                      duration: _autoDismissDuration,
                      color: _accentColor,
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

// ── 倒數進度條 ────────────────────────────────────────────────────

/// 橫幅底部的倒數進度條，與 5 秒自動消失同步。
class _CountdownBar extends StatefulWidget {
  final Duration duration;
  final Color color;

  const _CountdownBar({required this.duration, required this.color});

  @override
  State<_CountdownBar> createState() => _CountdownBarState();
}

class _CountdownBarState extends State<_CountdownBar>
    with SingleTickerProviderStateMixin {
  late final AnimationController _ctrl;

  @override
  void initState() {
    super.initState();
    _ctrl = AnimationController(vsync: this, duration: widget.duration)
      ..forward();
  }

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      height: 3,
      child: AnimatedBuilder(
        animation: _ctrl,
        builder: (_, __) => LayoutBuilder(
          builder: (_, constraints) {
            final remaining = (1 - _ctrl.value).clamp(0.0, 1.0);
            return Row(
              children: [
                Container(
                  width: constraints.maxWidth * remaining,
                  height: 3,
                  color: widget.color.withValues(alpha: 0.55),
                ),
              ],
            );
          },
        ),
      ),
    );
  }
}
