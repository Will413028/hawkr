import 'dart:async';
import 'dart:math';

import 'package:flutter/material.dart';
import '../icons/app_icons.dart';

/// 水果旋轉 loading 元件 — 比單純自轉更有「生鮮」感的動態效果。
///
/// 動畫層次：
/// - 背景圓暈：柔和 pulse（呼吸），讓 loading 不會死板。
/// - 水果輪播：每 ~1.6s 切換一顆水果，cross-fade + scale 進場。
/// - 自轉：略帶 ease 的旋轉，比 linear 更活潑。
/// - 上下浮動：±3px sine wave，像水果被輕拋接住。
///
/// 用於 inline loading 情境（商品上架中、訂單卡片載入等）。
class FruitSpinner extends StatefulWidget {
  const FruitSpinner({super.key, this.size = 40, this.durationMs = 1000});

  /// 整體容器大小（圓形底色 + 水果圖示）
  final double size;

  /// 一圈旋轉的毫秒數
  final int durationMs;

  @override
  State<FruitSpinner> createState() => _FruitSpinnerState();
}

class _FruitSpinnerState extends State<FruitSpinner>
    with TickerProviderStateMixin {
  static const _fruits = <({IconData icon, Color color})>[
    (icon: AppIcons.ecoOutlined, color: Color(0x4DFF3B30)),
    (icon: AppIcons.localFloristOutlined, color: Color(0x4D34C759)),
    (icon: AppIcons.spaOutlined, color: Color(0x4DFFCC00)),
    (icon: AppIcons.grassOutlined, color: Color(0x4DFF9500)),
    (icon: AppIcons.yardOutlined, color: Color(0x4DFFE433)),
    (icon: AppIcons.ecoOutlined, color: Color(0x4DFF4F64)),
    (icon: AppIcons.localFloristOutlined, color: Color(0x4DFF2D55)),
    (icon: AppIcons.spaOutlined, color: Color(0x4D5856D6)),
  ];

  static const _cycleInterval = Duration(milliseconds: 1600);

  late final AnimationController _spin;
  late final AnimationController _pulse;
  Timer? _cycleTimer;
  late int _index;

  @override
  void initState() {
    super.initState();
    _index = Random().nextInt(_fruits.length);

    _spin = AnimationController(
      vsync: this,
      duration: Duration(milliseconds: widget.durationMs),
    )..repeat();

    _pulse = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 1200),
    )..repeat(reverse: true);

    _cycleTimer = Timer.periodic(_cycleInterval, (_) {
      if (!mounted) return;
      setState(() {
        _index = (_index + 1) % _fruits.length;
      });
    });
  }

  @override
  void dispose() {
    _cycleTimer?.cancel();
    _spin.dispose();
    _pulse.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final picked = _fruits[_index];
    final fruitSize = widget.size * 0.72;

    return AnimatedBuilder(
      animation: Listenable.merge([_spin, _pulse]),
      builder: (context, _) {
        // 呼吸：背景圓暈隨 pulse 放大 1.0→1.08，淡入淡出。
        final pulseT = Curves.easeInOut.transform(_pulse.value);
        final haloScale = 1.0 + 0.08 * pulseT;
        final haloOpacity = 0.55 + 0.45 * (1 - pulseT);

        // 上下浮動：sin wave 給水果一點「被拋」的感覺。
        final bob = sin(_spin.value * 2 * pi) * (widget.size * 0.06);

        // 旋轉：輕度 ease（fast→slow→fast）讓動作不要那麼機械。
        final spinT = _easedSpin(_spin.value);

        return SizedBox(
          width: widget.size,
          height: widget.size,
          child: Stack(
            alignment: Alignment.center,
            children: [
              // 背景呼吸暈
              Transform.scale(
                scale: haloScale,
                child: Opacity(
                  opacity: haloOpacity,
                  child: Container(
                    width: widget.size,
                    height: widget.size,
                    decoration: BoxDecoration(
                      shape: BoxShape.circle,
                      color: picked.color,
                    ),
                  ),
                ),
              ),
              // 水果（旋轉 + 上下浮動 + 切換時 cross-fade）
              Transform.translate(
                offset: Offset(0, bob),
                child: Transform.rotate(
                  angle: spinT * 2 * pi,
                  child: AnimatedSwitcher(
                    duration: const Duration(milliseconds: 350),
                    transitionBuilder: (child, anim) {
                      return FadeTransition(
                        opacity: anim,
                        child: ScaleTransition(
                          scale: Tween<double>(begin: 0.6, end: 1.0).animate(
                            CurvedAnimation(
                              parent: anim,
                              curve: Curves.easeOutBack,
                            ),
                          ),
                          child: child,
                        ),
                      );
                    },
                    child: ExcludeSemantics(
                      child: Icon(
                        picked.icon,
                        key: ValueKey(_index),
                        size: fruitSize,
                        color: picked.color.withValues(alpha: 1),
                      ),
                    ),
                  ),
                ),
              ),
            ],
          ),
        );
      },
    );
  }

  /// 把 0..1 的 spin 進度轉成輕 ease 曲線（兩段 cubic ease in-out 串接），
  /// 比純 linear 更活潑，比 elasticInOut 更克制。
  double _easedSpin(double t) {
    if (t < 0.5) {
      final u = t * 2;
      return Curves.easeInOutCubic.transform(u) * 0.5;
    }
    final u = (t - 0.5) * 2;
    return 0.5 + Curves.easeInOutCubic.transform(u) * 0.5;
  }
}
