import 'package:flutter/material.dart';
import 'package:flutter_svg/flutter_svg.dart';

import '../../core/motion/hk_motion_transitions.dart';
import '../../core/theme/app_colors.dart';

/// Hawkr 攤位商標載入動畫，移植自網頁 `components/shared/BrandLogoLoader.tsx`。
///
/// 線稿商標常駐，遮雨棚先自中心綻放、檯面接著綻放，兩者到齊後短暫停留，
/// 再一起縮回中心讓迴圈乾淨地重來。時間軸與網頁
/// `styles/components/decorations-overlays.css` 的 `ilBladeAwning` /
/// `ilBladeCounter` keyframes 相同（2600ms、同一組 stop），因此 App 與網頁的
/// 載入動態完全一致。
///
/// 取代 Material 的 [CircularProgressIndicator]：全 App 的載入指示只會有
/// 這一種，不再出現平台原生的圓弧 spinner。
enum HkBrandLoaderTone {
  /// Hawkr 原色（遮雨棚 primary、條紋 secondary、檯面 ink）。只用在
  /// 中性／淺色底（頁面、卡片、對話框）。
  brand,

  /// 同 [brand]，檯面改用深色底上的文字色。
  brandOnDark,

  /// 整枚商標壓成單色，吃呼叫端給的前景色（等同網頁的 `currentColor`）。
  ///
  /// 實心品牌色表面上一定要用這個或 [onDark]：原色商標疊在同色底上會
  /// 完全隱形。
  mono,

  /// 壓在淺色底上的單色商標，對照網頁 `--brand-logo-on-light`（primary）。
  onLight,

  /// 壓在深色底（含 primary-strong 實心底）上的單色商標，對照網頁
  /// `--brand-logo-on-dark`（ink-on-dark）。
  onDark,
}

class HkBrandLoader extends StatefulWidget {
  const HkBrandLoader({
    super.key,
    this.size,
    this.tone = HkBrandLoaderTone.brand,
    this.color,
    this.semanticsLabel = '載入中',
  });

  /// 網頁 `buttonLoadingIndicatorSize` 的三個按鈕尺寸。
  static const double buttonSm = 16;
  static const double buttonMd = 18;
  static const double buttonLg = 20;

  /// 行內／區塊載入的預設尺寸。
  static const double inline = 24;

  /// 整頁或大區塊留白時的尺寸。
  static const double page = 40;

  /// 不給尺寸時填滿父層（等同網頁的 `size="fit"`）：呼叫端外層本來就有
  /// 固定大小的盒子時用這個，才不必把尺寸重寫一次。父層必須有上界約束。
  final double? size;
  final HkBrandLoaderTone tone;

  /// 只在 [HkBrandLoaderTone.mono] 生效。未給時取 [IconTheme] 的顏色，
  /// 再退回 [DefaultTextStyle]，對應網頁交給呼叫端決定可讀前景色的作法。
  final Color? color;

  /// 傳 null 代表這顆 loader 純裝飾（同一畫面已有其他無障礙提示時）。
  final String? semanticsLabel;

  @override
  State<HkBrandLoader> createState() => _HkBrandLoaderState();
}

