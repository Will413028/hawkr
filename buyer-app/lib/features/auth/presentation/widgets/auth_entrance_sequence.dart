import 'package:flutter/material.dart';

import '../../../../core/theme/hk_tokens.dart';

/// Stages in the login screen's single, coordinated entrance timeline.
enum AuthEntranceStage {
  wordmark,
  moon,
  panel,
  headline,
  google,
  apple,
  line,
  footer,
}

/// Owns the one controller used by every entrance stage on the auth screen.
///
/// Children stay mounted throughout the animation so semantics order and hit
/// testing never depend on a delayed insertion.
class AuthEntranceSequence extends StatefulWidget {
  const AuthEntranceSequence({super.key, required this.child});

  static const duration = Duration(milliseconds: 840);

  final Widget child;

  @override
  State<AuthEntranceSequence> createState() => _AuthEntranceSequenceState();
}

class _AuthEntranceSequenceState extends State<AuthEntranceSequence>
    with SingleTickerProviderStateMixin {
  late final AnimationController _controller = AnimationController(
    vsync: this,
    duration: AuthEntranceSequence.duration,
  );
  bool _hasResolvedEntrance = false;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final disableAnimations = MediaQuery.disableAnimationsOf(context);
    // ignore: deprecated_member_use
    final tickerEnabled = TickerMode.of(context);

    if (disableAnimations || !tickerEnabled) {
      _controller
        ..stop(canceled: false)
        ..value = 1;
      _hasResolvedEntrance = true;
      return;
    }

    if (!_hasResolvedEntrance) {
      _hasResolvedEntrance = true;
      _controller.forward();
    }
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return _AuthEntranceScope(animation: _controller, child: widget.child);
  }
}

class AuthEntranceItem extends StatelessWidget {
  const AuthEntranceItem({super.key, required this.stage, required this.child});

  final AuthEntranceStage stage;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    final animation = _AuthEntranceScope.maybeAnimationOf(context);
    if (animation == null) return child;

    final spec = _specFor(stage);
    final interval = Interval(spec.start, spec.end, curve: HkMotion.ease);

    return AnimatedBuilder(
      animation: animation,
      child: child,
      builder: (context, staticChild) {
        final value = interval.transform(animation.value);
        return ExcludeFocus(
          excluding: value == 0,
          child: IgnorePointer(
            ignoring: value == 0,
            child: Opacity(
              opacity: value,
              child: Transform.translate(
                key: ValueKey('auth-entrance-${stage.name}'),
                offset: Offset(0, spec.translateY * (1 - value)),
                child: Transform.scale(
                  scale: spec.beginScale + (1 - spec.beginScale) * value,
                  child: staticChild,
                ),
              ),
            ),
          ),
        );
      },
    );
  }
}

class _AuthEntranceScope extends InheritedWidget {
  const _AuthEntranceScope({required this.animation, required super.child});

  final Animation<double> animation;

  static Animation<double>? maybeAnimationOf(BuildContext context) => context
      .dependOnInheritedWidgetOfExactType<_AuthEntranceScope>()
      ?.animation;

  @override
  bool updateShouldNotify(_AuthEntranceScope oldWidget) =>
      oldWidget.animation != animation;
}

class _AuthEntranceSpec {
  const _AuthEntranceSpec({
    required this.start,
    required this.end,
    required this.translateY,
    this.beginScale = 1,
  });

  final double start;
  final double end;
  final double translateY;
  final double beginScale;
}

_AuthEntranceSpec _specFor(AuthEntranceStage stage) => switch (stage) {
  AuthEntranceStage.wordmark => const _AuthEntranceSpec(
    start: 0,
    end: 0.34,
    translateY: 8,
  ),
  AuthEntranceStage.moon => const _AuthEntranceSpec(
    start: 0.08,
    end: 0.60,
    translateY: 18,
    beginScale: 0.96,
  ),
  AuthEntranceStage.panel => const _AuthEntranceSpec(
    start: 0.18,
    end: 0.70,
    translateY: 24,
    beginScale: 0.975,
  ),
  AuthEntranceStage.headline => const _AuthEntranceSpec(
    start: 0.30,
    end: 0.76,
    translateY: 10,
  ),
  AuthEntranceStage.google => const _AuthEntranceSpec(
    start: 0.44,
    end: 0.84,
    translateY: 12,
  ),
  AuthEntranceStage.apple => const _AuthEntranceSpec(
    start: 0.52,
    end: 0.92,
    translateY: 12,
  ),
  AuthEntranceStage.line => const _AuthEntranceSpec(
    start: 0.60,
    end: 1,
    translateY: 12,
  ),
  AuthEntranceStage.footer => const _AuthEntranceSpec(
    start: 0.66,
    end: 1,
    translateY: 8,
  ),
};
