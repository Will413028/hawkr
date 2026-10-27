import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pbenum.dart';
import 'package:hawkr_seller/core/session/my_store.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../support/my_store_fixtures.dart';

StoreShare _read(MyStore? store) {
  final container = ProviderContainer(
    overrides: [myStoreSnapshotProvider.overrideWithValue(store)],
  );
  addTearDown(container.dispose);
  return container.read(storeShareProvider);
}

MyStore _store({required bool listed, String storeCode = 'FRESH88'}) => MyStore(
  myStoreResponse(
    storeCode: storeCode,
    publicListing: publicListingMissing([
      if (!listed) SellerRequirement.SELLER_REQUIREMENT_KYC_APPROVED,
    ]),
  ),
);

void main() {
  test('unknown while the store has not been read', () {
    expect(_read(null), isA<StoreShareUnknown>());
  });

  test('disabled when the store cannot go public', () {
    expect(_read(_store(listed: false)), isA<StoreShareDisabled>());
  });

  test('enabled with the store code when the store is public', () {
    final share = _read(_store(listed: true));
    expect(share, isA<StoreShareEnabled>());
    expect((share as StoreShareEnabled).storeCode, 'FRESH88');
  });

  test('public without a store code is unknown, not enabled', () {
    expect(
      _read(_store(listed: true, storeCode: '')),
      isA<StoreShareUnknown>(),
    );
  });
}
