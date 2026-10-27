import 'package:flutter/material.dart';

/// Dismisses the active text input when the user touches outside it.
///
/// A pointer listener is used instead of an outer tap recognizer so child
/// buttons, scrollables, and text fields keep their normal gesture behavior.
class KeyboardDismissScope extends StatelessWidget {
  const KeyboardDismissScope({super.key, required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context) {
    return Listener(
      behavior: HitTestBehavior.deferToChild,
      onPointerDown: (_) => FocusManager.instance.primaryFocus?.unfocus(),
      child: child,
    );
  }
}
