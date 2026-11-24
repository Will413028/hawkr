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

  /// Retry button label after a failed load.
  ///
  /// In zh, this message translates to:
  /// **'重試'**
  String get commonRetry;

  /// Reload button label after a failed load.
  ///
  /// In zh, this message translates to:
  /// **'重新載入'**
  String get commonReload;

  /// Button that loads the next page of a list.
  ///
  /// In zh, this message translates to:
  /// **'載入更多'**
  String get commonLoadMore;

  /// Chat list footer button after loading the next page failed.
  ///
  /// In zh, this message translates to:
  /// **'載入更多失敗，點此重試'**
  String get chatLoadMoreFailedRetry;

  /// Chat title when the buyer has no name.
  ///
  /// In zh, this message translates to:
  /// **'顧客'**
  String get chatBuyerFallbackName;

  /// Hint under a chat message that failed to send.
  ///
  /// In zh, this message translates to:
  /// **'送出失敗，點一下重送'**
  String get chatSendFailedResend;

  /// Hint under a chat message that is being sent.
  ///
  /// In zh, this message translates to:
  /// **'傳送中…'**
  String get chatSending;

  /// Prefix before the time of a message the buyer has read, followed by the time.
  ///
  /// In zh, this message translates to:
  /// **'已讀 '**
  String get chatReadPrefix;

  /// Title of the chat thread load error panel.
  ///
  /// In zh, this message translates to:
  /// **'目前無法載入訊息'**
  String get chatLoadFailedTitle;

  /// Description of the chat thread load error panel.
  ///
  /// In zh, this message translates to:
  /// **'請確認網路連線後再試一次。'**
  String get chatLoadFailedDescription;

  /// Subtitle of the customer orders screen.
  ///
  /// In zh, this message translates to:
  /// **'顧客訂單'**
  String get customerOrdersTitle;

  /// Empty state of the customer orders screen.
  ///
  /// In zh, this message translates to:
  /// **'暫無訂單'**
  String get customerOrdersEmpty;

  /// Customer display name when the buyer has no name.
  ///
  /// In zh, this message translates to:
  /// **'買家'**
  String get customerBuyerFallbackName;

  /// Button that opens the customer picker when issuing a coupon.
  ///
  /// In zh, this message translates to:
  /// **'從客戶名單選擇'**
  String get couponPickFromCustomers;

  /// Hint when no customer is selected for a coupon issue.
  ///
  /// In zh, this message translates to:
  /// **'從客戶名單挑選這次發放的會員'**
  String get couponPickFromCustomersHint;

  /// Title of the report sheet for a chat message.
  ///
  /// In zh, this message translates to:
  /// **'檢舉訊息'**
  String get reportMessageTitle;

  /// Title of the customer picker sheet.
  ///
  /// In zh, this message translates to:
  /// **'選擇客戶'**
  String get customerPickerTitle;

  /// Empty state of the customer picker sheet.
  ///
  /// In zh, this message translates to:
  /// **'沒有可選的客戶'**
  String get customerPickerEmpty;

  /// Confirm button of the customer picker with the number selected.
  ///
  /// In zh, this message translates to:
  /// **'加入名單（{count}）'**
  String customerPickerAdd(int count);

  /// Helper text under the response type heading of the quote form.
  ///
  /// In zh, this message translates to:
  /// **'兩種回應都要填完整的計價基準與金額——「接受非價格條款」不是少填金額的捷徑。'**
  String get quoteResponseTypeHelp;

  /// Section title of the optional specification note on the quote form.
  ///
  /// In zh, this message translates to:
  /// **'規格說明（選填）'**
  String get quoteSpecNoteTitle;

  /// Helper text under the specification note title.
  ///
  /// In zh, this message translates to:
  /// **'這一欄只能由你自己填寫，最多 1000 字。'**
  String get quoteSpecNoteHelp;

  /// Label of the specification note field.
  ///
  /// In zh, this message translates to:
  /// **'規格說明'**
  String get quoteSpecNoteLabel;

  /// Hint inside the specification note field.
  ///
  /// In zh, this message translates to:
  /// **'例如：產地、等級、包裝方式、驗收條件'**
  String get quoteSpecNoteHint;

  /// Currency caption of the amounts card on the quote form.
  ///
  /// In zh, this message translates to:
  /// **'幣別 TWD'**
  String get quoteCurrencyTwd;

  /// Status of the latest seller quote version: in force.
  ///
  /// In zh, this message translates to:
  /// **'有效'**
  String get quoteStatusActive;

  /// Status of a seller quote version replaced by a newer version.
  ///
  /// In zh, this message translates to:
  /// **'已被改版取代'**
  String get quoteStatusSuperseded;

  /// Status of a seller quote version the seller withdrew.
  ///
  /// In zh, this message translates to:
  /// **'已撤回'**
  String get quoteStatusWithdrawn;

  /// Status of a seller quote version past its validity.
  ///
  /// In zh, this message translates to:
  /// **'已逾有效期'**
  String get quoteStatusExpired;

  /// Validation error for an amount that is not a non-negative whole number.
  ///
  /// In zh, this message translates to:
  /// **'{label}只能是非負整數（新台幣元），例如 1200'**
  String quoteAmountNotWholeNumber(String label);

  /// Title of the seller's earnings ledger (transfers in, reversals, top-ups), on the wallet and on an order's detail.
  ///
  /// In zh, this message translates to:
  /// **'入帳紀錄'**
  String get earningsTitle;

  /// Net earnings of one order: the sum of completed ledger rows, already formatted as money.
  ///
  /// In zh, this message translates to:
  /// **'淨入帳 {amount}'**
  String earningsNetAmount(String amount);

  /// Empty state of the wallet earnings ledger.
  ///
  /// In zh, this message translates to:
  /// **'還沒有入帳紀錄'**
  String get earningsEmpty;

  /// Error shown when the wallet earnings ledger fails to load.
  ///
  /// In zh, this message translates to:
  /// **'無法載入入帳紀錄'**
  String get earningsLoadFailed;
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
