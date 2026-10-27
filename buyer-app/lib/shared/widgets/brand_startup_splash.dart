import 'dart:async';
import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../../core/theme/app_colors.dart';
import 'package:flutter_svg/flutter_svg.dart';
import 'hk_brand_loader.dart';

/// Runs application bootstrap work behind the same branded first-paint loader
/// used by the web app. The real application is mounted underneath the splash
/// before the overlay fades, preventing a blank frame during hand-off.
class BrandStartupGate extends StatefulWidget {
  const BrandStartupGate({
    super.key,
    required this.bootstrap,
    this.minimumDisplayDuration = const Duration(milliseconds: 700),
    this.fadeDuration = const Duration(milliseconds: 360),
    this.onComplete,
  });

  final Future<Widget> Function() bootstrap;
  final Duration minimumDisplayDuration;
  final Duration fadeDuration;
  final VoidCallback? onComplete;

  @override
  State<BrandStartupGate> createState() => _BrandStartupGateState();
}

class _BrandStartupGateState extends State<BrandStartupGate>
    with SingleTickerProviderStateMixin {
  late final AnimationController _fadeController;
  late final Stopwatch _shownFor;
  Widget? _application;
  bool _showSplash = true;

  @override
  void initState() {
    super.initState();
    _shownFor = Stopwatch()..start();
    _fadeController = AnimationController(
      vsync: this,
      duration: widget.fadeDuration,
    );
    unawaited(_runBootstrap());
  }

  Future<void> _runBootstrap() async {
    try {
      final application = await widget.bootstrap();
      if (!mounted) return;

      setState(() => _application = application);
      await WidgetsBinding.instance.endOfFrame;

      final remaining = widget.minimumDisplayDuration - _shownFor.elapsed;
      if (remaining > Duration.zero) await Future<void>.delayed(remaining);
      if (!mounted) return;

      final reduceMotion = View.of(
        context,
      ).platformDispatcher.accessibilityFeatures.disableAnimations;
      if (reduceMotion) {
        _fadeController.value = 1;
      } else {
        await _fadeController.forward();
      }
      if (!mounted) return;
      setState(() => _showSplash = false);
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) widget.onComplete?.call();
      });
    } catch (error, stackTrace) {
      FlutterError.reportError(
        FlutterErrorDetails(
          exception: error,
          stack: stackTrace,
          library: 'Hawkr startup',
          context: ErrorDescription('while bootstrapping the application'),
        ),
      );
    }
  }

  @override
  void dispose() {
    _shownFor.stop();
    _fadeController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (!_showSplash && _application != null) return _application!;

    final splash = MediaQuery.fromView(
      view: View.of(context),
      child: const Directionality(
        textDirection: TextDirection.ltr,
        child: BrandStartupSplash(),
      ),
    );

    return Stack(
      textDirection: TextDirection.ltr,
      fit: StackFit.expand,
      children: [
        if (_application != null) _application!,
        FadeTransition(
          opacity: ReverseAnimation(_fadeController),
          child: splash,
        ),
      ],
    );
  }
}

/// Pixel-matched Flutter port of the web initial loader.
class BrandStartupSplash extends StatefulWidget {
  const BrandStartupSplash({super.key});

  @override
  State<BrandStartupSplash> createState() => _BrandStartupSplashState();
}

