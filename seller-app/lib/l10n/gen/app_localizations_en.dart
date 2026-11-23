// ignore: unused_import
import 'package:intl/intl.dart' as intl;
import 'app_localizations.dart';

// ignore_for_file: type=lint

/// The translations for English (`en`).
class AppLocalizationsEn extends AppLocalizations {
  AppLocalizationsEn([String locale = 'en']) : super(locale);

  @override
  String get languageName => 'English';

  @override
  String get timeAgoJustNow => 'Just now';

  @override
  String timeAgoMinutes(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count minutes ago',
      one: '1 minute ago',
    );
    return '$_temp0';
  }

  @override
  String timeAgoHours(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count hours ago',
      one: '1 hour ago',
    );
    return '$_temp0';
  }

  @override
  String timeAgoDays(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count days ago',
      one: '1 day ago',
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
  String get taipeiTimeLabel => 'Taipei time';

  @override
  String get commonRetry => 'Retry';

  @override
  String get commonReload => 'Reload';

  @override
  String get commonLoadMore => 'Load more';

  @override
  String get chatLoadMoreFailedRetry => 'Could not load more. Tap to retry';

  @override
  String get chatBuyerFallbackName => 'Customer';

  @override
  String get chatSendFailedResend => 'Failed to send. Tap to resend';

  @override
  String get chatSending => 'Sending…';

  @override
  String get chatReadPrefix => 'Read ';

  @override
  String get chatLoadFailedTitle => 'Cannot load messages right now';

  @override
  String get chatLoadFailedDescription =>
      'Check your connection and try again.';

  @override
  String get customerOrdersTitle => 'Customer orders';

  @override
  String get customerOrdersEmpty => 'No orders yet';

  @override
  String get customerBuyerFallbackName => 'Buyer';

  @override
  String get couponPickFromCustomers => 'Choose from customers';

  @override
  String get couponPickFromCustomersHint =>
      'Pick the members for this issue from your customer list';

  @override
  String get reportMessageTitle => 'Report message';

  @override
  String get customerPickerTitle => 'Select customers';

  @override
  String get customerPickerEmpty => 'No customers to select';

  @override
  String customerPickerAdd(int count) {
    return 'Add to list ($count)';
  }
}
