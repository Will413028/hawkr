import 'package:flutter/widgets.dart' show BuildContext;

import 'gen/app_localizations.dart';

export 'gen/app_localizations.dart';

/// widget 取 App 自己的 ARB 文字：`context.l10n.xxx`（規則 E 認得這個形式與 `AppLocalizations.of(context)`）。
extension AppL10n on BuildContext {
  AppLocalizations get l10n => AppLocalizations.of(this);
}
