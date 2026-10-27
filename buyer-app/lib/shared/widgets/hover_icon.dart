import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'dart:math' as math;

import 'package:flutter/material.dart';

enum HoverAnimation { slideThrough, launch, pulse, roll, swing, bounce, spin }

class HoverIcon extends StatefulWidget {
  const HoverIcon({
    super.key,
    required this.icon,
    required this.animation,
    this.size = 24,
    this.color,
    this.autoPlay = false,
    this.repeatInterval = const Duration(seconds: 3),
    this.duration = const Duration(milliseconds: 1000),
  });

  final IconData icon;
  final HoverAnimation animation;
  final double size;
  final Color? color;
  final bool autoPlay;
  final Duration repeatInterval;
  final Duration duration;

  @override
  State<HoverIcon> createState() => HoverIconState();
}

class HoverIconState extends State<HoverIcon>
    with SingleTickerProviderStateMixin {
  late AnimationController _ctrl;
  bool _looping = false;

  @override
  void initState() {
    super.initState();
    _ctrl = AnimationController(vsync: this, duration: widget.duration);
    if (widget.autoPlay) _startLoop();
  }

  @override
  void didUpdateWidget(HoverIcon oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.duration != widget.duration) {
      _ctrl.duration = widget.duration;
    }
    if (!oldWidget.autoPlay && widget.autoPlay) {
      _startLoop();
    } else if (oldWidget.autoPlay && !widget.autoPlay) {
      _looping = false;
    }
  }

  void _startLoop() {
    _looping = true;
    _runLoop();
  }

  Future<void> _runLoop() async {
    while (_looping && mounted) {
      await playOnce();
      if (!_looping || !mounted) break;
      await Future.delayed(widget.repeatInterval);
    }
  }

  Future<void> playOnce() async {
    if (!mounted) return;
    _ctrl.reset();
    await _ctrl.forward();
  }

  @override
  void dispose() {
    _looping = false;
    _ctrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final color = widget.color ?? AppColors.textPrimary;
    final child = Icon(widget.icon, size: widget.size, color: color);

    return SizedBox(
      width: widget.size,
      height: widget.size,
      child: ClipRect(
        child: AnimatedBuilder(
          animation: _ctrl,
          builder: (context, c) => _applyAnimation(c!),
          child: child,
        ),
      ),
    );
  }

  Widget _applyAnimation(Widget child) {
    switch (widget.animation) {
      case HoverAnimation.slideThrough:
        return _slideThrough(child);
      case HoverAnimation.launch:
        return _launch(child);
      case HoverAnimation.pulse:
        return _pulse(child);
      case HoverAnimation.roll:
        return _roll(child);
      case HoverAnimation.swing:
        return _swing(child);
      case HoverAnimation.bounce:
        return _bounce(child);
      case HoverAnimation.spin:
        return _spin(child);
    }
  }

  Widget _slideThrough(Widget child) {
    final t = _ctrl.value;
    final double dx;
    final double opacity;
    if (t <= 0.45) {
      final p = Curves.easeIn.transform(t / 0.45);
      dx = 30 * p;
      opacity = 1.0 - p;
    } else if (t <= 0.55) {
      dx = -30;
      opacity = 0;
    } else {
      final p = Curves.easeOut.transform((t - 0.55) / 0.45);
      dx = -30 + 30 * p;
      opacity = p;
    }
    return Transform.translate(
      offset: Offset(dx, 0),
      child: Opacity(opacity: opacity.clamp(0.0, 1.0), child: child),
    );
  }

  Widget _launch(Widget child) {
    final t = _ctrl.value;
    final double dx, dy, opacity;
    if (t <= 0.35) {
      final p = Curves.easeIn.transform(t / 0.35);
      dx = 40 * p;
      dy = -40 * p;
      opacity = 1.0 - p;
    } else if (t <= 0.55) {
      dx = -40;
      dy = 40;
      opacity = 0;
    } else {
      final p = Curves.easeOut.transform((t - 0.55) / 0.45);
      dx = -40 + 40 * p;
      dy = 40 - 40 * p;
      opacity = p;
    }
    return Transform.translate(
      offset: Offset(dx, dy),
      child: Opacity(opacity: opacity.clamp(0.0, 1.0), child: child),
    );
  }

  Widget _pulse(Widget child) {
    final t = _ctrl.value;
    final double scale;
    if (t <= 0.25) {
      scale = 1.0 + 0.15 * Curves.easeOut.transform(t / 0.25);
    } else if (t <= 0.5) {
      scale = 1.15 - 0.15 * Curves.easeIn.transform((t - 0.25) / 0.25);
    } else if (t <= 0.75) {
      scale = 1.0 + 0.25 * Curves.easeOut.transform((t - 0.5) / 0.25);
    } else {
      scale = 1.25 - 0.25 * Curves.easeIn.transform((t - 0.75) / 0.25);
    }
    return Transform.scale(scale: scale, child: child);
  }

  Widget _roll(Widget child) {
    final t = _ctrl.value;
    final dx = 10.0 * math.sin(t * math.pi);
    return Transform.translate(offset: Offset(dx, 0), child: child);
  }

  Widget _swing(Widget child) {
    final t = _ctrl.value;
    final double angle;
    if (t <= 0.2) {
      angle = -15 * Curves.easeOut.transform(t / 0.2);
    } else if (t <= 0.4) {
      angle = -15 + 30 * Curves.easeInOut.transform((t - 0.2) / 0.2);
    } else if (t <= 0.6) {
      angle = 15 - 25 * Curves.easeInOut.transform((t - 0.4) / 0.2);
    } else if (t <= 0.8) {
      angle = -10 + 20 * Curves.easeInOut.transform((t - 0.6) / 0.2);
    } else {
      angle = 10 - 10 * Curves.easeOut.transform((t - 0.8) / 0.2);
    }
    return Transform.rotate(
      angle: angle * math.pi / 180,
      alignment: Alignment.topCenter,
      child: child,
    );
  }

  Widget _bounce(Widget child) {
    final t = _ctrl.value.clamp(0.0, 1.0);
    final double dy;
    if (t <= 0.4) {
      dy = -12 * Curves.easeOut.transform((t / 0.4).clamp(0.0, 1.0));
    } else if (t <= 0.7) {
      dy =
          -12 + 12 * Curves.easeIn.transform(((t - 0.4) / 0.3).clamp(0.0, 1.0));
    } else if (t <= 0.85) {
      dy = -4 * Curves.easeOut.transform(((t - 0.7) / 0.15).clamp(0.0, 1.0));
    } else {
      dy =
          -4 + 4 * Curves.easeIn.transform(((t - 0.85) / 0.15).clamp(0.0, 1.0));
    }
    return Transform.translate(offset: Offset(0, dy), child: child);
  }

  Widget _spin(Widget child) {
    final angle = Curves.easeInOut.transform(_ctrl.value) * 2 * math.pi;
    return Transform.rotate(angle: angle, child: child);
  }
}
