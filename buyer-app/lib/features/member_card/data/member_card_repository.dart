import 'package:hawkr_api/images.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pb.dart' as buyer;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/connect_provider.dart';
import 'member_card_model.dart';

part 'member_card_repository.g.dart';

@riverpod
MemberCardRepository memberCardRepository(Ref ref) => MemberCardRepository(
  BuyerAppServiceClient(ref.watch(connectAuthTransportProvider)),
);

class MemberCardRepository {
  final BuyerAppServiceClient _buyer;
  MemberCardRepository(this._buyer);

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
              sellerName: card.store.name,
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
