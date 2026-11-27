import 'dart:async';
import 'dart:math' as math;
import 'package:flutter/material.dart';
import 'package:sensors_plus/sensors_plus.dart';

import '../../core/theme/app_colors.dart';
import '../icons/app_icons.dart';

/// 落葉紛飛背景 (Falling Leaves Background)
///
/// 多片葉子從上方緩緩飄落，帶有自然的左右搖擺與旋轉效果。
/// 葉子飄出畫面底部後會從頂部重新進入，形成無盡的落葉效果。
///
/// 使用範例：
/// ```dart
/// AnimatedOrbsBackground(
///   child: YourContentWidget(),
/// )
/// ```
class AnimatedOrbsBackground extends StatefulWidget {
  final Widget child;
  final double orb1Size;
  final double orb2Size;
  final double blurSigma;
  final Color backgroundColor;
  final bool enableAnimation;
  final bool enableBreathing;
  final bool enableColorTransition;
  final bool enableBlurTransition;
  final double speedMultiplier;
  final double breathingAmplitude;
  final double breathingPeriod;
  final double colorTransitionPeriod;
  final double blurTransitionPeriod;

  /// 落葉數量
  final int leafCount;

  /// 測試與預覽用固定時間；正式畫面留空即使用裝置時間。
  final DateTime? currentTime;

  const AnimatedOrbsBackground({
    super.key,
    required this.child,
    this.orb1Size = 288,
    this.orb2Size = 371,
    this.blurSigma = 4.7,
    this.backgroundColor = AppColors.bgPrimary,
    this.enableAnimation = true,
    this.enableBreathing = true,
    this.enableColorTransition = true,
    this.enableBlurTransition = true,
    this.speedMultiplier = 0.1,
    this.breathingAmplitude = 0.15,
    this.breathingPeriod = 4.0,
    this.colorTransitionPeriod = 24.0,
    this.blurTransitionPeriod = 24.0,
    this.leafCount = 28,
    this.currentTime,
  });

  /// 沒有傳 [currentTime] 時的時間來源；測試設定（`test/flutter_test_config.dart`）
  /// 固定它，讓晝夜分支不隨測試執行的時刻改變。
  @visibleForTesting
  static DateTime Function() debugNow = DateTime.now;

  @override
  State<AnimatedOrbsBackground> createState() => _AnimatedOrbsBackgroundState();
}

const double _legacyFramesPerSecond = 60;
const double _windRetentionPerLegacyFrame = 0.98;

/// Caps a single resumed frame to four legacy 60 Hz frames. This keeps a long
/// app suspension from teleporting leaves while normal 60/120 Hz ticks retain
/// their real wall-clock duration.
@visibleForTesting
const double animatedOrbsMaxFrameDeltaSeconds = 1 / 15;

/// Converts [AnimationController.lastElapsedDuration] samples to a bounded
/// wall-clock delta. A missing or non-monotonic sample starts a new timeline.
@visibleForTesting
double animatedOrbsFrameDeltaSeconds(
  Duration? previousElapsed,
  Duration? currentElapsed,
) {
  if (previousElapsed == null || currentElapsed == null) return 0;
  final elapsedMicroseconds =
      currentElapsed.inMicroseconds - previousElapsed.inMicroseconds;
  if (elapsedMicroseconds <= 0) return 0;
  final seconds =
      elapsedMicroseconds / Duration.microsecondsPerSecond.toDouble();
  return math.min(seconds, animatedOrbsMaxFrameDeltaSeconds);
}

/// Advances the gust velocity with the same 2% approach used by the old 60 Hz
/// loop, while analytically integrating displacement so subdividing one second
/// into 60 or 120 ticks produces the same result.
@visibleForTesting
({double velocityPerSecond, double displacement}) advanceAnimatedOrbsWind({
  required double velocityPerSecond,
  required double targetVelocityPerSecond,
  required double dtSeconds,
}) {
  if (dtSeconds <= 0) {
    return (velocityPerSecond: velocityPerSecond, displacement: 0);
  }

  final legacyFrames = dtSeconds * _legacyFramesPerSecond;
  final retention = math
      .pow(_windRetentionPerLegacyFrame, legacyFrames)
      .toDouble();
  final velocityDelta = velocityPerSecond - targetVelocityPerSecond;
  final nextVelocity = targetVelocityPerSecond + velocityDelta * retention;
  final displacement =
      targetVelocityPerSecond * dtSeconds +
      (velocityDelta / _legacyFramesPerSecond) *
          _windRetentionPerLegacyFrame *
          (1 - retention) /
          (1 - _windRetentionPerLegacyFrame);
  return (velocityPerSecond: nextVelocity, displacement: displacement);
}

