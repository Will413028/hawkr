import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/gen/google/rpc/error_details.pb.dart'
    show LocalizedMessage;
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart';
import 'package:hawkr_seller/features/seller/data/seller_onboarding.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../../support/connect_errors.dart';
import '../../../support/fake_my_store.dart';
import '../../../support/default_error_text.dart';

final _acceptance = AgreementAcceptance(
  kind: AgreementKind.AGREEMENT_KIND_SELLER_TERMS,
  version: '2026-10-01',
);

void main() {
  late List<ApplyStoreRequest> applies;
  late List<UpdateMyStoreRequest> updates;
  late List<CompleteMyStoreOnboardingRequest> completes;
  late int reloads;
  Object? applyFailure;
  Object? completeFailure;

  SellerOnboarding build(FakeMyStoreServer server) {
    applies = [];
    updates = [];
    completes = [];
    reloads = 0;
    return SellerOnboarding(
      client: SellerAppServiceClient(
        FakeTransportBuilder()
            .unary(SellerAppService.applyStore, (req, _) {
              applies.add(req);
              final error = applyFailure;
              if (error != null) throw error;
              return ApplyStoreResponse(store: Store(etag: 'applied'));
            })
            .unary(SellerAppService.updateMyStore, (req, _) {
              updates.add(req);
              return UpdateMyStoreResponse(store: Store(etag: 'updated'));
            })
            .unary(SellerAppService.completeMyStoreOnboarding, (req, _) {
              completes.add(req);
              final error = completeFailure;
              if (error != null) throw error;
              return CompleteMyStoreOnboardingResponse();
            })
            .build(),
      ),
      stores: server.repository,
      reload: () async {
        reloads++;
        return true;
      },
      defaults: zhDefaultErrorText,
    );
  }

  setUp(() {
    applyFailure = null;
    completeFailure = null;
  });

  test('沒有店家：ApplyStore 帶賣家契約與取得的版本，再用回應的 etag 完成引導', () async {
    final outcome = await build(FakeMyStoreServer.absent()).submit(
      storeName: '阿明果園',
      townCode: '63000010',
      addressDetail: '路一號',
      acceptance: _acceptance,
    );

    expect(outcome, isA<OnboardingCompleted>());
    expect((outcome as OnboardingCompleted).created, isTrue);
    final apply = applies.single;
    expect(apply.storeName, '阿明果園');
    expect(apply.address.townCode, '63000010');
    expect(apply.address.addressDetail, '路一號');
    expect(
      apply.acceptedAgreements.single.kind,
      AgreementKind.AGREEMENT_KIND_SELLER_TERMS,
    );
    expect(apply.acceptedAgreements.single.version, '2026-10-01');
    expect(updates, isEmpty);
    expect(completes.single.etag, 'applied');
    expect(reloads, 1);
  });

  test('沒有店家又沒有同意：不送出', () async {
    final outcome = await build(
      FakeMyStoreServer.absent(),
    ).submit(storeName: '阿明果園', townCode: '63000010', addressDetail: '路一號');

    expect(outcome, isA<OnboardingTermsRequired>());
    expect(applies, isEmpty);
  });

  test('已有店家：UpdateMyStore 只送填的欄位（店名、地址），再用更新後的 etag 完成引導', () async {
    final server = FakeMyStoreServer();
    final outcome = await build(
      server,
    ).submit(storeName: '新店名', townCode: '63000010', addressDetail: '路二號');

    expect(outcome, isA<OnboardingCompleted>());
    expect((outcome as OnboardingCompleted).created, isFalse);
    expect(applies, isEmpty);
    expect(updates.single.updateMask.paths, ['store_name', 'address']);
    expect(updates.single.store.etag, 'etag-1');
    expect(updates.single.store.storeName, '新店名');
    expect(completes.single.etag, 'updated');
  });

  test('AGREEMENT_OUTDATED：回報條款過期，不完成引導', () async {
    applyFailure = reasonError(Code.failedPrecondition, 'AGREEMENT_OUTDATED');

    final outcome = await build(FakeMyStoreServer.absent()).submit(
      storeName: 'a',
      townCode: '63000010',
      addressDetail: 'b',
      acceptance: _acceptance,
    );

    expect(outcome, isA<OnboardingAgreementOutdated>());
    expect(completes, isEmpty);
    expect(agreementOutdatedMessage, '條款已更新，請重新閱讀後再勾選。');
  });

  group('storeOpeningBlock（GetMyStoreOpening 的答案）', () {
    StoreOpeningUnavailableReason r(String suffix) =>
        StoreOpeningUnavailableReason.values.firstWhere(
          (v) => v.name.endsWith(suffix),
        );

    StoreOpeningUnavailability u(String suffix, [String? message]) =>
        StoreOpeningUnavailability(
          reason: r(suffix),
          message: message == null
              ? null
              : LocalizedMessage(locale: 'zh-TW', message: message),
        );

    GetMyStoreOpeningResponse answer(List<StoreOpeningUnavailability> us) =>
        GetMyStoreOpeningResponse(
          canApplyStore: us.isEmpty,
          unavailabilities: us,
        );

    test('可以開店：不擋', () {
      expect(storeOpeningBlock(answer(const [])), isNull);
    });

    test('取伺服器排的第一筆：原因決定申覆入口，句子是伺服器的；終止先於停權', () {
      final first = storeOpeningBlock(
        answer([u('_TERMINATED', '終止的句子'), u('_SUSPENDED', '停權的句子')]),
      )!;
      expect(first.refusal, StoreOpeningRefusal.terminated);
      expect(first.message, '終止的句子');
      final second = storeOpeningBlock(
        answer([u('_SUSPENDED', '停權的句子'), u('_TERMINATED', '終止的句子')]),
      )!;
      expect(second.refusal, StoreOpeningRefusal.suspended);
      expect(second.message, '停權的句子');
      final deleting = storeOpeningBlock(
        answer([u('_ACCOUNT_DELETING', '刪除的句子')]),
      )!;
      expect(deleting.refusal, StoreOpeningRefusal.accountDeleting);
      expect(deleting.message, '刪除的句子');
    });

    test('是否帶申覆入口只看原因', () {
      expect(StoreOpeningRefusal.accountDeleting.offersAppeal, isFalse);
      expect(StoreOpeningRefusal.storeExists.offersAppeal, isFalse);
      expect(StoreOpeningRefusal.terminated.offersAppeal, isTrue);
      expect(StoreOpeningRefusal.suspended.offersAppeal, isTrue);
    });

    test('STORE_EXISTS 和其他原因一樣擋', () {
      expect(
        storeOpeningBlock(answer([u('_STORE_EXISTS', '已有店家')]))?.refusal,
        StoreOpeningRefusal.storeExists,
      );
      expect(
        storeOpeningBlock(
          answer([u('_SUSPENDED', '停權的句子'), u('_STORE_EXISTS', '已有店家')]),
        )?.refusal,
        StoreOpeningRefusal.suspended,
      );
    });

    test('已知原因但伺服器沒有句子（沒有訊息或空字串）：仍擋，沒有可顯示的文字，client 不自己組字', () {
      for (final m in <String?>[null, '']) {
        final block = storeOpeningBlock(
          answer([
            StoreOpeningUnavailability(
              reason: r('_SUSPENDED'),
              message: m == null ? null : LocalizedMessage(message: m),
            ),
          ]),
        )!;
        expect(block.refusal, StoreOpeningRefusal.suspended, reason: '$m');
        expect(block.message, isNull, reason: '$m');
      }
    });

    test('可以不認得的原因但有句子：沒有申覆入口，仍顯示伺服器的句子', () {
      final block = storeOpeningBlock(answer([u('_UNSPECIFIED', '新原因的句子')]))!;
      expect(block.refusal, isNull);
      expect(block.message, '新原因的句子');
    });

    test('can_apply_store 為 false 而原因是空（未知值解碼後即為空）：仍擋，但沒有可顯示的原因', () {
      // 這個版本不認得的值在 protobuf-dart 解碼時就被丟棄，所以線上形狀等同空列表。
      final block = storeOpeningBlock(
        GetMyStoreOpeningResponse(canApplyStore: false),
      );
      expect(block, isNotNull);
      expect(block!.refusal, isNull);
      expect(block.message, isNull);
    });
  });

  for (final c in [
    (
      'STORE_OPENING_TERMINATED',
      StoreOpeningRefusal.terminated,
      '此帳號因終止合作（第 21.3 條）不能重新開店。如有異議，可對該終止提出申覆。',
    ),
    (
      'STORE_OPENING_SUSPENDED',
      StoreOpeningRefusal.suspended,
      '帳號停權中，解除停權前不能開店',
    ),
  ]) {
    test('${c.$1}：回報開店被拒與錯誤的 LocalizedMessage，不完成引導、不重讀', () async {
      applyFailure = reasonError(Code.failedPrecondition, c.$1, message: c.$3);

      final outcome = await build(FakeMyStoreServer.absent()).submit(
        storeName: 'a',
        townCode: '63000010',
        addressDetail: 'b',
        acceptance: _acceptance,
      );

      expect(outcome, isA<OnboardingStoreOpeningRefused>());
      final refused = outcome as OnboardingStoreOpeningRefused;
      expect(refused.refusal, c.$2);
      expect(refused.message, c.$3);
      expect(completes, isEmpty);
      expect(reloads, 0);
    });
  }

  test('STORE_OPENING_SUSPENDED 但錯誤沒有 LocalizedMessage：仍是開店被拒，沒有文字', () async {
    applyFailure = reasonError(
      Code.failedPrecondition,
      'STORE_OPENING_SUSPENDED',
    );

    final outcome = await build(FakeMyStoreServer.absent()).submit(
      storeName: 'a',
      townCode: '63000010',
      addressDetail: 'b',
      acceptance: _acceptance,
    );

    final refused = outcome as OnboardingStoreOpeningRefused;
    expect(refused.refusal, StoreOpeningRefusal.suspended);
    expect(refused.message, isNull);
  });

  test('failed_precondition 但沒有 STORE_OPENING reason：不是開店被拒', () async {
    applyFailure = reasonError(Code.failedPrecondition, 'SELLER_SUSPENDED');

    final outcome = await build(FakeMyStoreServer.absent()).submit(
      storeName: 'a',
      townCode: '63000010',
      addressDetail: 'b',
      acceptance: _acceptance,
    );

    expect(outcome, isA<OnboardingFailed>());
  });

  test('ALREADY_EXISTS：重讀店家並回報衝突，輸入保留', () async {
    applyFailure = ConnectException(Code.alreadyExists, 'exists');

    final outcome = await build(FakeMyStoreServer.absent()).submit(
      storeName: 'a',
      townCode: '63000010',
      addressDetail: 'b',
      acceptance: _acceptance,
    );

    expect(outcome, isA<OnboardingConflict>());
    expect(
      (outcome as OnboardingConflict).message,
      '店家資料已在其他地方建立，已載入最新版本，你輸入的內容仍保留，請確認後再送出。',
    );
    expect(reloads, 1);
  });

  test('完成引導時 VERSION_CONFLICT：重讀並回報衝突', () async {
    completeFailure = reasonError(Code.aborted, 'VERSION_CONFLICT');

    final outcome = await build(
      FakeMyStoreServer(),
    ).submit(storeName: 'a', townCode: '63000010', addressDetail: 'b');

    expect(outcome, isA<OnboardingConflict>());
    expect(reloads, 1);
  });

  test('讀不到店家狀態（暫時失敗）：不能當成沒有店家去建立', () async {
    final outcome = await build(FakeMyStoreServer.unavailable()).submit(
      storeName: 'a',
      townCode: '63000010',
      addressDetail: 'b',
      acceptance: _acceptance,
    );

    expect(outcome, isA<OnboardingFailed>());
    expect((outcome as OnboardingFailed).message, '目前無法確認商店狀態，請稍後再試一次。');
    expect(applies, isEmpty);
    expect(updates, isEmpty);
  });
}
