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
  String get taipeiTimeTableNote => 'Times are in Taipei time';

  @override
  String get viewedJustNow => 'Viewed just now';

  @override
  String viewedMinutes(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: 'Viewed $count minutes ago',
      one: 'Viewed 1 minute ago',
    );
    return '$_temp0';
  }

  @override
  String viewedHours(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: 'Viewed $count hours ago',
      one: 'Viewed 1 hour ago',
    );
    return '$_temp0';
  }

  @override
  String get viewedYesterday => 'Viewed yesterday';

  @override
  String viewedDays(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: 'Viewed $count days ago',
      one: 'Viewed 1 day ago',
    );
    return '$_temp0';
  }

  @override
  String viewedDate(int month, int day) {
    return 'Viewed $month/$day';
  }

  @override
  String get viewedRecently => 'Viewed recently';

  @override
  String get commonRetry => 'Retry';

  @override
  String get productReviewsTitle => 'Reviews';

  @override
  String productReviewsTitleWithCount(int count) {
    return 'Reviews ($count)';
  }

  @override
  String get chatStoreFallbackName => 'Store';

  @override
  String get paymentProcessing => 'Processing...';

  @override
  String get paymentSucceededTitle => 'Payment successful';

  @override
  String get paymentNotCompletedTitle => 'Payment not completed';

  @override
  String get paymentSucceededMessage =>
      'Your payment is complete. Thank you for your purchase.';

  @override
  String get paymentPendingMessage =>
      'Your payment is still being processed. Check My Orders in a moment for the result.';

  @override
  String get paymentCanceledMessage =>
      'You canceled the payment. The order is still unpaid; you can pay again from My Orders.';

  @override
  String get paymentFailedMessage =>
      'The payment did not go through. The order is still unpaid; you can pay again from My Orders.';

  @override
  String get paymentAnomalyMessage =>
      'This payment needs help from support and cannot be retried right now. Please contact support.';

  @override
  String get afterSaleReturnPendingSummary => 'Please send the item back.';
}
