import 'package:flutter/material.dart';

import '../../core/theme/app_colors.dart';
import '../../core/theme/hk_tokens.dart';

/// Visual weight for Hawkr's iOS-style functional glass layer.
///
/// [regular] is for navigation, toolbars and text-bearing transient surfaces.
/// [clear] is reserved for compact controls placed directly above rich media.
enum HkLiquidGlassVariant { regular, clear }

BorderSide _accessibleGlassBorderSide(BorderSide side) {
  if (side.style == BorderStyle.none) return BorderSide.none;
  return side.copyWith(
    color: AppColors.borderStrong,
    width: side.width < 1 ? 1 : side.width,
  );
}

BoxBorder _accessibleGlassBorder(BoxBorder? border) {
  if (border == null) {
    return Border.all(color: AppColors.borderStrong);
  }
  if (border is Border) {
    return Border(
      top: _accessibleGlassBorderSide(border.top),
      right: _accessibleGlassBorderSide(border.right),
      bottom: _accessibleGlassBorderSide(border.bottom),
      left: _accessibleGlassBorderSide(border.left),
    );
  }
  if (border is BorderDirectional) {
    return BorderDirectional(
      top: _accessibleGlassBorderSide(border.top),
      start: _accessibleGlassBorderSide(border.start),
      end: _accessibleGlassBorderSide(border.end),
      bottom: _accessibleGlassBorderSide(border.bottom),
    );
  }
  return Border.all(color: AppColors.borderStrong);
}

/// A bounded Liquid Glass approximation for Flutter surfaces.
///
/// Apple reserves Liquid Glass for controls and navigation rather than content
/// cards. Keep this widget clipped to a compact surface and place it over real
/// content so the backdrop blur has meaningful pixels to sample.
class HkLiquidGlassSurface extends StatelessWidget {
  const HkLiquidGlassSurface({
    super.key,
    required this.child,
    this.variant = HkLiquidGlassVariant.regular,
    this.backgroundColor,
    this.surfaceOpacity,
    this.blurSigma,
    this.borderRadius = const BorderRadius.all(Radius.circular(HkRadius.xl)),
    this.border,
    this.showBorder = true,
    this.shadows,
    this.padding,
    this.width,
    this.height,
    this.surfaceKey,
  });

  final Widget child;
  final HkLiquidGlassVariant variant;
  final Color? backgroundColor;
  final double? surfaceOpacity;
  final double? blurSigma;
  final BorderRadiusGeometry borderRadius;
  final BoxBorder? border;
  final bool showBorder;
  final List<BoxShadow>? shadows;
  final EdgeInsetsGeometry? padding;
  final double? width;
  final double? height;
  final Key? surfaceKey;

  @override
  Widget build(BuildContext context) {
    final highContrast = MediaQuery.maybeHighContrastOf(context) ?? false;
    final resolvedRadius = borderRadius.resolve(Directionality.of(context));
    final isClear = variant == HkLiquidGlassVariant.clear;
    final resolvedBlur = highContrast
        ? 0.0
        : blurSigma ??
              (isClear ? HkGlass.clearBlurSigma : HkGlass.regularBlurSigma);
    final resolvedOpacity = highContrast
        ? HkGlass.accessibleSurfaceOpacity
        : surfaceOpacity ??
              (isClear
                  ? HkGlass.clearSurfaceOpacity
                  : HkGlass.regularSurfaceOpacity);
    final resolvedColor =
        backgroundColor ??
        (isClear ? AppColors.neutralWhite : AppColors.topBarGlass);
    final resolvedBorder = !showBorder
        ? null
        : highContrast
        ? _accessibleGlassBorder(border)
        : border ??
              Border.all(
                color: AppColors.neutralWhite.withValues(
                  alpha: HkGlass.edgeHighlightOpacity,
                ),
                width: 0.75,
              );

    Widget content = SizedBox(
      width: width,
      height: height,
      child: DecoratedBox(
        key: surfaceKey,
        decoration: BoxDecoration(
          color: resolvedColor.withValues(alpha: resolvedOpacity),
          gradient: highContrast
              ? null
              : LinearGradient(
                  begin: Alignment.topLeft,
                  end: Alignment.bottomRight,
                  colors: [
                    AppColors.neutralWhite.withValues(
                      alpha: isClear
                          ? HkGlass.clearSpecularOpacity
                          : HkGlass.topBarHighlightStartOpacity,
                    ),
                    AppColors.neutralWhite.withValues(alpha: 0.08),
                    AppColors.actionPrimary.withValues(alpha: 0.025),
                  ],
                  stops: const [0, 0.52, 1],
                ),
          borderRadius: resolvedRadius,
          border: resolvedBorder,
        ),
        child: padding == null
            ? child
            : Padding(padding: padding!, child: child),
      ),
    );

    content = ClipRSuperellipse(
      borderRadius: resolvedRadius,
      child: BackdropFilter(
        enabled: resolvedBlur > 0,
        filterConfig: ImageFilterConfig.blur(
          sigmaX: resolvedBlur,
          sigmaY: resolvedBlur,
          bounded: true,
        ),
        child: content,
      ),
    );

    return RepaintBoundary(
      child: DecoratedBox(
        decoration: BoxDecoration(
          borderRadius: resolvedRadius,
          boxShadow: shadows,
        ),
        child: content,
      ),
    );
  }
}

