import 'package:flutter/material.dart';

/// Shared semantic wrapper for custom icon controls.
///
/// Keeps the visual icon compact while enforcing the website/app 48dp touch
/// target and exposing one clear button node to assistive technologies.
class HkTapTarget extends StatelessWidget {
  const HkTapTarget({
    super.key,
    required this.semanticLabel,
    required this.child,
    this.onTap,
    this.onLongPress,
    this.checked,
    this.toggled,
    this.selected,
    this.hint,
    this.tooltip,
    this.button = true,
  });

  final String semanticLabel;
  final Widget child;
  final VoidCallback? onTap;
  final VoidCallback? onLongPress;
  final bool? checked;
  final bool? toggled;
  final bool? selected;
  final String? hint;
  final String? tooltip;
  final bool button;

  @override
  Widget build(BuildContext context) {
    final enabled = onTap != null || onLongPress != null;
    final target = GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: onTap,
      onLongPress: onLongPress,
      child: ConstrainedBox(
        constraints: const BoxConstraints(minWidth: 48, minHeight: 48),
        child: Center(child: child),
      ),
    );

    return Semantics(
      container: true,
      button: button,
      enabled: enabled,
      checked: checked,
      toggled: toggled,
      selected: selected,
      label: semanticLabel,
      hint: hint,
      tooltip: tooltip,
      onTap: onTap,
      onLongPress: onLongPress,
      child: ExcludeSemantics(
        child: tooltip == null
            ? target
            : Tooltip(message: tooltip!, child: target),
      ),
    );
  }
}
