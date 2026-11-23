import 'package:hawkr_seller/shared/time/taipei_display.dart';

/// 裝置在 +8：顯示臺北時間時不附標籤。
const taipeiDeviceDisplay = TaipeiDisplay(
  label: '台北時間',
  deviceOffset: Duration(hours: 8),
);

/// 裝置在 UTC-5：顯示臺北時間時附「台北時間」。
const foreignDeviceDisplay = TaipeiDisplay(
  label: '台北時間',
  deviceOffset: Duration(hours: -5),
);