/// Fixed-size Apple-style glass icon control with immediate press feedback.
///
/// The glass lens stays 44 logical pixels while the interaction target remains
/// 48 logical pixels on every device and text scale.
class HkLiquidGlassIconButton extends StatefulWidget {
  const HkLiquidGlassIconButton({
    super.key,
    required this.semanticLabel,
    required this.onPressed,
    required this.child,
    this.variant = HkLiquidGlassVariant.clear,
    this.backgroundColor,
    this.surfaceOpacity,
    this.surfaceKey,
  });

  final String semanticLabel;
  final VoidCallback? onPressed;
  final Widget child;
  final HkLiquidGlassVariant variant;
  final Color? backgroundColor;
  final double? surfaceOpacity;
  final Key? surfaceKey;

  @override
  State<HkLiquidGlassIconButton> createState() =>
      _HkLiquidGlassIconButtonState();
}

class _HkLiquidGlassIconButtonState extends State<HkLiquidGlassIconButton> {
  static const double _lensSize = 44;
  static const double _pressedScale = 0.94;
  static const Duration _pressInDuration = Duration(milliseconds: 90);
  static const Duration _pressOutDuration = Duration(milliseconds: 180);

  bool _pressed = false;

  void _setPressed(bool value) {
    if (_pressed == value) return;
    setState(() => _pressed = value);
  }

  @override
  void didUpdateWidget(covariant HkLiquidGlassIconButton oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (widget.onPressed == null) _pressed = false;
  }

  @override
  Widget build(BuildContext context) {
    final enabled = widget.onPressed != null;
    final platformReduceMotion = View.of(
      context,
    ).platformDispatcher.accessibilityFeatures.reduceMotion;
    final reduceMotion =
        (MediaQuery.maybeDisableAnimationsOf(context) ?? false) ||
        platformReduceMotion;
    final duration = reduceMotion
        ? Duration.zero
        : _pressed
        ? _pressInDuration
        : _pressOutDuration;

    return Semantics(
      button: true,
      enabled: enabled,
      label: widget.semanticLabel,
      onTap: widget.onPressed,
      child: ExcludeSemantics(
        child: MouseRegion(
          cursor: enabled ? SystemMouseCursors.click : MouseCursor.defer,
          child: Listener(
            onPointerDown: enabled && !reduceMotion
                ? (_) => _setPressed(true)
                : null,
            onPointerUp: (_) => _setPressed(false),
            onPointerCancel: (_) => _setPressed(false),
            child: GestureDetector(
              behavior: HitTestBehavior.opaque,
              onTap: widget.onPressed,
              child: SizedBox.square(
                dimension: HkControl.minTap,
                child: Center(
                  child: AnimatedScale(
                    key: const ValueKey('hk-liquid-glass-control-scale'),
                    scale: _pressed && !reduceMotion ? _pressedScale : 1,
                    duration: duration,
                    curve: Curves.easeOutCubic,
                    child: HkLiquidGlassSurface(
                      variant: widget.variant,
                      backgroundColor: widget.backgroundColor,
                      surfaceOpacity: widget.surfaceOpacity,
                      width: _lensSize,
                      height: _lensSize,
                      surfaceKey: widget.surfaceKey,
                      borderRadius: BorderRadius.circular(HkRadius.pill),
                      shadows: HkShadows.sm,
                      child: Material(
                        type: MaterialType.transparency,
                        child: Center(child: widget.child),
                      ),
                    ),
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
