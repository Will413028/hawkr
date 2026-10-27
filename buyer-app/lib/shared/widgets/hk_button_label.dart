import 'package:flutter/material.dart';

/// Canonical label for text-bearing buttons.
///
/// Button copy always occupies one line. When a translated or dynamic label is
/// wider than the available control, the visual text is ellipsized while the
/// complete string remains available to the semantics tree.
class HkButtonLabel extends StatelessWidget {
  const HkButtonLabel(
    this.label, {
    super.key,
    this.style,
    this.textAlign = TextAlign.center,
  });

  final String label;
  final TextStyle? style;
  final TextAlign textAlign;

  @override
  Widget build(BuildContext context) {
    return Text(
      label,
      maxLines: 1,
      softWrap: false,
      overflow: TextOverflow.ellipsis,
      textAlign: textAlign,
      style: style,
    );
  }
}
