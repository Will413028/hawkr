import 'package:flutter/widgets.dart';
import 'package:flutter_hooks/flutter_hooks.dart';

/// 這個畫面是否看得到，由 widget 樹決定：被隱藏的主導航分頁，或被不透明頁面蓋住時，`TickerMode` 是關閉的。
/// 從看不到變成看得到時呼叫 [onVisible]；[includeInitial] 為真時，第一次建立就看得到也算（畫面剛開啟）。
/// 回傳目前是否看得到。
bool useOnBecameVisible(VoidCallback onVisible, {bool includeInitial = false}) {
  final visible = TickerMode.valuesOf(useContext()).enabled;
  final latest = useRef(onVisible)..value = onVisible;
  final previous = useRef<bool?>(null);
  useEffect(() {
    final before = previous.value;
    previous.value = visible;
    if (visible && (before == false || (before == null && includeInitial))) {
      latest.value();
    }
    return null;
  }, [visible]);
  return visible;
}
