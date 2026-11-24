import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:intl/intl.dart' as intl;

import 'app_localizations_en.dart';
import 'app_localizations_zh.dart';

// ignore_for_file: type=lint

/// Callers can lookup localized strings with an instance of AppLocalizations
/// returned by `AppLocalizations.of(context)`.
///
/// Applications need to include `AppLocalizations.delegate()` in their app's
/// `localizationDelegates` list, and the locales they support in the app's
/// `supportedLocales` list. For example:
///
/// ```dart
/// import 'gen/app_localizations.dart';
///
/// return MaterialApp(
///   localizationsDelegates: AppLocalizations.localizationsDelegates,
///   supportedLocales: AppLocalizations.supportedLocales,
///   home: MyApplicationHome(),
/// );
/// ```
///
/// ## Update pubspec.yaml
///
/// Please make sure to update your pubspec.yaml to include the following
/// packages:
///
/// ```yaml
/// dependencies:
///   # Internationalization support.
///   flutter_localizations:
///     sdk: flutter
///   intl: any # Use the pinned version from flutter_localizations
///
///   # Rest of dependencies
/// ```
///
/// ## iOS Applications
///
/// iOS applications define key application metadata, including supported
/// locales, in an Info.plist file that is built into the application bundle.
/// To configure the locales supported by your app, you’ll need to edit this
/// file.
///
/// First, open your project’s ios/Runner.xcworkspace Xcode workspace file.
/// Then, in the Project Navigator, open the Info.plist file under the Runner
/// project’s Runner folder.
///
/// Next, select the Information Property List item, select Add Item from the
/// Editor menu, then select Localizations from the pop-up menu.
///
/// Select and expand the newly-created Localizations item then, for each
/// locale your application supports, add a new item and select the locale
/// you wish to add from the pop-up menu in the Value field. This list should
/// be consistent with the languages listed in the AppLocalizations.supportedLocales
/// property.
abstract class AppLocalizations {
  AppLocalizations(String locale)
    : localeName = intl.Intl.canonicalizedLocale(locale.toString());

  final String localeName;

  static AppLocalizations of(BuildContext context) {
    return Localizations.of<AppLocalizations>(context, AppLocalizations)!;
  }

  static const LocalizationsDelegate<AppLocalizations> delegate =
      _AppLocalizationsDelegate();

  /// A list of this localizations delegate along with the default localizations
  /// delegates.
  ///
  /// Returns a list of localizations delegates containing this delegate along with
  /// GlobalMaterialLocalizations.delegate, GlobalCupertinoLocalizations.delegate,
  /// and GlobalWidgetsLocalizations.delegate.
  ///
  /// Additional delegates can be added by appending to this list in
  /// MaterialApp. This list does not have to be used at all if a custom list
  /// of delegates is preferred or required.
  static const List<LocalizationsDelegate<dynamic>> localizationsDelegates =
      <LocalizationsDelegate<dynamic>>[
        delegate,
        GlobalMaterialLocalizations.delegate,
        GlobalCupertinoLocalizations.delegate,
        GlobalWidgetsLocalizations.delegate,
      ];

  /// A list of this localizations delegate's supported locales.
  static const List<Locale> supportedLocales = <Locale>[
    Locale('en'),
    Locale('zh'),
  ];

  /// This language's own name, shown in a language picker. Never translated: each locale spells its own name.
  ///
  /// In zh, this message translates to:
  /// **'繁體中文'**
  String get languageName;

  /// Relative time under one minute.
  ///
  /// In zh, this message translates to:
  /// **'剛剛'**
  String get timeAgoJustNow;

  /// Relative time in whole minutes (1 to 59).
  ///
  /// In zh, this message translates to:
  /// **'{count, plural, other{{count}分鐘前}}'**
  String timeAgoMinutes(int count);

  /// Relative time in whole hours (1 to 23).
  ///
  /// In zh, this message translates to:
  /// **'{count, plural, other{{count}小時前}}'**
  String timeAgoHours(int count);

  /// Relative time in Taipei calendar days (1 to 6).
  ///
  /// In zh, this message translates to:
  /// **'{count, plural, other{{count}天前}}'**
  String timeAgoDays(int count);

  /// Taipei date in the current year, month and day.
  ///
  /// In zh, this message translates to:
  /// **'{month}/{day}'**
  String timeAgoMonthDay(int month, int day);

  /// Taipei date in an earlier year.
  ///
  /// In zh, this message translates to:
  /// **'{year}/{month}/{day}'**
  String timeAgoFullDate(int year, int month, int day);

  /// Marks a displayed time as Taipei time when the device time zone offset is not +8.
  ///
  /// In zh, this message translates to:
  /// **'台北時間'**
  String get taipeiTimeLabel;

