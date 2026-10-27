import '../cart_typography.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:hawkr_buyer/shared/widgets/hk_button_label.dart';
import 'package:flutter/material.dart';

/// Opens the existing checkout review sheet by tap or upward swipe.
/// No order is created by this control.
class CartCheckoutAction extends StatefulWidget {
  const CartCheckoutAction({
    super.key,
    required this.onPressed,
    this.storeName,
    this.selectedItemCount,
    this.isValidating = false,
  });
  final VoidCallback? onPressed;
  final String? storeName;
  final int? selectedItemCount;
  final bool isValidating;

  @override
  State<CartCheckoutAction> createState() => _CartCheckoutActionState();
}

class _CartCheckoutActionState extends State<CartCheckoutAction> {
  double _dragDistance = 0;
  bool _openedBySwipe = false;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.fromLTRB(
      HkSpace.lg,
      HkSpace.sm,
      HkSpace.lg,
      HkSpace.xxl,
    ),
    child: GestureDetector(
      onVerticalDragStart: (_) {
        _dragDistance = 0;
        _openedBySwipe = false;
      },
      onVerticalDragUpdate: (details) {
        _dragDistance += details.delta.dy;
        if (_dragDistance < -HkSpace.xxl &&
            !_openedBySwipe &&
            widget.onPressed != null &&
            !widget.isValidating) {
          _openedBySwipe = true;
          widget.onPressed?.call();
        }
      },
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          if (widget.selectedItemCount != null) ...[
            Semantics(
              liveRegion: true,
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    widget.storeName == null ? '請先選取一家店的商品' : '本次結帳商店',
                    style: CartTypography.supportMuted,
                  ),
                  if (widget.storeName != null) ...[
                    Text(
                      widget.storeName!,
                      key: const ValueKey('cart-checkout-store'),
                      style: CartTypography.subtitle,
                    ),
                    Text(
                      widget.isValidating
                          ? '正在確認商品與配送方式…'
                          : '已選 ${widget.selectedItemCount} 項商品',
                      style: CartTypography.supportMuted,
                    ),
                  ],
                ],
              ),
            ),
            const SizedBox(height: HkSpace.md),
          ],
          SizedBox(
            width: double.infinity,
            child: FilledButton.icon(
              key: const ValueKey('cart-checkout-action'),
              style: FilledButton.styleFrom(textStyle: CartTypography.button),
              onPressed: widget.isValidating ? null : widget.onPressed,
              icon: const Icon(AppIcons.keyboardArrowUp, size: HkControl.icon),
              label: const HkButtonLabel('前往結帳'),
            ),
          ),
        ],
      ),
    ),
  );
}
