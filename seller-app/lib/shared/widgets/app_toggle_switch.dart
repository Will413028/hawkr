import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter/physics.dart';
import 'package:flutter/services.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

/// 精緻滑桿元件（Hawkr 品牌樣式，Figma 36×20 spec）
///
/// 特色：
/// - **彈簧物理**：用 `SpringSimulation` 讓 thumb 自然回彈，比 easeInOut 更絲滑
/// - **拖拉跟手**：支援水平拖拉，thumb 即時跟著手指移動
/// - **速度感知**：放手時依甩動速度決定目標位置（像滑手機一樣）
/// - **觸覺回饋**：切換瞬間震動 `HapticFeedback.lightImpact`
/// - **品牌主色**，`#CCCCCC` 灰，顏色跟位置同步動畫
/// - **尺寸可調**：傳 [height] 自訂高度（預設 20），寬度 / thumb / padding 依比例縮放
class AppToggleSwitch extends StatefulWidget {
  const AppToggleSwitch({
    super.key,
    required this.value,
    required this.onChanged,
    this.height,
  });

  final bool value;
  final ValueChanged<bool> onChanged;

  /// 自訂軌道高度（logical pixel，內部會套 `.h` scaling）。
  /// 預設 20 對應 Figma 36×20 spec。整體寬度 = height × 1.8、
  /// thumb = height × 0.8、padding = height × 0.1。
  final double? height;

  @override
  State<AppToggleSwitch> createState() => _AppToggleSwitchState();
}

class _AppToggleSwitchState extends State<AppToggleSwitch>
    with SingleTickerProviderStateMixin {
  // ── 尺寸常數（跟原 Figma 36×20 spec 的比例）──
  // width = h × 1.8   thumb = h × 0.8   padding = h × 0.1
  static const double _defaultHeight = 20;

  double get _h => widget.height ?? _defaultHeight;
  double get _trackW => _h * 1.8;
  double get _trackH => _h;
  double get _thumbSize => _h * 0.8;
  double get _padding => _h * 0.1;

  // 可拖拉的水平距離 (track 內扣掉 padding + thumb 寬度)
  double get _maxTranslate => _trackW.w - _thumbSize.w - _padding.w * 2;

  late final AnimationController _controller;
  bool _isDragging = false;
  bool _isPressed = false;

  // Spring: stiffness 500 + damping 26 大約是「有點 bounce 但不誇張」的黏手感
  static const _spring = SpringDescription(
    mass: 1,
    stiffness: 500,
    damping: 26,
  );

  @override
  void initState() {
    super.initState();
    _controller = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 280),
      value: widget.value ? 1.0 : 0.0,
    );
  }

  @override
  void didUpdateWidget(AppToggleSwitch oldWidget) {
    super.didUpdateWidget(oldWidget);
    // 拖拉中時由 drag handler 自己控制，避免 parent setState 打斷手感
    if (!_isDragging && oldWidget.value != widget.value) {
      _springTo(widget.value ? 1.0 : 0.0, velocity: 0);
    }
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  void _springTo(double target, {required double velocity}) {
    _controller.animateWith(
      SpringSimulation(_spring, _controller.value, target, velocity),
    );
  }

  void _handleTap() {
    HapticFeedback.lightImpact();
    widget.onChanged(!widget.value);
  }

  void _handleDragStart(DragStartDetails d) {
    _isDragging = true;
    _controller.stop();
  }

  void _handleDragUpdate(DragUpdateDetails d) {
    final delta = d.primaryDelta ?? 0;
    final normalizedDelta = delta / _maxTranslate;
    _controller.value = (_controller.value + normalizedDelta).clamp(0.0, 1.0);
  }

  void _handleDragEnd(DragEndDetails d) {
    _isDragging = false;
    final vxPx = d.velocity.pixelsPerSecond.dx;
    // 甩動速度 > 800 px/s → 直接決定方向；否則依當前位置靠哪一邊
    final bool targetOn;
    if (vxPx.abs() > 800) {
      targetOn = vxPx > 0;
    } else {
      targetOn = _controller.value > 0.5;
    }
    final targetValue = targetOn ? 1.0 : 0.0;
    // 把手指甩動速度 normalize 成 controller 0..1 單位
    final normalizedVelocity = vxPx / _maxTranslate;
    _springTo(targetValue, velocity: normalizedVelocity);

    if (targetOn != widget.value) {
      HapticFeedback.lightImpact();
      widget.onChanged(targetOn);
    }
  }

  void _handleTapDown(TapDownDetails d) => setState(() => _isPressed = true);
  void _handleTapUp(TapUpDetails d) => setState(() => _isPressed = false);
  void _handleTapCancel() => setState(() => _isPressed = false);

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: _handleTap,
      onTapDown: _handleTapDown,
      onTapUp: _handleTapUp,
      onTapCancel: _handleTapCancel,
      onHorizontalDragStart: _handleDragStart,
      onHorizontalDragUpdate: _handleDragUpdate,
      onHorizontalDragEnd: _handleDragEnd,
      behavior: HitTestBehavior.opaque,
      child: AnimatedBuilder(
        animation: _controller,
        builder: (context, _) {
          final t = _controller.value.clamp(0.0, 1.0);
          // 顏色走 easeOut，讓起始顏色變化較快（更有回應感）
          final colorT = Curves.easeOut.transform(t);
          final trackColor = Color.lerp(
            AppColors.gray300,
            AppColors.primary,
            colorT,
          )!;
          // 按壓時 thumb 微微放大 1.1x（黏手觸感）
          final thumbScale = _isPressed || _isDragging ? 1.1 : 1.0;

          return Container(
            width: _trackW.w,
            height: _trackH.h,
            padding: EdgeInsets.all(_padding.w),
            decoration: BoxDecoration(
              color: trackColor,
              borderRadius: BorderRadius.circular(999.r),
            ),
            child: Align(
              alignment: Alignment(-1.0 + 2.0 * t, 0),
              child: AnimatedScale(
                scale: thumbScale,
                duration: const Duration(milliseconds: 150),
                curve: Curves.easeOut,
                child: Container(
                  width: _thumbSize.w,
                  height: _thumbSize.w,
                  decoration: const BoxDecoration(
                    color: AppColors.bgCard,
                    shape: BoxShape.circle,
                    boxShadow: [
                      BoxShadow(
                        color: AppColors.cardShadow,
                        blurRadius: 4,
                        offset: Offset(0, 1),
                      ),
                    ],
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
