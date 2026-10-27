import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../data/seller_quote_model.dart';
import '../data/seller_quote_repository.dart';

part 'seller_publication_list_controller.g.dart';

Duration? _noAutoRetry(int retryCount, Object error) => null;

@Riverpod(retry: _noAutoRetry)
Future<List<SellerPublicationListItem>> sellerPublicationList(Ref ref) {
  return ref.watch(sellerQuoteRepositoryProvider).listPublications();
}

enum SellerPublicationDeadlineState { unknown, overdue, dueToday, upcoming }

class SellerPublicationDeadline {
  const SellerPublicationDeadline._(this.state, this.date, this.daysLeft);

  static const SellerPublicationDeadline unknown = SellerPublicationDeadline._(
    SellerPublicationDeadlineState.unknown,
    null,
    0,
  );

  final SellerPublicationDeadlineState state;

  final DateTime? date;

  final int daysLeft;

  bool get isKnown => state != SellerPublicationDeadlineState.unknown;
}

final RegExp _isoDatePrefix = RegExp(r'^(\d{4})-(\d{2})-(\d{2})');

SellerPublicationDeadline classifyResponseDeadline(
  String raw, {
  required DateTime now,
}) {
  final trimmed = raw.trim();
  if (trimmed.isEmpty) return SellerPublicationDeadline.unknown;

  final match = _isoDatePrefix.firstMatch(trimmed);
  if (match == null) return SellerPublicationDeadline.unknown;
  final year = int.parse(match.group(1)!);
  final month = int.parse(match.group(2)!);
  final day = int.parse(match.group(3)!);

  final date = DateTime(year, month, day);
  if (date.year != year || date.month != month || date.day != day) {
    return SellerPublicationDeadline.unknown;
  }

  final due = DateTime.utc(year, month, day);
  final today = DateTime.utc(now.year, now.month, now.day);
  final days = due.difference(today).inDays;

  if (days < 0) {
    return SellerPublicationDeadline._(
      SellerPublicationDeadlineState.overdue,
      date,
      days,
    );
  }
  if (days == 0) {
    return SellerPublicationDeadline._(
      SellerPublicationDeadlineState.dueToday,
      date,
      0,
    );
  }
  return SellerPublicationDeadline._(
    SellerPublicationDeadlineState.upcoming,
    date,
    days,
  );
}