class _BrandStartupSplashState extends State<BrandStartupSplash>
    with SingleTickerProviderStateMixin {
  late final AnimationController _loopController;
  bool? _reduceMotion;

  @override
  void initState() {
    super.initState();
    _loopController = AnimationController(
      vsync: this,
      duration: HkBrandMark.loopDuration,
    );
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final reduceMotion = MediaQuery.disableAnimationsOf(context);
    if (_reduceMotion == reduceMotion) return;
    _reduceMotion = reduceMotion;
    if (reduceMotion) {
      _loopController
        ..stop()
        ..value = 0.85;
    } else {
      _loopController.repeat();
    }
  }

  @override
  void dispose() {
    _loopController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final dark = MediaQuery.platformBrightnessOf(context) == Brightness.dark;
    final overlayStyle = dark
        ? SystemUiOverlayStyle.light
        : SystemUiOverlayStyle.dark;

    return AnnotatedRegion<SystemUiOverlayStyle>(
      value: overlayStyle.copyWith(
        statusBarColor: Colors.transparent,
        systemNavigationBarColor: dark
            ? AppColors.neutralInk
            : AppColors.neutralMuted,
      ),
      child: Semantics(
        container: true,
        liveRegion: true,
        label: 'Hawkr 載入中',
        child: ColoredBox(
          color: dark ? AppColors.neutralInk : AppColors.neutralCream,
          child: Stack(
            fit: StackFit.expand,
            children: [
              _SplashBackground(dark: dark),
              SafeArea(
                child: Center(
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      HkBrandMark(
                        animation: _loopController,
                        size: 88,
                        tone: dark
                            ? HkBrandLoaderTone.brandOnDark
                            : HkBrandLoaderTone.brand,
                      ),
                      const SizedBox(height: 16),
                      _AnimatedWordmark(animation: _loopController, dark: dark),
                    ],
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _SplashBackground extends StatelessWidget {
  const _SplashBackground({required this.dark});

  final bool dark;

  @override
  Widget build(BuildContext context) {
    return Stack(
      fit: StackFit.expand,
      children: [
        DecoratedBox(
          decoration: BoxDecoration(
            gradient: LinearGradient(
              begin: Alignment.topLeft,
              end: Alignment.bottomRight,
              colors: dark
                  ? const [
                      AppColors.neutralInk,
                      AppColors.brandPrimaryStrong,
                      AppColors.neutralBody,
                    ]
                  : const [
                      AppColors.neutralCream,
                      AppColors.neutralMuted,
                      AppColors.brandSecondarySoft,
                    ],
              stops: const [0, 0.54, 1],
            ),
          ),
        ),
        Positioned(
          left: -72,
          top: -104,
          width: 340,
          height: 340,
          child: DecoratedBox(
            decoration: BoxDecoration(
              gradient: RadialGradient(
                colors: dark
                    ? const [Color(0x1AF1CD70), Color(0x00F1CD70)]
                    : const [Color(0x42D8B457), Color(0x00D8B457)],
              ),
            ),
          ),
        ),
        Positioned(
          right: -96,
          bottom: -84,
          width: 360,
          height: 360,
          child: DecoratedBox(
            decoration: BoxDecoration(
              gradient: RadialGradient(
                colors: dark
                    ? const [Color(0x1F7AA2C0), Color(0x007AA2C0)]
                    : const [Color(0x57B4D7E8), Color(0x00B4D7E8)],
              ),
            ),
          ),
        ),
        CustomPaint(painter: _GridPainter(dark: dark)),
      ],
    );
  }
}

class _GridPainter extends CustomPainter {
  const _GridPainter({required this.dark});

  final bool dark;

  @override
  void paint(Canvas canvas, Size size) {
    const spacing = 28.0;
    final base = dark ? AppColors.brandSecondary : AppColors.brandPrimaryStrong;
    final paint = Paint()..strokeWidth = 1;

    for (double y = 0; y <= size.height; y += spacing) {
      final fade = 1 - (y / math.max(size.height, 1)) * 0.84;
      paint.color = base.withValues(alpha: (dark ? 0.05 : 0.045) * fade);
      canvas.drawLine(Offset(0, y), Offset(size.width, y), paint);
    }
    for (double x = 0; x <= size.width; x += spacing) {
      paint.color = base.withValues(alpha: dark ? 0.035 : 0.032);
      canvas.drawLine(Offset(x, 0), Offset(x, size.height), paint);
    }
  }

  @override
  bool shouldRepaint(covariant _GridPainter oldDelegate) =>
      oldDelegate.dark != dark;
}

class _AnimatedWordmark extends StatelessWidget {
  const _AnimatedWordmark({required this.animation, required this.dark});

  final Animation<double> animation;
  final bool dark;

  @override
  Widget build(BuildContext context) {
    final wordmark = SvgPicture.asset(
      'assets/brand/wordmark.svg',
      fit: BoxFit.contain,
    );
    return SizedBox(
      width: 158,
      height: 64,
      child: Stack(
        fit: StackFit.expand,
        children: [
          ColorFiltered(
            colorFilter: ColorFilter.mode(
              dark ? const Color(0x526F746F) : const Color(0x66555555),
              BlendMode.srcIn,
            ),
            child: SvgPicture.asset(
              'assets/brand/wordmark.svg',
              fit: BoxFit.contain,
            ),
          ),
          AnimatedBuilder(
            animation: animation,
            child: wordmark,
            builder: (context, child) {
              final t = animation.value;
              final reveal = Curves.easeInOut.transform(
                (t / 0.62).clamp(0.0, 1.0),
              );
              final opacity =
                  1 -
                  Curves.easeIn.transform(((t - 0.86) / 0.14).clamp(0.0, 1.0));
              final featherStart = math.max(0.0, reveal - 0.08);

              return Opacity(
                opacity: opacity,
                child: ShaderMask(
                  blendMode: BlendMode.dstIn,
                  shaderCallback: (bounds) => LinearGradient(
                    colors: const [
                      Colors.black,
                      Colors.black,
                      Colors.transparent,
                    ],
                    stops: [0, featherStart, math.max(reveal, 0.001)],
                  ).createShader(bounds),
                  child: child,
                ),
              );
            },
          ),
        ],
      ),
    );
  }
}
