import 'package:flutter/widgets.dart';

/// Keeps chat navigation from inheriting an active text-input connection from
/// the previous route.
abstract final class ChatFocusPolicy {
  static void dismissKeyboard() {
    FocusManager.instance.primaryFocus?.unfocus(
      disposition: UnfocusDisposition.scope,
    );
  }

  /// Route transitions can complete one frame after the source route loses
  /// focus on iOS. Clear once immediately and once after the destination frame
  /// so the chat composer only opens after an explicit user tap.
  static void keepClosedOnEntry(FocusNode inputFocusNode) {
    inputFocusNode.unfocus(disposition: UnfocusDisposition.scope);
    dismissKeyboard();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      inputFocusNode.unfocus(disposition: UnfocusDisposition.scope);
      dismissKeyboard();
    });
  }
}