/// Advances one leaf using per-second motion values and the already integrated
/// horizontal [windDisplacement].
@visibleForTesting
({double x, double y, double rotation}) advanceAnimatedOrbsLeaf({
  required double x,
  required double y,
  required double rotation,
  required double fallSpeedPerSecond,
  required double rotationSpeedPerSecond,
  required double gravityX,
  required double gravityY,
  required double windDisplacement,
  required double dtSeconds,
}) {
  return (
    x: x + fallSpeedPerSecond * gravityX * dtSeconds + windDisplacement,
    y: y + fallSpeedPerSecond * gravityY * dtSeconds,
    rotation: rotation + rotationSpeedPerSecond * dtSeconds,
  );
}

/// Converts one uniform random sample into the existing 5-14 second gust gap.
@visibleForTesting
double animatedOrbsGustIntervalSeconds(double randomUnit) {
  assert(randomUnit >= 0 && randomUnit <= 1);
  return 5 + randomUnit * 9;
}

class _FallingLeaf {
  /// 歸一化位置 (0~1)
  double x;
  double y;

  /// 下落速度 (歸一化/秒)
  double fallSpeedPerSecond;

  /// 水平搖擺幅度 (歸一化)
  double swayAmplitude;

  /// 搖擺角速度 (弧度/秒)
  double swayAngularVelocity;

  /// 搖擺相位偏移
  double swayPhase;

  /// 當前旋轉角度
  double rotation;

  /// 旋轉速度 (弧度/秒)
  double rotationSpeedPerSecond;

  /// 葉子大小
  double size;

  /// 透明度
  double opacity;

  /// 模糊程度
  double blur;

  /// 使用的葉子圖示 index
  int iconIndex;

  /// 季節色（從當季調色盤抽出，套到圖示上）
  Color tintColor;

  /// 不會隨每一幀改變的圖示 subtree。
  ///
  /// 位置、旋轉與透明度仍由外層動畫更新，但保留同一個 Widget instance
  /// 可以避免每 16ms 重新建立 Icon。
  Widget visual;

  _FallingLeaf({
    required this.x,
    required this.y,
    required this.fallSpeedPerSecond,
    required this.swayAmplitude,
    required this.swayAngularVelocity,
    required this.swayPhase,
    required this.rotation,
    required this.rotationSpeedPerSecond,
    required this.size,
    required this.opacity,
    required this.blur,
    required this.iconIndex,
    required this.tintColor,
    required this.visual,
  });
}

/// 一天內背景色關鍵格 (小時 0~24, color)
const List<(double, Color)> _dayColorStops = [
  (0.0, Color(0xFF1A2540)),
  (5.5, Color(0xFF2C3855)),
  (6.5, Color(0xFFFFD9AE)),
  (8.0, AppColors.bgPrimary),
  (12.0, AppColors.neutralCream),
  (16.0, Color(0xFFFFF4DC)),
  (18.0, Color(0xFFFFA877)),
  (19.5, Color(0xFF55557F)),
  (21.0, Color(0xFF1A2540)),
  (24.0, Color(0xFF1A2540)),
];

Color _computeBackgroundColor(DateTime now) {
  final h = now.hour + now.minute / 60.0;
  for (int i = 0; i < _dayColorStops.length - 1; i++) {
    final (t1, c1) = _dayColorStops[i];
    final (t2, c2) = _dayColorStops[i + 1];
    if (h >= t1 && h <= t2) {
      final t = (h - t1) / (t2 - t1);
      return Color.lerp(c1, c2, t)!;
    }
  }
  return _dayColorStops.first.$2;
}