  /// Caption above a table of times, shown when the device time zone offset is not +8.
  ///
  /// In zh, this message translates to:
  /// **'時間為台北時間'**
  String get taipeiTimeTableNote;

  /// Browsing history label under one minute.
  ///
  /// In zh, this message translates to:
  /// **'剛剛看過'**
  String get viewedJustNow;

  /// Browsing history label in whole minutes.
  ///
  /// In zh, this message translates to:
  /// **'{count, plural, other{{count} 分鐘前看過}}'**
  String viewedMinutes(int count);

  /// Browsing history label in whole hours.
  ///
  /// In zh, this message translates to:
  /// **'{count, plural, other{{count} 小時前看過}}'**
  String viewedHours(int count);

  /// Browsing history label for the previous Taipei calendar day.
  ///
  /// In zh, this message translates to:
  /// **'昨天看過'**
  String get viewedYesterday;

  /// Browsing history label in Taipei calendar days.
  ///
  /// In zh, this message translates to:
  /// **'{count, plural, other{{count} 天前看過}}'**
  String viewedDays(int count);

  /// Browsing history label with the Taipei date.
  ///
  /// In zh, this message translates to:
  /// **'{month}/{day} 看過'**
  String viewedDate(int month, int day);

  /// Browsing history label when the time is unknown.
  ///
  /// In zh, this message translates to:
  /// **'最近看過'**
  String get viewedRecently;

  /// Retry button label after a failed load.
  ///
  /// In zh, this message translates to:
  /// **'重試'**
  String get commonRetry;

  /// Title of the product reviews section and screen.
  ///
  /// In zh, this message translates to:
  /// **'商品評價'**
  String get productReviewsTitle;

  /// Product reviews title with the total review count.
  ///
  /// In zh, this message translates to:
  /// **'商品評價 ({count})'**
  String productReviewsTitleWithCount(int count);

  /// Conversation title when the store profile is missing.
  ///
  /// In zh, this message translates to:
  /// **'店家'**
  String get chatStoreFallbackName;

  /// Loading overlay text while a payment is being created or confirmed.
  ///
  /// In zh, this message translates to:
  /// **'處理中...'**
  String get paymentProcessing;

  /// Dialog title after the payment is confirmed as succeeded.
  ///
  /// In zh, this message translates to:
  /// **'付款成功'**
  String get paymentSucceededTitle;

  /// Dialog title when a payment is canceled, failed, pending or needs support.
  ///
  /// In zh, this message translates to:
  /// **'付款未完成'**
  String get paymentNotCompletedTitle;

  /// Message after the payment is confirmed as succeeded.
  ///
  /// In zh, this message translates to:
  /// **'付款已完成，感謝你的購買。'**
  String get paymentSucceededMessage;

  /// Message when the payment result is not final after the capped re-reads.
  ///
  /// In zh, this message translates to:
  /// **'付款處理中，請稍後到「我的訂單」確認結果。'**
  String get paymentPendingMessage;

  /// Message when the buyer closes the payment sheet.
  ///
  /// In zh, this message translates to:
  /// **'你已取消付款，訂單仍待付款，可到「我的訂單」重新付款。'**
  String get paymentCanceledMessage;

  /// Message when the payment attempt failed or was canceled by the provider.
  ///
  /// In zh, this message translates to:
  /// **'付款沒有完成，訂單仍待付款，可到「我的訂單」重新付款。'**
  String get paymentFailedMessage;

  /// Message when the payment is in ANOMALY; the buyer cannot retry.
  ///
  /// In zh, this message translates to:
  /// **'這筆付款需要客服協助處理，暫時無法重新付款，請聯絡客服。'**
  String get paymentAnomalyMessage;
}

class _AppLocalizationsDelegate
    extends LocalizationsDelegate<AppLocalizations> {
  const _AppLocalizationsDelegate();

  @override
  Future<AppLocalizations> load(Locale locale) {
    return SynchronousFuture<AppLocalizations>(lookupAppLocalizations(locale));
  }

  @override
  bool isSupported(Locale locale) =>
      <String>['en', 'zh'].contains(locale.languageCode);

  @override
  bool shouldReload(_AppLocalizationsDelegate old) => false;
}

AppLocalizations lookupAppLocalizations(Locale locale) {
  // Lookup logic when only language code is specified.
  switch (locale.languageCode) {
    case 'en':
      return AppLocalizationsEn();
    case 'zh':
      return AppLocalizationsZh();
  }

  throw FlutterError(
    'AppLocalizations.delegate failed to load unsupported locale "$locale". This is likely '
    'an issue with the localizations generation tool. Please file an issue '
    'on GitHub with a reproducible sample app and the gen-l10n configuration '
    'that was used.',
  );
}
