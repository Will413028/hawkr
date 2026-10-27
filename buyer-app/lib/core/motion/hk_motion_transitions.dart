import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';

/// Presentation semantics for transient surfaces.
///
/// Keeping these explicit prevents a photo viewer, contextual popup and
/// full-screen task from accidentally sharing the same generic animation.
enum HkModalPresentation { popup, photoZoom, fullscreenDialog }

/// App-wide page-transition theme.
///
/// App 只發 iOS：每個平台都用 Flutter 的 Cupertino builder，讓 edge-swipe 返回保持
/// 可互動；測試環境（預設 android）也因此跑出貨的轉場。
abstract final class HkPageTransitions {
  static const theme = PageTransitionsTheme(
    builders: <TargetPlatform, PageTransitionsBuilder>{
      TargetPlatform.android: CupertinoPageTransitionsBuilder(),
      TargetPlatform.fuchsia: CupertinoPageTransitionsBuilder(),
      TargetPlatform.linux: CupertinoPageTransitionsBuilder(),
      TargetPlatform.windows: CupertinoPageTransitionsBuilder(),
      TargetPlatform.iOS: CupertinoPageTransitionsBuilder(),
      TargetPlatform.macOS: CupertinoPageTransitionsBuilder(),
    },
  );
}

abstract final class HkMotionTransitions {
  /// Flutter's Cupertino modal-popup timing. The pinned Flutter 3.41 SDK tunes
  /// the matching route with a critically damped iOS spring.
  static const Duration cupertinoPopupDuration = Duration(milliseconds: 335);

  static const Curve _cupertinoForwardCurve = Curves.fastEaseInToSlowEaseOut;
  static const Curve _cupertinoReverseCurve = FlippedCurve(
    Curves.fastEaseInToSlowEaseOut,
  );

  static bool reduceMotionOf(BuildContext context) =>
      (MediaQuery.maybeOf(context)?.disableAnimations ?? false) ||
      View.of(context).platformDispatcher.accessibilityFeatures.reduceMotion;

  static bool _disabled(BuildContext context) => reduceMotionOf(context);

  /// Resolves modal timing from the presentation and reduced-motion policy.
  static Duration durationOf(
    BuildContext context, {
    HkModalPresentation presentation = HkModalPresentation.popup,
  }) {
    if (_disabled(context)) return Duration.zero;
    return presentation == HkModalPresentation.fullscreenDialog
        ? CupertinoRouteTransitionMixin.kTransitionDuration
        : cupertinoPopupDuration;
  }

  /// Maps a global press location to the transform origin used by contextual
  /// previews, matching the direct-manipulation feel of iOS context menus.
  static Alignment alignmentForGlobalPosition(
    BuildContext context,
    Offset position,
  ) {
    final size = MediaQuery.sizeOf(context);
    if (size.width <= 0 || size.height <= 0) return Alignment.center;
    return Alignment(
      ((position.dx / size.width) * 2 - 1).clamp(-1.0, 1.0).toDouble(),
      ((position.dy / size.height) * 2 - 1).clamp(-1.0, 1.0).toDouble(),
    );
  }

  /// Compact entrance for dialogs, menus, sheets and lightboxes.
  ///
  /// The barrier still provides depth; the foreground itself uses only
  /// compositor-friendly translate/scale motion, avoiding a dated full fade.
  static Widget modal({
    required BuildContext context,
    required Animation<double> animation,
    required Widget child,
    Animation<double> secondaryAnimation = kAlwaysDismissedAnimation,
    HkModalPresentation presentation = HkModalPresentation.popup,
    Alignment? alignment,
    Offset? offset,
    double? beginScale,
  }) {
    if (_disabled(context)) return child;

    if (presentation == HkModalPresentation.fullscreenDialog) {
      return CupertinoFullscreenDialogTransition(
        key: const ValueKey('hk-cupertino-fullscreen-dialog-transition'),
        primaryRouteAnimation: animation,
        secondaryRouteAnimation: secondaryAnimation,
        linearTransition: false,
        child: child,
      );
    }

    final resolvedAlignment = alignment ?? Alignment.center;
    final resolvedOffset =
        offset ??
        switch (presentation) {
          HkModalPresentation.popup => const Offset(0, 8),
          HkModalPresentation.photoZoom => Offset.zero,
          HkModalPresentation.fullscreenDialog => const Offset(0, 12),
        };
    final resolvedBeginScale =
        beginScale ??
        switch (presentation) {
          HkModalPresentation.popup => 0.985,
          HkModalPresentation.photoZoom => 0.96,
          HkModalPresentation.fullscreenDialog => 1.0,
        };

    return _HkCurvedProgress(
      animation: animation,
      curve: _cupertinoForwardCurve,
      reverseCurve: _cupertinoReverseCurve,
      child: child,
      builder: (context, progress, staticChild) {
        return KeyedSubtree(
          key: ValueKey('hk-modal-presentation-${presentation.name}'),
          child: Transform.translate(
            key: const ValueKey('hk-modal-motion-translate'),
            offset: Offset(
              resolvedOffset.dx * (1 - progress),
              resolvedOffset.dy * (1 - progress),
            ),
            child: Transform.scale(
              key: const ValueKey('hk-modal-motion-scale'),
              scale: resolvedBeginScale + ((1 - resolvedBeginScale) * progress),
              alignment: resolvedAlignment,
              child: staticChild,
            ),
          ),
        );
      },
    );
  }
}

typedef _HkProgressBuilder =
    Widget Function(BuildContext context, double progress, Widget child);

/// Owns [CurvedAnimation] so a transition that reverses before reaching an end
/// keeps its active curve and cannot jump between two transformed values.
class _HkCurvedProgress extends StatefulWidget {
  const _HkCurvedProgress({
    required this.animation,
    required this.curve,
    required this.reverseCurve,
    required this.child,
    required this.builder,
  });

  final Animation<double> animation;
  final Curve curve;
  final Curve reverseCurve;
  final Widget child;
  final _HkProgressBuilder builder;

  @override
  State<_HkCurvedProgress> createState() => _HkCurvedProgressState();
}

class _HkCurvedProgressState extends State<_HkCurvedProgress> {
  late CurvedAnimation _progress;

  @override
  void initState() {
    super.initState();
    _createProgress();
  }

  @override
  void didUpdateWidget(_HkCurvedProgress oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.animation != widget.animation ||
        oldWidget.curve != widget.curve ||
        oldWidget.reverseCurve != widget.reverseCurve) {
      _progress.dispose();
      _createProgress();
    }
  }

  void _createProgress() {
    _progress = CurvedAnimation(
      parent: widget.animation,
      curve: widget.curve,
      reverseCurve: widget.reverseCurve,
    );
  }

  @override
  void dispose() {
    _progress.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: _progress,
      child: widget.child,
      builder: (context, staticChild) => widget.builder(
        context,
        _progress.value.clamp(0.0, 1.0).toDouble(),
        staticChild!,
      ),
    );
  }
}