/// 一天的「夜晚強度」0~1：21~5 滿值，5~7 漸退、19~21 漸升，白天 0
double _computeNightIntensity(DateTime now) {
  final h = now.hour + now.minute / 60.0;
  if (h >= 21.0 || h < 5.0) return 1.0;
  if (h >= 19.0 && h < 21.0) return (h - 19.0) / 2.0;
  if (h >= 5.0 && h < 7.0) return 1.0 - (h - 5.0) / 2.0;
  return 0.0;
}

/// 一顆星：固定位置 + 個別閃爍頻率/相位，靠 sin(time) 做明滅
class _Star {
  final double x;
  final double y;
  final double size;
  final double baseBrightness;
  final double twinkleAngularVelocity;
  final double twinklePhase;
  final Color color;

  _Star({
    required this.x,
    required this.y,
    required this.size,
    required this.baseBrightness,
    required this.twinkleAngularVelocity,
    required this.twinklePhase,
    required this.color,
  });
}

class _StarFieldPainter extends CustomPainter {
  _StarFieldPainter({
    required this.stars,
    required this.time,
    required this.intensity,
  });

  final List<_Star> stars;
  final double time;
  final double intensity;

  @override
  void paint(Canvas canvas, Size size) {
    if (intensity <= 0.001) return;
    for (final s in stars) {
      final wave = math.sin(time * s.twinkleAngularVelocity + s.twinklePhase);
      final w2 = wave * wave;
      final twinkle = 0.15 + 0.85 * (w2 * w2);
      final alpha = (s.baseBrightness * intensity * twinkle).clamp(0.0, 1.0);
      final paint = Paint()
        ..color = s.color.withValues(alpha: alpha)
        ..style = PaintingStyle.fill;
      canvas.drawCircle(
        Offset(s.x * size.width, s.y * size.height),
        s.size,
        paint,
      );
    }
  }

  @override
  bool shouldRepaint(_StarFieldPainter old) =>
      old.time != time || old.intensity != intensity || old.stars != stars;
}

/// 月份 → 季節調色盤（3~5 春 / 6~8 夏 / 9~11 秋 / 12~2 冬）
/// 一律走 AppColors token，與品牌色系統一致；同時保留四季視覺差異
List<Color> _seasonalPalette(int month) {
  if (month >= 3 && month <= 5) {
    return const [
      AppColors.primaryLight,
      AppColors.primarySoft,
      AppColors.warningSoft,
      AppColors.dangerSoft,
    ];
  } else if (month >= 6 && month <= 8) {
    return const [
      AppColors.primary,
      AppColors.primaryDark,
      AppColors.primaryLight,
      AppColors.secondary,
    ];
  } else if (month >= 9 && month <= 11) {
    return const [
      AppColors.warning,
      AppColors.accentFill,
      AppColors.primaryDark,
      AppColors.primary,
    ];
  } else {
    return const [
      AppColors.white,
      AppColors.gray100,
      AppColors.gray200,
      AppColors.infoSoft,
    ];
  }
}

