import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

/// iOS-style swipe-to-reveal action button.
///
/// Swipe left to reveal a styled action button (default: delete).
/// Tap the button to trigger [onAction]. Full-swipe is disabled —
/// only the revealed button can trigger the action.
class IosSwipeAction extends StatefulWidget {
  final Widget child;
  final VoidCallback onAction;

  /// Width of the revealed action area.
  final double actionWidth;

  /// Label shown under the icon.
  final String actionLabel;

  /// Icon displayed in the action area.
  final IconData actionIcon;

  /// Background color of the action area.
  final Color actionColor;

  const IosSwipeAction({
    super.key,
    required this.child,
    required this.onAction,
    this.actionWidth = 72,
    this.actionLabel = '刪除',
    this.actionIcon = AppIcons.deleteOutlineRounded,
    this.actionColor = AppColors.danger,
  });

  @override
  State<IosSwipeAction> createState() => _IosSwipeActionState();
}

class _IosSwipeActionState extends State<IosSwipeAction>
    with SingleTickerProviderStateMixin {
  late final AnimationController _controller;
  double _dragExtent = 0;
  bool _isOpen = false;
  bool _isBeingDragged = false;

  @override
  void initState() {
    super.initState();
    _controller = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 250),
    );
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  void _close() {
    _isOpen = false;
    _controller.animateTo(0, curve: Curves.easeOut);
  }

  void _snapOpen() {
    _isOpen = true;
    _controller.animateTo(1, curve: Curves.easeOut);
  }

  void _onDragStart(DragStartDetails details) {
    _isBeingDragged = true;
    _dragExtent = _controller.value * widget.actionWidth;
  }

  void _onDragUpdate(DragUpdateDetails details) {
    setState(() {
      _dragExtent -= details.delta.dx;
      // 限制最大滑動距離為 actionWidth，不允許超出
      _dragExtent = _dragExtent.clamp(0.0, widget.actionWidth * 1.1);
      _controller.value = _dragExtent / widget.actionWidth;
    });
  }

  void _onDragEnd(DragEndDetails details) {
    _isBeingDragged = false;
    final velocity = details.primaryVelocity ?? 0;

    // 快速向左滑 → 展開
    if (velocity < -300 && !_isOpen) {
      _snapOpen();
      return;
    }

    // 快速向右滑 → 收合
    if (velocity > 300 && _isOpen) {
      _close();
      return;
    }

    // 根據滑動距離決定展開或收合
    if (_dragExtent > widget.actionWidth * 0.4) {
      _snapOpen();
    } else {
      _close();
    }
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: _controller,
      builder: (context, _) {
        final offset = _controller.isAnimating || !_isBeingDragged
            ? _controller.value * widget.actionWidth
            : _dragExtent;

        final clampedOffset = offset.clamp(0.0, widget.actionWidth);
        final progress = (clampedOffset / widget.actionWidth).clamp(0.0, 1.0);

        return GestureDetector(
          onHorizontalDragStart: _onDragStart,
          onHorizontalDragUpdate: _onDragUpdate,
          onHorizontalDragEnd: _onDragEnd,
          child: ClipRect(
            child: Stack(
              children: [
                // ── 右側刪除按鈕 ──
                if (offset > 0)
                  Positioned.fill(
                    child: Row(
                      children: [
                        const Spacer(),
                        GestureDetector(
                          behavior: HitTestBehavior.opaque,
                          onTap: () {
                            _close();
                            widget.onAction();
                          },
                          child: SizedBox(
                            width: clampedOffset,
                            child: Opacity(
                              opacity: progress,
                              child: Center(
                                child: Icon(
                                  widget.actionIcon,
                                  color: widget.actionColor,
                                  size: 24.sp,
                                ),
                              ),
                            ),
                          ),
                        ),
                      ],
                    ),
                  ),

                // ── 滑動內容 ──
                Transform.translate(
                  offset: Offset(-offset, 0),
                  child: widget.child,
                ),
              ],
            ),
          ),
        );
      },
    );
  }
}
