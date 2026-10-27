import 'package:flutter/foundation.dart';

/// 控制全域點擊自動收鍵盤的機制。
///
/// 有些畫面（例如聊天）需要讓鍵盤長時間維持開啟，
/// 此時可以呼叫 [KeyboardUnfocusController.pushOverride] 來暫停全域自動 unfocus，
/// 並在畫面離開時呼叫 [KeyboardUnfocusController.popOverride] 解除。
class KeyboardUnfocusController {
  KeyboardUnfocusController._();

  static final ValueNotifier<int> _overrideCounter = ValueNotifier<int>(0);

  static ValueListenable<int> get overrideCounterListenable => _overrideCounter;

  /// 啟用抑制。可重入，需搭配 [popOverride] 成對呼叫。
  static void pushOverride() {
    _overrideCounter.value = _overrideCounter.value + 1;
  }

  /// 解除抑制。
  static void popOverride() {
    if (_overrideCounter.value == 0) return;
    _overrideCounter.value = _overrideCounter.value - 1;
  }

  /// 目前是否允許全域自動 unfocus。
  static bool get autoUnfocusEnabled => _overrideCounter.value == 0;
}
