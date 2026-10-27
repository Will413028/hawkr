import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import '../../features/cart/presentation/cart_controller.dart';
import '../utils/require_login.dart';
import '../widgets/hk_icon.dart';
import 'notification_count_badge.dart';

/// 帶有數量徽章的購物車圖標組件（加入購物車時有彈跳動畫）
class CartIconWithBadge extends HookConsumerWidget {
  const CartIconWithBadge({
    super.key,
    this.iconSize = 24.0,
    this.badgeColor = AppColors.danger,
    this.badgeTextColor = AppColors.bgCard,
    this.showBadgeWhenZero = false,
    this.compact = false,
  });

  final double iconSize;
  final Color badgeColor;
  final Color badgeTextColor;
  final bool showBadgeWhenZero;

  /// 緊湊模式：移除 IconButton 的預設 padding
  final bool compact;

  Future<void> _navigateToCart(BuildContext context, WidgetRef ref) async {
    if (!await requireLogin(context, ref)) return;
    if (!context.mounted) return;
    await context.push('/cart');
    ref.invalidate(cartCountProvider);
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // 彈跳動畫：數量增加時 scale 1.0 → 1.35 → 1.0
    final animCtrl = useAnimationController(
      duration: const Duration(milliseconds: 400),
    );
    final scaleAnim = useMemoized(
      () => TweenSequence<double>([
        TweenSequenceItem(
          tween: Tween(
            begin: 1.0,
            end: 1.35,
          ).chain(CurveTween(curve: Curves.easeOut)),
          weight: 40,
        ),
        TweenSequenceItem(
          tween: Tween(
            begin: 1.35,
            end: 1.0,
          ).chain(CurveTween(curve: Curves.elasticOut)),
          weight: 60,
        ),
      ]).animate(animCtrl),
      [animCtrl],
    );

    final itemCount = ref.watch(cartCountProvider).value ?? 0;
    final shouldShowBadge = showBadgeWhenZero || itemCount > 0;
    Future<void> handleTap() async {
      await _navigateToCart(context, ref);
    }

    // 追踪上次數量，數量增加時觸發動畫
    final prevCount = useRef<int?>(null);
    useEffect(() {
      if (prevCount.value != null && itemCount > prevCount.value!) {
        animCtrl.forward(from: 0);
      }
      prevCount.value = itemCount;
      return null;
    }, [itemCount]);

    // 用動畫包裹整個圖標
    Widget wrapWithAnimation(Widget child) {
      return AnimatedBuilder(
        animation: scaleAnim,
        builder: (_, c) => Transform.scale(scale: scaleAnim.value, child: c),
        child: child,
      );
    }

    // 緊湊模式：使用 GestureDetector + Icon，不帶 padding
    if (compact) {
      return wrapWithAnimation(
        Semantics(
          label: itemCount > 0 ? '購物車，$itemCount 件商品' : '購物車',
          button: true,
          onTap: handleTap,
          child: ExcludeSemantics(
            child: GestureDetector(
              behavior: HitTestBehavior.opaque,
              onTap: handleTap,
              child: SizedBox.square(
                dimension: 48,
                child: Stack(
                  clipBehavior: Clip.none,
                  alignment: Alignment.center,
                  children: [
                    HkIcon(
                      HkIconName.cart,
                      key: const ValueKey('topbar-cart-icon'),
                      size: iconSize,
                    ),
                    if (shouldShowBadge)
                      Positioned(
                        right: 3,
                        top: 3,
                        child: NotificationCountBadge(count: itemCount),
                      ),
                  ],
                ),
              ),
            ),
          ),
        ),
      );
    }

    // 標準模式：使用 IconButton
    return wrapWithAnimation(
      Stack(
        clipBehavior: Clip.none,
        children: [
          IconButton(
            tooltip: '購物車',
            icon: HkIcon(
              HkIconName.cart,
              key: const ValueKey('topbar-cart-icon'),
              size: iconSize,
            ),
            onPressed: handleTap,
          ),
          if (shouldShowBadge)
            Positioned(
              right: 4,
              top: 4,
              child: GestureDetector(
                behavior: HitTestBehavior.opaque,
                onTap: handleTap,
                child: NotificationCountBadge(count: itemCount),
              ),
            ),
        ],
      ),
    );
  }
}
