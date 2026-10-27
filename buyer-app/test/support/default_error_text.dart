import 'package:hawkr_api/transport.dart' show Code;
import 'package:hawkr_ui_text/hawkr_ui_text.dart';

/// 測試用的 Connect code 預設文字：與 App 一樣取自共用文案目錄（zh-TW）。
String zhDefaultErrorText(Code code) => commonErrorText(code, UiLocale.zhTW)!;