class _HkBrandLoaderState extends State<HkBrandLoader>
    with SingleTickerProviderStateMixin {
  late final AnimationController _controller;
  bool? _reduceMotion;

  @override
  void initState() {
    super.initState();
    _controller = AnimationController(
      vsync: this,
      duration: HkBrandMark.loopDuration,
    );
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final reduceMotion = HkMotionTransitions.reduceMotionOf(context);
    if (_reduceMotion == reduceMotion) return;
    _reduceMotion = reduceMotion;
    if (reduceMotion) {
      _controller
        ..stop()
        ..value = HkBrandMark.staticFrame;
    } else {
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
    final mark = RepaintBoundary(
      child: HkBrandMark(
        animation: _controller,
        size: widget.size,
        tone: widget.tone,
        color: widget.color,
      ),
    );

    final label = widget.semanticsLabel;
    if (label == null) {
      return ExcludeSemantics(child: mark);
    }
    return Semantics(
      container: true,
      liveRegion: true,
      label: label,
      child: mark,
    );
  }
}

/// 純渲染層：不持有 controller，讓呼叫端用自己的動畫驅動。
///
/// 開場動畫需要商標與下方 wordmark 的掃光共用同一條時間軸，各自跑 controller
/// 會逐漸失步，因此開場動畫用這個、一般載入指示用 [HkBrandLoader]。
class HkBrandMark extends StatelessWidget {
  const HkBrandMark({
    super.key,
    required this.animation,
    this.size,
    this.tone = HkBrandLoaderTone.brand,
    this.color,
  });

  /// 與網頁 `.il-blade` 的 `2.6s` 迴圈相同。
  static const Duration loopDuration = Duration(milliseconds: 2600);

  /// 減少動態時停住的那一格：遮雨棚與檯面皆已到齊、尚未開始縮回，
  /// 等同網頁 `prefers-reduced-motion` 直接顯示完成態的商標。
  static const double staticFrame = 0.85;

  final Animation<double> animation;

  /// null 代表填滿父層，見 [HkBrandLoader.size]。
  final double? size;
  final HkBrandLoaderTone tone;
  final Color? color;

  _MarkColors _colors(BuildContext context) {
    switch (tone) {
      case HkBrandLoaderTone.brand:
        return const _MarkColors(
          awning: AppColors.brandPrimary,
          stripe: AppColors.brandSecondary,
          counter: AppColors.brandInk,
        );
      case HkBrandLoaderTone.brandOnDark:
        return const _MarkColors(
          awning: AppColors.brandPrimary,
          stripe: AppColors.brandSecondary,
          counter: AppColors.brandInkOnDark,
        );
      case HkBrandLoaderTone.mono:
        final resolved =
            color ??
            IconTheme.of(context).color ??
            DefaultTextStyle.of(context).style.color ??
            AppColors.brandPrimaryStrong;
        return _MarkColors.solid(resolved);
      case HkBrandLoaderTone.onLight:
        return _MarkColors.solid(AppColors.brandPrimary);
      case HkBrandLoaderTone.onDark:
        return _MarkColors.solid(AppColors.brandInkOnDark);
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = _colors(context);
    final stack = Stack(
      fit: StackFit.expand,
      children: [
        _tinted(_awningOutlineSvg, colors.awning),
        _tinted(_counterOutlineSvg, colors.counter),
        _Blade(
          animation: animation,
          stops: _awningStops,
          layers: [
            _tinted(_awningFillSvg, colors.awning),
            _tinted(_stripeFillSvg, colors.stripe),
          ],
        ),
        _Blade(
          animation: animation,
          stops: _counterStops,
          layers: [_tinted(_counterFillSvg, colors.counter)],
        ),
      ],
    );
    final dimension = size;
    if (dimension == null) return stack;
    return SizedBox.square(dimension: dimension, child: stack);
  }
}

class _MarkColors {
  const _MarkColors({
    required this.awning,
    required this.stripe,
    required this.counter,
  });

  const _MarkColors.solid(Color color)
    : awning = color,
      stripe = color,
      counter = color;

  final Color awning;
  final Color stripe;
  final Color counter;
}

/// 顏色一律留給 [ColorFilter]，SVG 字串本身保持定值，flutter_svg 的解析快取
/// 才會命中（否則每種 tone 都會重新解析一次路徑）。
Widget _tinted(String svg, Color color) => SvgPicture.string(
  svg,
  colorFilter: ColorFilter.mode(color, BlendMode.srcIn),
);

/// 單一區塊在 0→1 迴圈裡的三個時間點（開始綻放／衝到最大／回穩到 1.0）。
typedef _BladeStops = ({double start, double peak, double settle});

/// 先遮雨棚、後檯面，數值逐一對應網頁 `ilBladeAwning` / `ilBladeCounter`
/// 的百分比。
const _BladeStops _awningStops = (start: 0.06, peak: 0.20, settle: 0.26);
const _BladeStops _counterStops = (start: 0.26, peak: 0.40, settle: 0.46);

/// 兩個區塊同時縮回中心的起點與長度，對應網頁 keyframes 的 `86% → 100%`。
const double _drainStart = 0.86;
const double _drainSpan = 1 - _drainStart;

/// 綻放起點與衝過頭的比例，對應 CSS 的 `scale(0.35)` → `scale(1.04)` → `scale(1)`。
const double _scaleFrom = 0.35;
const double _scaleOvershoot = 1.04;

class _Blade extends StatelessWidget {
  const _Blade({
    required this.animation,
    required this.layers,
    required this.stops,
  });

  final Animation<double> animation;
  final List<Widget> layers;
  final _BladeStops stops;

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: animation,
      child: Stack(fit: StackFit.expand, children: layers),
      builder: (context, child) {
        final t = animation.value;
        final entering = Curves.easeOutCubic.transform(
          ((t - stops.start) / (stops.peak - stops.start)).clamp(0.0, 1.0),
        );
        final settling = Curves.easeOut.transform(
          ((t - stops.peak) / (stops.settle - stops.peak)).clamp(0.0, 1.0),
        );
        final leaving = Curves.easeInOut.transform(
          ((t - _drainStart) / _drainSpan).clamp(0.0, 1.0),
        );
        final scaleIn = _scaleFrom + (_scaleOvershoot - _scaleFrom) * entering;
        final scale =
            (scaleIn + (1 - scaleIn) * settling) * (1 - leaving) +
            _scaleFrom * leaving;
        final opacity = entering * (1 - leaving);

        if (opacity <= 0) return const SizedBox.shrink();
        return Opacity(
          opacity: opacity,
          child: Transform.scale(scale: scale, child: child),
        );
      },
    );
  }
}

