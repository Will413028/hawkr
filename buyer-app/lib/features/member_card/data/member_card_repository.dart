import 'package:hawkr_buyer/core/l10n/ui_locale.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:hawkr_api/images.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pb.dart' as buyer;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/connect_provider.dart';
import '../../seller/data/store_card_display.dart';
import 'member_card_model.dart';

part 'member_card_repository.g.dart';

@riverpod
MemberCardRepository memberCardRepository(Ref ref) => MemberCardRepository(
  BuyerAppServiceClient(ref.watch(connectAuthTransportProvider)),
  locale: ref.watch(uiLocaleProvider),
);

class MemberCardRepository {
  final BuyerAppServiceClient _buyer;
  final UiLocale _locale;
  MemberCardRepository(this._buyer, {UiLocale locale = UiLocale.zhTW})
    : _locale = locale;

  Future<({List<MemberCard> records, String nextPageToken})> getMemberCards({
    String pageToken = '',
    int size = 50,
  }) async {
    final result = await _buyer.listMemberCards(
      buyer.ListMemberCardsRequest(pageSize: size, pageToken: pageToken),
    );
    return (
      records: result.memberCards
          .map(
            (card) => MemberCard(
              sellerId: card.storeId,
              sellerName: card.store.displayName(_locale),
              statusLabel: card.store.statusLabel(_locale) ?? '',
              storeCode: card.store.isOpen ? card.store.storeCode : '',
              sellerLogoKey: card.store.hasLogo()
                  ? card.store.logo.uri(ImagePreset.avatar)
                  : '',
              sellerCoverKey: card.store.hasCoverImage()
                  ? card.store.coverImage.uri(ImagePreset.card)
                  : '',
              totalSpending: card.totalSpent.toString(),
            ),
          )
          .where((card) => card.sellerId.isNotEmpty)
          .toList(),
      nextPageToken: result.nextPageToken,
    );
  }

  Future<
    ({
      int totalSpending,
      int totalRefund,
      List<SpendingRecord> records,
      String nextPageToken,
    })
  >
  getSpendingRecords({
    required String sellerId,
    String pageToken = '',
    int size = 50,
  }) async {
    final result = await _buyer.listSpendingRecords(
      buyer.ListSpendingRecordsRequest(
        storeId: sellerId,
        pageSize: size,
        pageToken: pageToken,
      ),
    );
    return (
      totalSpending: result.totalSpent.toInt(),
      totalRefund: result.totalRefunded.toInt(),
      records: result.records.map((record) {
        final date = record.hasOrderTime()
            ? record.orderTime.toDateTime()
            : null;
        return SpendingRecord(
          orderDate: date == null
              ? '-'
              : '${date.year}-${date.month.toString().padLeft(2, '0')}-${date.day.toString().padLeft(2, '0')}',
          orderSn: record.orderNumber,
          amount: record.amount.toString(),
          refund: record.refundedAmount.toString(),
          hasCoupon: record.couponApplied,
        );
      }).toList(),
      nextPageToken: result.nextPageToken,
    );
  }
}
