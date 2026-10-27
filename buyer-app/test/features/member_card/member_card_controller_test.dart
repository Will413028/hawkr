import 'dart:async';

import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pb.dart' as buyer;
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/store_card.pb.dart'
    as common;
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_buyer/core/session/session_model.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hawkr_buyer/features/member_card/data/member_card_repository.dart';
import 'package:hawkr_buyer/features/member_card/presentation/member_card_controller.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

class _FakeSession extends Session {
  @override
  Future<SessionModel?> build() async => const SessionModel(userId: '7');
}

buyer.MemberCard _card(String id) => buyer.MemberCard(
  storeId: id,
  store: common.StoreCard(id: id, name: id),
);

void main() {
  test(
    'member cards: a loadMore in flight during refresh is discarded',
    () async {
      final firstPageCalls = <int>[];
      final requestedTokens = <String>[];
      final staleNextPage = Completer<buyer.ListMemberCardsResponse>();
      final transport = FakeTransportBuilder().unary(
        BuyerAppService.listMemberCards,
        (request, _) {
          requestedTokens.add(request.pageToken);
          switch (request.pageToken) {
            case '':
              firstPageCalls.add(firstPageCalls.length);
              return firstPageCalls.length == 1
                  ? buyer.ListMemberCardsResponse(
                      memberCards: [_card('old')],
                      nextPageToken: 'old-next',
                    )
                  : buyer.ListMemberCardsResponse(
                      memberCards: [_card('fresh')],
                      nextPageToken: 'fresh-next',
                    );
            case 'old-next':
              return staleNextPage.future;
            default:
              return buyer.ListMemberCardsResponse(memberCards: [_card('p2')]);
          }
        },
      ).build();
      final container = ProviderContainer(
        overrides: [
          sessionProvider.overrideWith(_FakeSession.new),
          memberCardRepositoryProvider.overrideWithValue(
            MemberCardRepository(BuyerAppServiceClient(transport)),
          ),
        ],
      );
      addTearDown(container.dispose);
      container.listen(memberCardsControllerProvider, (_, _) {});
      await container.read(sessionProvider.future);
      await container.read(memberCardsControllerProvider.future);

      final notifier = container.read(memberCardsControllerProvider.notifier);
      final pending = notifier.loadMore();
      await notifier.refresh();
      staleNextPage.complete(
        buyer.ListMemberCardsResponse(
          memberCards: [_card('stale')],
          nextPageToken: 'stale-next',
        ),
      );
      await pending;

      var state = container.read(memberCardsControllerProvider).requireValue;
      expect(state.records.map((c) => c.sellerId), ['fresh']);
      expect(state.hasMore, isTrue);

      await container.read(memberCardsControllerProvider.notifier).loadMore();
      state = container.read(memberCardsControllerProvider).requireValue;
      expect(requestedTokens.last, 'fresh-next');
      expect(state.records.map((c) => c.sellerId), ['fresh', 'p2']);
    },
  );

  test(
    'spending records: a loadMore in flight during refresh is discarded',
    () async {
      var firstPageCalls = 0;
      final requestedTokens = <String>[];
      final staleNextPage = Completer<buyer.ListSpendingRecordsResponse>();
      final transport = FakeTransportBuilder().unary(
        BuyerAppService.listSpendingRecords,
        (request, _) {
          requestedTokens.add(request.pageToken);
          switch (request.pageToken) {
            case '':
              firstPageCalls++;
              return buyer.ListSpendingRecordsResponse(
                records: [
                  buyer.SpendingRecord(
                    orderNumber: firstPageCalls == 1 ? 'old' : 'fresh',
                  ),
                ],
                nextPageToken: firstPageCalls == 1 ? 'old-next' : 'fresh-next',
              );
            case 'old-next':
              return staleNextPage.future;
            default:
              return buyer.ListSpendingRecordsResponse(
                records: [buyer.SpendingRecord(orderNumber: 'p2')],
              );
          }
        },
      ).build();
      final container = ProviderContainer(
        overrides: [
          memberCardRepositoryProvider.overrideWithValue(
            MemberCardRepository(BuyerAppServiceClient(transport)),
          ),
        ],
      );
      addTearDown(container.dispose);
      final provider = spendingRecordsControllerProvider('9');
      container.listen(provider, (_, _) {});
      await container.read(provider.future);

      final pending = container.read(provider.notifier).loadMore();
      container.invalidate(provider);
      await container.read(provider.future);
      staleNextPage.complete(
        buyer.ListSpendingRecordsResponse(
          records: [buyer.SpendingRecord(orderNumber: 'stale')],
          nextPageToken: 'stale-next',
        ),
      );
      await pending;

      var state = container.read(provider).requireValue;
      expect(state.records.map((r) => r.orderSn), ['fresh']);
      expect(state.hasMore, isTrue);

      await container.read(provider.notifier).loadMore();
      state = container.read(provider).requireValue;
      expect(requestedTokens.last, 'fresh-next');
      expect(state.records.map((r) => r.orderSn), ['fresh', 'p2']);
    },
  );
}