class _AnimatedOrbsBackgroundState extends State<AnimatedOrbsBackground>
    with SingleTickerProviderStateMixin {
  late AnimationController _controller;
  final math.Random _random = math.Random();
  List<_FallingLeaf> _leaves = [];
  StreamSubscription<AccelerometerEvent>? _accelSub;
  bool _tickerEnabled = true;
  bool _animationsDisabled = false;

  double _gravityX = 0;
  double _gravityY = 1.0;

  double _windVelocity = 0;
  double _windTargetVelocity = 0;
  double _gustCooldownSeconds = 0;

  double _elapsedSeconds = 0;
  Duration? _lastControllerElapsed;

  DateTime _now = AnimatedOrbsBackground.debugNow();
  Timer? _clockTimer;

  late final List<_Star> _stars;

  static const List<IconData> _leafIcons = [
    AppIcons.ecoOutlined,
    AppIcons.spaOutlined,
    AppIcons.localFloristOutlined,
    AppIcons.grassOutlined,
    AppIcons.yardOutlined,
    AppIcons.energySavingsLeafOutlined,
    AppIcons.filterVintageOutlined,
    AppIcons.parkOutlined,
    AppIcons.forestOutlined,
  ];

  @override
  void initState() {
    super.initState();
    _syncClock();
    _initStars();
    _initLeaves();
    _scheduleNextGust();
    _controller = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 16),
    );
    _controller.addListener(_tick);
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    // ignore: deprecated_member_use
    _tickerEnabled = TickerMode.of(context);
    _animationsDisabled =
        MediaQuery.maybeOf(context)?.disableAnimations ?? false;
    _syncActivity();
  }

  @override
  void didUpdateWidget(covariant AnimatedOrbsBackground oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.leafCount != widget.leafCount) {
      _initLeaves();
    }
    if (oldWidget.currentTime != widget.currentTime) {
      _syncClock();
      _initLeaves();
    }
    if (oldWidget.enableAnimation != widget.enableAnimation) {
      _syncActivity();
    }
  }

  void _syncClock() {
    _clockTimer?.cancel();
    _clockTimer = null;
    _now = widget.currentTime ?? AnimatedOrbsBackground.debugNow();
    if (widget.currentTime != null) return;
    _clockTimer = Timer.periodic(const Duration(minutes: 1), (_) {
      if (mounted) setState(() => _now = AnimatedOrbsBackground.debugNow());
    });
  }

  void _syncActivity() {
    final shouldAnimate =
        widget.enableAnimation && _tickerEnabled && !_animationsDisabled;
    if (shouldAnimate) {
      if (!_controller.isAnimating) {
        _controller.repeat();
        _lastControllerElapsed = Duration.zero;
      }
      _startAccelerometer();
      return;
    }

    _controller.stop(canceled: false);
    _lastControllerElapsed = null;
    _accelSub?.cancel();
    _accelSub = null;
  }

  void _startAccelerometer() {
    if (_accelSub != null) return;
    _accelSub =
        accelerometerEventStream(
          samplingPeriod: SensorInterval.uiInterval,
        ).listen(
          (e) {
            const invG = 1.0 / 9.81;
            double gx = e.x * invG;
            double gy = -e.y * invG;
            final mag = math.sqrt(gx * gx + gy * gy);
            if (mag < 0.3) {
              gx = 0;
              gy = 0.3;
            } else if (mag > 1.2) {
              gx /= mag;
              gy /= mag;
            }
            _gravityX = _gravityX * 0.88 + gx * 0.12;
            _gravityY = _gravityY * 0.88 + gy * 0.12;
          },
          onError: (_) {
            _accelSub = null;
          },
          cancelOnError: true,
        );
  }

  /// 每隔 5~14 秒重抽一次風向 / 強度，平滑漸變過去模擬陣風
  void _scheduleNextGust() {
    _gustCooldownSeconds = animatedOrbsGustIntervalSeconds(
      _random.nextDouble(),
    );
    final strengthPerSecond =
        (0.0003 + _random.nextDouble() * 0.0008) * _legacyFramesPerSecond;
    _windTargetVelocity = strengthPerSecond * (_random.nextBool() ? 1 : -1);
  }

  void _initStars() {
    const starColors = [
      Color(0xFFFFFFFF),
      Color(0xFFFFE9B8),
      Color(0xFFD4E4FF),
    ];
    _stars = List.generate(80, (_) {
      final depth = _random.nextDouble();
      final colorRoll = _random.nextDouble();
      final color = colorRoll < 0.7
          ? starColors[0]
          : (colorRoll < 0.9 ? starColors[1] : starColors[2]);
      return _Star(
        x: _random.nextDouble(),
        y: _random.nextDouble(),
        size: 0.6 + depth * 1.6,
        baseBrightness: 0.35 + depth * 0.65,
        twinkleAngularVelocity:
            (0.0035 + _random.nextDouble() * 0.0085) * _legacyFramesPerSecond,
        twinklePhase: _random.nextDouble() * 2 * math.pi,
        color: color,
      );
    });
  }

  void _initLeaves() {
    _leaves = List.generate(
      widget.leafCount,
      (i) => _createLeaf(randomY: true),
    );
  }

  _FallingLeaf _createLeaf({bool randomY = false}) {
    final depth = _random.nextDouble();

    final size = 30 + depth * 90;

    final fallSpeedPerSecond =
        (0.00018 + depth * 0.00045) *
        widget.speedMultiplier *
        10 *
        _legacyFramesPerSecond;

    final swayAmplitude = 0.04 + _random.nextDouble() * 0.14;

    final swayAngularVelocity =
        (0.005 + _random.nextDouble() * 0.012) * _legacyFramesPerSecond;

    final opacity = 0.15 + depth * 0.35;

    const blur = 0.0;

    final palette = _seasonalPalette(_now.month);
    final tint = palette[_random.nextInt(palette.length)];

    final iconIndex = _random.nextInt(_leafIcons.length);
    final visual = Icon(_leafIcons[iconIndex], size: size, color: tint);

    return _FallingLeaf(
      x: _random.nextDouble(),
      y: randomY
          ? -0.1 + _random.nextDouble() * 1.2
          : -0.1 - _random.nextDouble() * 0.3,
      fallSpeedPerSecond: fallSpeedPerSecond,
      swayAmplitude: swayAmplitude,
      swayAngularVelocity: swayAngularVelocity,
      swayPhase: _random.nextDouble() * 2 * math.pi,
      rotation: _random.nextDouble() * 2 * math.pi,
      rotationSpeedPerSecond:
          (0.003 + _random.nextDouble() * 0.010) *
          (_random.nextBool() ? 1 : -1) *
          _legacyFramesPerSecond,
      size: size,
      opacity: opacity,
      blur: blur,
      iconIndex: iconIndex,
      tintColor: tint,
      visual: visual,
    );
  }

  void _tick() {
    if (!mounted) return;
    final controllerElapsed = _controller.lastElapsedDuration;
    final dtSeconds = animatedOrbsFrameDeltaSeconds(
      _lastControllerElapsed,
      controllerElapsed,
    );
    _lastControllerElapsed = controllerElapsed;
    if (dtSeconds <= 0) return;
    _elapsedSeconds += dtSeconds;

    final windDisplacement = _advanceWind(dtSeconds);

    for (final leaf in _leaves) {
      final motion = advanceAnimatedOrbsLeaf(
        x: leaf.x,
        y: leaf.y,
        rotation: leaf.rotation,
        fallSpeedPerSecond: leaf.fallSpeedPerSecond,
        rotationSpeedPerSecond: leaf.rotationSpeedPerSecond,
        gravityX: _gravityX,
        gravityY: _gravityY,
        windDisplacement: windDisplacement,
        dtSeconds: dtSeconds,
      );
      leaf.x = motion.x;
      leaf.y = motion.y;
      leaf.rotation = motion.rotation;

      final outOfBounds =
          leaf.y > 1.2 || leaf.y < -0.3 || leaf.x < -0.3 || leaf.x > 1.3;
      if (outOfBounds) {
        final newLeaf = _createLeaf();
        if (_gravityY.abs() >= _gravityX.abs()) {
          leaf.x = newLeaf.x;
          leaf.y = _gravityY >= 0
              ? -0.1 - _random.nextDouble() * 0.3
              : 1.1 + _random.nextDouble() * 0.3;
        } else {
          leaf.y = _random.nextDouble();
          leaf.x = _gravityX >= 0
              ? -0.1 - _random.nextDouble() * 0.3
              : 1.1 + _random.nextDouble() * 0.3;
        }
        leaf.fallSpeedPerSecond = newLeaf.fallSpeedPerSecond;
        leaf.swayAmplitude = newLeaf.swayAmplitude;
        leaf.swayAngularVelocity = newLeaf.swayAngularVelocity;
        leaf.swayPhase = newLeaf.swayPhase;
        leaf.rotation = newLeaf.rotation;
        leaf.rotationSpeedPerSecond = newLeaf.rotationSpeedPerSecond;
        leaf.size = newLeaf.size;
        leaf.opacity = newLeaf.opacity;
        leaf.blur = newLeaf.blur;
        leaf.iconIndex = newLeaf.iconIndex;
        leaf.tintColor = newLeaf.tintColor;
        leaf.visual = newLeaf.visual;
      }
    }
  }

  double _advanceWind(double dtSeconds) {
    var remainingSeconds = dtSeconds;
    var displacement = 0.0;

    while (remainingSeconds > 0) {
      if (_gustCooldownSeconds <= 0) _scheduleNextGust();
      final segmentSeconds = math.min(remainingSeconds, _gustCooldownSeconds);
      final advance = advanceAnimatedOrbsWind(
        velocityPerSecond: _windVelocity,
        targetVelocityPerSecond: _windTargetVelocity,
        dtSeconds: segmentSeconds,
      );
      _windVelocity = advance.velocityPerSecond;
      displacement += advance.displacement;
      remainingSeconds -= segmentSeconds;
      _gustCooldownSeconds -= segmentSeconds;

      if (_gustCooldownSeconds <= 1e-12) {
        _gustCooldownSeconds = 0;
        _scheduleNextGust();
      }
    }

    return displacement;
  }

  @override
  void dispose() {
    _clockTimer?.cancel();
    _accelSub?.cancel();
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final screenWidth = MediaQuery.of(context).size.width;
    final screenHeight = MediaQuery.of(context).size.height;

    final nightIntensity = _computeNightIntensity(_now);
    final bgColor = _computeBackgroundColor(_now);
    final luminance = bgColor.computeLuminance();
    return AnimatedBuilder(
      animation: _controller,
      child: RepaintBoundary(child: widget.child),
      builder: (context, foreground) {
        return AmbientBrightnessScope(
          luminance: luminance,
          child: Container(
            color: bgColor,
            child: Stack(
              children: [
                if (nightIntensity > 0)
                  Positioned.fill(
                    child: IgnorePointer(
                      child: CustomPaint(
                        painter: _StarFieldPainter(
                          stars: _stars,
                          time: _elapsedSeconds,
                          intensity: nightIntensity,
                        ),
                      ),
                    ),
                  ),
                if (nightIntensity < 0.95)
                  Positioned.fill(
                    key: const ValueKey('seasonal-leaves-layer'),
                    child: Stack(
                      clipBehavior: Clip.none,
                      children: [
                        for (final leaf in _leaves)
                          _buildLeaf(
                            leaf,
                            screenWidth,
                            screenHeight,
                            nightIntensity,
                          ),
                      ],
                    ),
                  ),
                if (foreground != null)
                  Positioned.fill(
                    key: const ValueKey('orbs-foreground'),
                    child: foreground,
                  ),
              ],
            ),
          ),
        );
      },
    );
  }

  Widget _buildLeaf(
    _FallingLeaf leaf,
    double screenWidth,
    double screenHeight,
    double nightIntensity,
  ) {
    final swayOffset =
        math.sin(_elapsedSeconds * leaf.swayAngularVelocity + leaf.swayPhase) *
        leaf.swayAmplitude;
    final displayX = (leaf.x + swayOffset) * screenWidth - leaf.size / 2;
    final displayY = leaf.y * screenHeight - leaf.size / 2;

    final nightDim = 1.0 - nightIntensity;
    final effOpacity = (leaf.opacity * nightDim).clamp(0.0, 1.0);

    return Positioned(
      left: displayX,
      top: displayY,
      child: Transform.rotate(
        angle: leaf.rotation,
        child: Opacity(opacity: effOpacity, child: leaf.visual),
      ),
    );
  }
}

/// 把當前背景亮度透給前景內容，前景據此挑深 / 淺字。
///
/// 只有「越過 prefersDarkText 閾值」才會通知 dependents rebuild —— 例如清晨
/// 由暗轉亮、傍晚由亮轉暗各觸發一次，平時零成本。
class AmbientBrightnessScope extends InheritedWidget {
  const AmbientBrightnessScope({
    super.key,
    required this.luminance,
    required super.child,
  });

  /// 0 = 純黑、1 = 純白
  final double luminance;

  /// 背景夠亮、應該用深色文字提升對比
  bool get prefersDarkText => luminance >= 0.55;

  /// 取背景是否該用深字。InheritedWidget 不存在時預設 true（亮底假設）。
  static bool prefersDarkTextOf(BuildContext context) {
    final scope = context
        .dependOnInheritedWidgetOfExactType<AmbientBrightnessScope>();
    return scope?.prefersDarkText ?? true;
  }

  @override
  bool updateShouldNotify(AmbientBrightnessScope old) =>
      old.prefersDarkText != prefersDarkText;
}
