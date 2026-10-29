import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/store_card.pb.dart';
import 'package:hawkr_buyer/features/seller/data/seller_repository.dart';
import 'package:hawkr_buyer/features/seller/data/store_card_provider.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../support/fake_seller_repository.dart';

void main() {
  test(
    'stores asked for in the same synchronous pass share one batch',
    () async {
      final batches = <List<String>>[];
      final container = ProviderContainer(
        overrides: [
          sellerRepositoryProvider.overrideWithValue(
            fakeSellerRepository(
              batches: batches,
              cards: [
                StoreCard(id: '1', name: 'A'),
                StoreCard(id: '2', name: 'B'),
              ],
            ),
          ),
        ],
      );
      addTearDown(container.dispose);

      final results = await Future.wait([
        container.read(storeCardProvider('1').future),
        container.read(storeCardProvider('2').future),
        container.read(storeCardProvider('3').future),
        container.read(storeCardProvider('1').future),
      ]);

      expect(batches, hasLength(1));
      expect(batches.single, unorderedEquals(['1', '2', '3']));
      expect(results[0]?.name, 'A');
      expect(results[1]?.name, 'B');
      expect(results[2], isNull);
      expect(results[3]?.name, 'A');
    },
  );

  test('a blank id never reaches the server', () async {
    final batches = <List<String>>[];
    final container = ProviderContainer(
      overrides: [
        sellerRepositoryProvider.overrideWithValue(
          fakeSellerRepository(batches: batches),
        ),
      ],
    );
    addTearDown(container.dispose);

    expect(await container.read(storeCardProvider('  ').future), isNull);
    expect(batches, isEmpty);
  });
}
