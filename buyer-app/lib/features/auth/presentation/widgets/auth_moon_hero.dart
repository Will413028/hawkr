import 'dart:async';
import 'dart:ui' show lerpDouble;

import 'package:flutter/material.dart';

import '../../../../shared/widgets/hk_placeholder.dart';

/// 登入頁月亮在一個晝夜週期中的呈現狀態。
///
/// 時段刻意與登入背景的夜色一致：19:00 開始出現、21:00 完全可見，
/// 05:00 開始淡出、07:00 完全隱藏。月亮在這 12 小時中只向上移動，
/// 不會在午夜跳回起點。
@immutable
class AuthMoonTimeState {
  const AuthMoonTimeState({required this.opacity, required this.riseProgress});

  final double opacity;
  final double riseProgress;

  bool get isVisible => opacity > 0.001;

  static AuthMoonTimeState at(DateTime time) {
    const duskStart = 19 * 60.0;
    const fullNightStart = 21 * 60.0;
    const dawnStart = 5 * 60.0;
    const dawnEnd = 7 * 60.0;
    const dayMinutes = 24 * 60.0;
    const cycleMinutes = 12 * 60.0;

    final minute = time.hour * 60.0 + time.minute + time.second / 60.0;
    final inNightCycle = minute >= duskStart || minute < dawnEnd;
    if (!inNightCycle) {
      return const AuthMoonTimeState(opacity: 0, riseProgress: 0);
    }

    final elapsed = minute >= duskStart
        ? minute - duskStart
        : minute + dayMinutes - duskStart;
    final riseProgress = (elapsed / cycleMinutes).clamp(0.0, 1.0);

    final double opacity;
    if (minute >= duskStart && minute < fullNightStart) {
      opacity = (minute - duskStart) / (fullNightStart - duskStart);
    } else if (minute >= dawnStart && minute < dawnEnd) {
      opacity = 1 - (minute - dawnStart) / (dawnEnd - dawnStart);
    } else {
      opacity = 1;
    }

    return AuthMoonTimeState(
      opacity: opacity.clamp(0.0, 1.0),
      riseProgress: riseProgress,
    );
  }
}

/// 帶柔和過曝光暈、會隨真實時間升起並緩慢自轉的登入頁月亮。
///
/// 月面與光暈共用一個 24 秒的完整呼吸週期，月面另以 400 秒完成一圈；
/// 動畫停用或頁面不在前景時，兩個 controller 都會停止。
class AuthMoonHero extends StatefulWidget {
  const AuthMoonHero({super.key, required this.size, this.time})
    : assert(size > 0);

  final double size;

  /// 測試與預覽用固定時間；正式畫面留空即使用裝置時間。
  final DateTime? time;

  @override
  State<AuthMoonHero> createState() => _AuthMoonHeroState();
}

