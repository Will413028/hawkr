import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/transport.dart';
import 'package:hawkr_seller/core/session/my_store.dart';
import 'package:hawkr_seller/features/seller/data/my_store_repository.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../support/fake_my_store.dart';
import '../../support/my_store_fixtures.dart';

/// `GetMyStore` 的狀態推導，和 website 的 `lib/seller/store.ts` 同一套語意。
void main() {
  group('MyStoreRepository.fetch', () {
    test('not_found → 沒有店家（不是錯誤）', () async {
      final store = await FakeMyStoreServer.absent().repository.fetch();

      expect(store.hasStore, isFalse);
      expect(store.storeId, isNull);
      expect(store.needsOnboarding, isTrue);
      expect(store.isSuspended, isFalse);
      expect(store.capabilities, isEmpty);
    });

    test('暫時失敗不是「沒有店家」：拋 MyStoreUnavailable', () async {
      await expectLater(
        FakeMyStoreServer.unavailable().repository.fetch(),
        throwsA(
          isA<MyStoreUnavailable>().having(
            (e) => e.cause.code,
            'code',
            Code.unavailable,
          ),
        ),
      );
    });

    for (final code in [
      Code.internal,
      Code.deadlineExceeded,
      Code.permissionDenied,
      Code.unauthenticated,
    ]) {
      test('${code.name} 也不是「沒有店家」', () async {
        final server = FakeMyStoreServer(failure: ConnectException(code, 'x'));

        await expectLater(
          server.repository.fetch(),
          throwsA(isA<MyStoreUnavailable>()),
        );
      });
    }

    test('有店家 → 回快照', () async {
      final server = FakeMyStoreServer(
        response: myStoreResponse(id: 'store-9', name: '山上農場'),
      );

      final store = await server.repository.fetch();

      expect(store.hasStore, isTrue);
      expect(store.storeId, 'store-9');
      expect(store.storeName, '山上農場');
      expect(server.calls, 1);
    });
  });

  group('狀態推導', () {
    test('營業中、已完成新手引導', () {
      final store = myStore();

      expect(store.hasStore, isTrue);
      expect(store.isSuspended, isFalse);
      expect(store.isPendingDeletion, isFalse);
      expect(store.needsOnboarding, isFalse);
      expect(store.capabilities, activeCapabilities);
    });

    test('停權讀伺服器的 suspended 旗標', () {
      expect(suspendedStore().isSuspended, isTrue);
      expect(myStore().isSuspended, isFalse);
    });

    test('停權不從能力清單反推：旗標沒設，能力被收回也不算停權', () {
      expect(myStore(capabilities: suspendedCapabilities).isSuspended, isFalse);
    });

    test('等待刪除 ⇔ status 是 PENDING_DELETION；UNSPECIFIED 與 ACTIVE 都不是', () {
      expect(pendingDeletionStore().isPendingDeletion, isTrue);
      expect(myStore().isPendingDeletion, isFalse);
      expect(
        myStore(status: StoreStatus.STORE_STATUS_UNSPECIFIED).isPendingDeletion,
        isFalse,
      );
    });

    test('needsOnboarding ⇔ 沒有店家，或店家沒有完成新手引導', () {
      expect(myStore(onboardingCompleted: false).needsOnboarding, isTrue);
      expect(myStore().needsOnboarding, isFalse);
      expect(const MyStore.absent().needsOnboarding, isTrue);
    });

    test('回應沒有 store 欄位 → 視為沒有店家', () {
      final store = MyStore(GetMyStoreResponse());

      expect(store.hasStore, isFalse);
      expect(store.storeId, isNull);
      expect(store.needsOnboarding, isTrue);
    });

    test('store 有設但 id 是空字串 → storeId 是 null', () {
      expect(myStore().storeId, 'store-1');
      expect(MyStore(myStoreResponse(id: '')).storeId, isNull);
    });

    test('能力集合不含 UNSPECIFIED；UNSPECIFIED 不算持有任何能力', () {
      final store = myStore(
        capabilities: [
          SellerCapability.SELLER_CAPABILITY_UNSPECIFIED,
          SellerCapability.SELLER_CAPABILITY_ORDER_FULFILL,
        ],
      );

      expect(store.capabilities, [
        SellerCapability.SELLER_CAPABILITY_ORDER_FULFILL,
      ]);
      expect(
        store.can(SellerCapability.SELLER_CAPABILITY_UNSPECIFIED),
        isFalse,
      );
      expect(
        store.can(SellerCapability.SELLER_CAPABILITY_ORDER_FULFILL),
        isTrue,
      );
    });

    test('停權與等待刪除的能力來自伺服器：停權沒有 PRODUCT_MANAGE', () {
      expect(
        suspendedStore().can(SellerCapability.SELLER_CAPABILITY_PRODUCT_MANAGE),
        isFalse,
      );
      expect(
        pendingDeletionStore().can(
          SellerCapability.SELLER_CAPABILITY_ACCOUNT_CLOSE,
        ),
        isFalse,
      );
    });

    test('能力只有 active 的才算持有；沒有啟用的能力連同缺項照實回報', () {
      final store = MyStore(
        GetMyStoreResponse(
          capabilities: [
            CapabilityStatus(
              capability: SellerCapability.SELLER_CAPABILITY_ORDER_FULFILL,
              active: true,
            ),
            CapabilityStatus(
              capability: SellerCapability.SELLER_CAPABILITY_SHIPPING_SETUP,
              missing: [SellerRequirement.SELLER_REQUIREMENT_KYC_APPROVED],
            ),
          ],
        ),
      );

      expect(store.capabilities, [
        SellerCapability.SELLER_CAPABILITY_ORDER_FULFILL,
      ]);
      expect(
        store.can(SellerCapability.SELLER_CAPABILITY_SHIPPING_SETUP),
        isFalse,
      );
      expect(
        store
            .capabilityStatus(SellerCapability.SELLER_CAPABILITY_SHIPPING_SETUP)
            ?.missing,
        [SellerRequirement.SELLER_REQUIREMENT_KYC_APPROVED],
      );
      expect(
        store.capabilityStatus(SellerCapability.SELLER_CAPABILITY_MARKETING),
        isNull,
      );
    });

    test('PUBLIC_LISTING 沒回報時缺項是 null，不是「什麼都不缺」', () {
      expect(
        myStore().missingFor(SellerCapability.SELLER_CAPABILITY_PUBLIC_LISTING),
        isNull,
      );
      expect(myStore().isCertified, isFalse);
      final store = myStore(
        publicListing: publicListingMissing([
          SellerRequirement.SELLER_REQUIREMENT_KYC_APPROVED,
        ]),
      );

      expect(
        store.missingFor(SellerCapability.SELLER_CAPABILITY_PUBLIC_LISTING),
        [SellerRequirement.SELLER_REQUIREMENT_KYC_APPROVED],
      );
      expect(store.isCertified, isFalse);
      expect(
        myStore(publicListing: publicListingMissing()).isCertified,
        isTrue,
      );
    });

    test('shippingGate 只讀 SHIPPING_SETUP 的啟用與缺項，不看 PUBLIC_LISTING 或 KYC', () {
      const kyc = SellerRequirement.SELLER_REQUIREMENT_KYC_APPROVED;
      const suspended = SellerRequirement.SELLER_REQUIREMENT_NOT_SUSPENDED;
      const pending = SellerRequirement.SELLER_REQUIREMENT_NOT_PENDING_DELETION;
      expect(myStore().shippingGate, ShippingGate.open);
      expect(
        myStore(publicListing: publicListingMissing([kyc])).shippingGate,
        ShippingGate.open,
      );
      expect(
        myStore(shippingSetup: shippingSetupMissing([kyc])).shippingGate,
        ShippingGate.certification,
      );
      expect(
        myStore(
          shippingSetup: shippingSetupMissing([suspended, kyc]),
        ).shippingGate,
        ShippingGate.suspended,
      );
      expect(
        myStore(
          shippingSetup: shippingSetupMissing([suspended, pending]),
        ).shippingGate,
        ShippingGate.pendingDeletion,
      );
      expect(const MyStore.absent().shippingGate, ShippingGate.unknown);
      expect(
        myStore(
          shippingSetup: shippingSetupMissing([
            SellerRequirement.SELLER_REQUIREMENT_STORE_ACTIVE,
            kyc,
          ]),
        ).shippingGate,
        ShippingGate.unknown,
      );
      expect(
        myStore(
          shippingSetup: shippingSetupMissing([
            SellerRequirement.SELLER_REQUIREMENT_LOGO,
          ]),
        ).shippingGate,
        ShippingGate.unknown,
      );
      expect(
        MyStore(GetMyStoreResponse(store: Store(id: 'store-1'))).shippingGate,
        ShippingGate.unknown,
      );
    });

    test('missingFor：沒回報的能力是 null，回報了就照實給缺項', () {
      expect(
        const MyStore.absent().missingFor(
          SellerCapability.SELLER_CAPABILITY_SHIPPING_SETUP,
        ),
        isNull,
      );
      expect(
        myStore(
          shippingSetup: shippingSetupMissing([
            SellerRequirement.SELLER_REQUIREMENT_KYC_APPROVED,
          ]),
        ).missingFor(SellerCapability.SELLER_CAPABILITY_SHIPPING_SETUP),
        [SellerRequirement.SELLER_REQUIREMENT_KYC_APPROVED],
      );
    });

    test('地址顯示：縣市＋鄉鎮市區＋明細；沒填地址是 null', () {
      expect(myStore().addressText, isNull);
      expect(
        myStore(townCode: '63000050', addressDetail: '忠孝西路一段 1 號').addressText,
        '臺北市中正區忠孝西路一段 1 號',
      );
      expect(myStore(townCode: '63000050').addressText, '臺北市中正區');
    });

    test('商店電話與店名空白時是 null', () {
      expect(myStore(contactPhone: '').contactPhone, isNull);
      expect(myStore(contactPhone: '0912345678').contactPhone, '0912345678');
    });

    test('相同回應的快照相等', () {
      expect(myStore(), myStore());
      expect(myStore(), isNot(suspendedStore()));
    });
  });
}
