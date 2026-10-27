import 'package:flutter/material.dart';

import '../../core/motion/hk_motion_transitions.dart';

/// 骨架屏的掃光層，移植自網頁 `styles/components/route-loading-skeleton.css`。
///
/// 網頁只在「整頁骨架屏」用掃光，單張圖片不用（圖片走 [AppCachedImage] 的
/// 靜態佔位）。參數與網頁 `routeLoadingSkeletonShimmer` 對齊：1.6s、
/// `cubic-bezier(0.4, 0, 0.2, 1)`、100deg 的柔和漸層、白色 42% 高光、
/// 由 -120% 掃到 120%。
///
/// 先前 App 內有四份各自手寫的版本，都是 1100ms + 白色 80% 的窄光帶——
/// 正是網頁註解裡「read as a harsh strobe」而被換掉的那一版。
class HkSkeletonShimmer extends StatefulWidget {
  const HkSkeletonShimmer({super.key, required this.child});

  final Widget child;

  /// 對應網頁 `.route-loading-skeleton *::after` 的 `animation` 長度。
  static const Duration duration = Duration(milliseconds: 1600);

  /// 對應 `cubic-bezier(0.4, 0, 0.2, 1)`。
  static const Curve curve = Cubic(0.4, 0, 0.2, 1);

  /// 對應 `--skeleton-sheen` 的淺色值 `rgba(255, 255, 255, 0.42)`。
  static const Color sheen = Color(0x6BFFFFFF);

  @override
  State<HkSkeletonShimmer> createState() => _HkSkeletonShimmerState();
}

class _HkSkeletonShimmerState extends State<HkSkeletonShimmer>
    with SingleTickerProviderStateMixin {
  late final AnimationController _controller;
  late final Animation<double> _sweep;
  bool? _reduceMotion;

  @override
  void initState() {
    super.initState();
    _controller = AnimationController(
      vsync: this,
      duration: HkSkeletonShimmer.duration,
    );
    _sweep = CurvedAnimation(
      parent: _controller,
      curve: HkSkeletonShimmer.curve,
    );
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final reduceMotion = HkMotionTransitions.reduceMotionOf(context);
    if (_reduceMotion == reduceMotion) return;
    _reduceMotion = reduceMotion;
    if (reduceMotion) {
      _controller.stop();
    } else if (!_controller.isAnimating) {
      _controller.repeat();
    }
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    // 網頁的 prefers-reduced-motion 直接拿掉掃光，只留靜態色塊。
    if (_reduceMotion ?? false) return widget.child;

    return Stack(
      children: [
        widget.child,
        Positioned.fill(
          child: IgnorePointer(
            child: ClipRect(
              child: LayoutBuilder(
                builder: (context, constraints) {
                  final width = constraints.maxWidth.isFinite
                      ? constraints.maxWidth
                      : MediaQuery.sizeOf(context).width;
                  return AnimatedBuilder(
                    animation: _sweep,
                    builder: (context, child) => Transform.translate(
                      offset: Offset(width * (-1.2 + 2.4 * _sweep.value), 0),
                      child: child,
                    ),
                    child: const DecoratedBox(
                      decoration: BoxDecoration(
                        gradient: LinearGradient(
                          // CSS 的 100deg：水平再往下傾 10 度。
                          begin: Alignment(-1, -0.18),
                          end: Alignment(1, 0.18),
                          stops: [0.3, 0.5, 0.7],
                          colors: [
                            Color(0x00FFFFFF),
                            HkSkeletonShimmer.sheen,
                            Color(0x00FFFFFF),
                          ],
                        ),
                      ),
                      child: SizedBox.expand(),
                    ),
                  );
                },
              ),
            ),
          ),
        ),
      ],
    );
  }
}