class _AuthMoonHeroState extends State<AuthMoonHero>
    with TickerProviderStateMixin {
  late final AnimationController _breathingController = AnimationController(
    vsync: this,
    duration: const Duration(seconds: 12),
    value: 0.5,
  );
  late final AnimationController _rotationController = AnimationController(
    vsync: this,
    duration: const Duration(seconds: 400),
  );
  Timer? _clockTimer;
  DateTime _now = DateTime.now();
  bool _tickerEnabled = true;
  bool _animationsDisabled = false;

  @override
  void initState() {
    super.initState();
    _syncClock();
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    // ignore: deprecated_member_use
    _tickerEnabled = TickerMode.of(context);
    _animationsDisabled =
        MediaQuery.maybeOf(context)?.disableAnimations ?? false;
    _syncAnimationActivity();
  }

  void _syncAnimationActivity() {
    final isVisible = AuthMoonTimeState.at(widget.time ?? _now).isVisible;
    final shouldAnimate = isVisible && _tickerEnabled && !_animationsDisabled;
    if (shouldAnimate && !_breathingController.isAnimating) {
      _breathingController.repeat(reverse: true);
    } else if (!shouldAnimate && _breathingController.isAnimating) {
      _breathingController.stop(canceled: false);
      _breathingController.value = 0.5;
    }
    if (shouldAnimate && !_rotationController.isAnimating) {
      _rotationController.repeat();
    } else if (!shouldAnimate && _rotationController.isAnimating) {
      _rotationController.stop(canceled: false);
      if (_animationsDisabled) _rotationController.value = 0;
    }
  }

  @override
  void didUpdateWidget(covariant AuthMoonHero oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.time != widget.time) {
      _syncClock();
      _syncAnimationActivity();
    }
  }

  void _syncClock() {
    _clockTimer?.cancel();
    _clockTimer = null;
    _now = widget.time ?? DateTime.now();
    if (widget.time != null) return;
    _clockTimer = Timer.periodic(const Duration(minutes: 1), (_) {
      if (!mounted) return;
      setState(() => _now = DateTime.now());
      _syncAnimationActivity();
    });
  }

  @override
  void dispose() {
    _clockTimer?.cancel();
    _breathingController.dispose();
    _rotationController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return SizedBox.square(
      key: const Key('auth-moon-slot'),
      dimension: widget.size,
      child: AnimatedBuilder(
        animation: _breathingController,
        child: RepaintBoundary(
          child: ClipOval(
            child: Stack(
              fit: StackFit.expand,
              children: [
                Center(
                  child: SizedBox.square(
                    dimension: widget.size * 0.82,
                    child: const ClipOval(
                      child: HkPlaceholder(HkPlaceholderKind.hero),
                    ),
                  ),
                ),
                Center(
                  // 月面素材四周有透明留白；曝光層只覆蓋實際月球，避免
                  // 透明區被 radial gradient 畫成灰色圓框。
                  child: SizedBox.square(
                    dimension: widget.size * 0.82,
                    child: const ClipOval(
                      child: DecoratedBox(
                        key: Key('auth-moon-overexposure'),
                        decoration: BoxDecoration(
                          gradient: RadialGradient(
                            center: Alignment(-0.28, -0.32),
                            radius: 0.92,
                            colors: [
                              Color(0x5CFFFCEB),
                              Color(0x2EFFF8E8),
                              Colors.transparent,
                            ],
                            stops: [0, 0.56, 1],
                          ),
                        ),
                      ),
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
        builder: (context, moonSurface) {
          final effectiveTime = widget.time ?? _now;
          final phase = AuthMoonTimeState.at(effectiveTime);
          if (!phase.isVisible) return const SizedBox.shrink();

          final breath = Curves.easeInOutSine.transform(
            _breathingController.value,
          );
          final scale = lerpDouble(0.995, 1.02, breath)!;
          final surfaceOpacity = lerpDouble(0.98, 1, breath)!;
          final riseOffset = lerpDouble(
            widget.size * 0.12,
            -widget.size * 0.16,
            phase.riseProgress,
          )!;
          final glowAlpha = lerpDouble(0.26, 0.38, breath)!;

          return Opacity(
            key: const Key('auth-moon-opacity'),
            opacity: (phase.opacity * surfaceOpacity).clamp(0.0, 1.0),
            child: Transform.translate(
              offset: Offset(0, riseOffset),
              child: Transform.scale(
                scale: scale,
                child: Stack(
                  key: const Key('auth-moon-visual'),
                  clipBehavior: Clip.none,
                  fit: StackFit.expand,
                  children: [
                    OverflowBox(
                      // 單層大範圍柔光一路淡出到透明；不使用大 blur shadow，
                      // 也不縮放透明圖層，避免 iOS Impeller 的多邊形瑕疵。
                      maxWidth: widget.size * 2.4,
                      maxHeight: widget.size * 2.4,
                      child: SizedBox.square(
                        dimension: widget.size * 2.4,
                        child: DecoratedBox(
                          key: const Key('auth-moon-outer-halo'),
                          decoration: BoxDecoration(
                            shape: BoxShape.circle,
                            gradient: RadialGradient(
                              colors: [
                                Colors.white.withValues(
                                  alpha: glowAlpha * 0.50,
                                ),
                                Colors.white.withValues(
                                  alpha: glowAlpha * 0.28,
                                ),
                                Colors.white.withValues(
                                  alpha: glowAlpha * 0.08,
                                ),
                                Colors.transparent,
                              ],
                              stops: const [0, 0.42, 0.75, 1],
                            ),
                          ),
                        ),
                      ),
                    ),
                    if (moonSurface != null)
                      RotationTransition(
                        key: const Key('auth-moon-rotation'),
                        turns: _rotationController,
                        child: moonSurface,
                      ),
                  ],
                ),
              ),
            ),
          );
        },
      ),
    );
  }
}