/// 商標的路徑，與網頁 `BrandLogoLoader.tsx` 與 `brand/hawkr/hawkr-mark.svg`
/// 同一份幾何：遮雨棚、遮雨棚上的兩條條紋、兩根柱子加檯面。
const String brandMarkAwningPath =
    'M28 24H92L102 46A10.5 10.5 0 0 1 81 46A10.5 10.5 0 0 1 60 46A10.5 10.5 0 0 1 39 46A10.5 10.5 0 0 1 18 46Z';
const String brandMarkStripePath =
    'M81 24H92L102 46A10.5 10.5 0 0 1 81 46ZM39 24H60V46A10.5 10.5 0 0 1 39 46Z';
const String brandMarkCounterPath =
    'M36 60A7 7 0 0 1 43 67V76H77V67A7 7 0 0 1 91 67V101A7 7 0 0 1 77 101V90H43V101A7 7 0 0 1 29 101V67A7 7 0 0 1 36 60Z';

String _fillSvg(String path) =>
    '<svg viewBox="0 0 120 120" xmlns="http://www.w3.org/2000/svg">'
    '<path d="$path" fill="#000000"/>'
    '</svg>';

/// 線稿的 `stroke-opacity` 由 srcIn 保留，因此仍是網頁的 0.7 半透明。
String _outlineSvg(String path) =>
    '<svg viewBox="0 0 120 120" fill="none" xmlns="http://www.w3.org/2000/svg">'
    '<path d="$path" stroke="#000000" stroke-width="2.4" '
    'stroke-opacity=".7" stroke-linejoin="round"/>'
    '</svg>';

final String _awningFillSvg = _fillSvg(brandMarkAwningPath);
final String _stripeFillSvg = _fillSvg(brandMarkStripePath);
final String _counterFillSvg = _fillSvg(brandMarkCounterPath);
final String _awningOutlineSvg = _outlineSvg(brandMarkAwningPath);
final String _counterOutlineSvg = _outlineSvg(brandMarkCounterPath);
