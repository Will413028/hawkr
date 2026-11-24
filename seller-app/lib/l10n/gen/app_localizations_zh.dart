// ignore: unused_import
import 'package:intl/intl.dart' as intl;
import 'app_localizations.dart';

// ignore_for_file: type=lint

/// The translations for Chinese (`zh`).
class AppLocalizationsZh extends AppLocalizations {
  AppLocalizationsZh([String locale = 'zh']) : super(locale);

  @override
  String get languageName => '繁體中文';

  @override
  String get timeAgoJustNow => '剛剛';

  @override
  String timeAgoMinutes(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count分鐘前',
    );
    return '$_temp0';
  }

  @override
  String timeAgoHours(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count小時前',
    );
    return '$_temp0';
  }

  @override
  String timeAgoDays(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count天前',
    );
    return '$_temp0';
  }

  @override
  String timeAgoMonthDay(int month, int day) {
    return '$month/$day';
  }

  @override
  String timeAgoFullDate(int year, int month, int day) {
    return '$year/$month/$day';
  }

  @override
  String get taipeiTimeLabel => '台北時間';

  @override
  String get commonRetry => '重試';

  @override
  String get commonReload => '重新載入';

  @override
  String get commonLoadMore => '載入更多';

  @override
  String get chatLoadMoreFailedRetry => '載入更多失敗，點此重試';

  @override
  String get chatBuyerFallbackName => '顧客';

  @override
  String get chatSendFailedResend => '送出失敗，點一下重送';

  @override
  String get chatSending => '傳送中…';

  @override
  String get chatReadPrefix => '已讀 ';

  @override
  String get chatLoadFailedTitle => '目前無法載入訊息';

  @override
  String get chatLoadFailedDescription => '請確認網路連線後再試一次。';

  @override
  String get customerOrdersTitle => '顧客訂單';

  @override
  String get customerOrdersEmpty => '暫無訂單';

  @override
  String get customerBuyerFallbackName => '買家';

  @override
  String get couponPickFromCustomers => '從客戶名單選擇';

  @override
  String get couponPickFromCustomersHint => '從客戶名單挑選這次發放的會員';

  @override
  String get reportMessageTitle => '檢舉訊息';

  @override
  String get customerPickerTitle => '選擇客戶';

  @override
  String get customerPickerEmpty => '沒有可選的客戶';

  @override
  String customerPickerAdd(int count) {
    return '加入名單（$count）';
  }

  @override
  String get quoteResponseTypeHelp => '兩種回應都要填完整的計價基準與金額——「接受非價格條款」不是少填金額的捷徑。';

  @override
  String get quoteSpecNoteTitle => '規格說明（選填）';

  @override
  String get quoteSpecNoteHelp => '這一欄只能由你自己填寫，最多 1000 字。';

  @override
  String get quoteSpecNoteLabel => '規格說明';

  @override
  String get quoteSpecNoteHint => '例如：產地、等級、包裝方式、驗收條件';

  @override
  String get quoteCurrencyTwd => '幣別 TWD';

  @override
  String get quoteStatusActive => '有效';

  @override
  String get quoteStatusSuperseded => '已被改版取代';

  @override
  String get quoteStatusWithdrawn => '已撤回';

  @override
  String get quoteStatusExpired => '已逾有效期';

  @override
  String quoteAmountNotWholeNumber(String label) {
    return '$label只能是非負整數（新台幣元），例如 1200';
  }

  @override
  String get earningsTitle => '入帳紀錄';

  @override
  String earningsNetAmount(String amount) {
    return '淨入帳 $amount';
  }

  @override
  String get earningsEmpty => '還沒有入帳紀錄';

  @override
  String get earningsLoadFailed => '無法載入入帳紀錄';
}
