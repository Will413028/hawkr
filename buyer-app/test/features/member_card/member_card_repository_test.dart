import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pb.dart' as buyer;
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/store_card.pb.dart'
    as common;
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_buyer/features/member_card/data/member_card_repository.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('member cards use the returned cursor for the next page', () async {
    final tokens = <String>[];
    final transport = FakeTransportBuilder().unary(
      BuyerAppService.listMemberCards,
      (request, _) {
        tokens.add(request.pageToken);
        if (request.pageToken.isEmpty) {
          return buyer.ListMemberCardsResponse(
            memberCards: [
              buyer.MemberCard(
                storeId: '1',
                store: common.StoreCard(id: '1', name: '甲'),
              ),
            ],
            nextPageToken: 'next',
          );
        }
        return buyer.ListMemberCardsResponse(
          memberCards: [
            buyer.MemberCard(
              storeId: '2',
              store: common.StoreCard(id: '2', name: '乙'),
            ),
          ],
        );
      },
    ).build();

    final repository = MemberCardRepository(BuyerAppServiceClient(transport));
    final first = await repository.getMemberCards(size: 1);
    final result = await repository.getMemberCards(
      pageToken: first.nextPageToken,
      size: 1,
    );
    expect(tokens, ['', 'next']);
    expect(result.records.single.sellerId, '2');
  });

  test('spending failures remain visible to the caller', () async {
    final transport = FakeTransportBuilder()
        .unary(
          BuyerAppService.listSpendingRecords,
          (_, _) =>
              throw ConnectException(Code.unauthenticated, 'session expired'),
        )
        .build();

    await expectLater(
      MemberCardRepository(
        BuyerAppServiceClient(transport),
      ).getSpendingRecords(sellerId: '9'),
      throwsA(isA<ConnectException>()),
    );
  });
}
