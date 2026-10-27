import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart';
import 'package:hawkr_seller/features/seller/data/store_editor.dart';
import 'package:hawkr_seller/features/seller/data/store_form.dart';
import 'package:hawkr_seller/features/seller/data/version_conflict.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../../support/connect_errors.dart';
import '../../../support/my_store_fixtures.dart';
import '../../../support/default_error_text.dart';

final _noticeAcceptances = [
  AgreementAcceptance(
    kind: AgreementKind.AGREEMENT_KIND_FOOD_REGISTRATION_NOTICE,
    version: 'n1',
  ),
];

void main() {
  late List<UpdateMyStoreRequest> requests;
  late int reloads;
  late bool reloadOk;
  late List<Store> adopted;
  Object? failure;

  StoreEditor editor() {
    requests = [];
    reloads = 0;
    adopted = [];
    reloadOk = true;
    return StoreEditor(
      client: SellerAppServiceClient(
        FakeTransportBuilder().unary(SellerAppService.updateMyStore, (req, _) {
          requests.add(req);
          final error = failure;
          if (error != null) throw error;
          return UpdateMyStoreResponse(store: Store(etag: 'v2'));
        }).build(),
      ),
      current: () => myStore(etag: 'v1'),
      reload: () async {
        reloads++;
        return reloadOk;
      },
      adopt: adopted.add,
      defaults: zhDefaultErrorText,
    );
  }

  setUp(() => failure = null);

  test('每個欄位的寫入只列那個欄位，帶讀到的 etag，成功後重讀', () async {
    final e = editor();
    for (final field in StoreTextField.values) {
      requests.clear();
      expect(await e.save(field.changes('x')), isA<StoreSaved>());
      expect(requests.single.updateMask.paths, [field.path]);
      expect(requests.single.store.etag, 'v1');
    }
    expect(reloads, StoreTextField.values.length);
  });

  test('成功：立刻採用回應的店家（新的 etag），再重讀校正', () async {
    final e = editor();
    await e.save(StoreTextField.prize.changes('得獎'));
    expect(adopted.single.etag, 'v2');
    expect(reloads, 1);
  });

  test('衝突與失敗不採用任何店家', () async {
    final e = editor();
    failure = reasonError(Code.aborted, 'VERSION_CONFLICT');
    await e.save(StoreTextField.prize.changes('得獎'));
    expect(adopted, isEmpty);
  });

  test('地址：整個換掉；清掉時 mask 有 address 但請求沒有 address', () async {
    final e = editor();
    await e.save(
      const StoreChanges().withAddress(
        townCode: '63000010',
        addressDetail: '路一號',
      ),
    );
    expect(requests.single.updateMask.paths, ['address']);
    expect(requests.single.store.address.townCode, '63000010');
    expect(requests.single.store.address.addressDetail, '路一號');

    requests.clear();
    await e.save(const StoreChanges(clearAddress: true));
    expect(requests.single.updateMask.paths, ['address']);
    expect(requests.single.store.hasAddress(), isFalse);
  });

  test('withAddress 保留圖片，mask 照固定順序', () async {
    final changes = const StoreChanges(
      logoStagedToken: 'logo',
    ).withAddress(townCode: '63000010', addressDetail: '路一號');
    expect(changes.paths, ['address', 'logo_staged_token']);
    final store = changes.toStore('v1');
    expect(store.logoStagedToken, 'logo');
    expect(store.address.townCode, '63000010');
  });

  group('食品業者登錄', () {
    late List<Object> calls;
    var omitStore = false;

    StoreEditor foodEditor() {
      omitStore = false;
      calls = [];
      adopted = [];
      reloads = 0;
      reloadOk = true;
      return StoreEditor(
        client: SellerAppServiceClient(
          FakeTransportBuilder()
              .unary(SellerAppService.submitFoodRegistration, (req, _) {
                calls.add(req);
                final error = failure;
                if (error != null) throw error;
                if (omitStore) return SubmitFoodRegistrationResponse();
                return SubmitFoodRegistrationResponse(store: Store(etag: 'v3'));
              })
              .unary(SellerAppService.withdrawFoodRegistration, (req, _) {
                calls.add(req);
                return WithdrawFoodRegistrationResponse(
                  store: Store(etag: 'v4'),
                );
              })
              .unary(SellerAppService.clearFoodRegistration, (req, _) {
                calls.add(req);
                return ClearFoodRegistrationResponse(store: Store(etag: 'v5'));
              })
              .build(),
        ),
        current: () => myStore(etag: 'v1'),
        reload: () async {
          reloads++;
          return reloadOk;
        },
        adopt: adopted.add,
        defaults: zhDefaultErrorText,
      );
    }

    test('送出、撤回、清除各打自己的方法，成功後採用回應的店家並重讀', () async {
      final e = foodEditor();
      expect(
        await e.submitFoodRegistration('A-1', _noticeAcceptances),
        isA<StoreSaved>(),
      );
      final submitted = calls.last as SubmitFoodRegistrationRequest;
      expect(submitted.registrationNumber, 'A-1');
      expect(submitted.acceptedAgreements.map((a) => (a.kind, a.version)), [
        (AgreementKind.AGREEMENT_KIND_FOOD_REGISTRATION_NOTICE, 'n1'),
      ]);
      expect(await e.withdrawFoodRegistration(), isA<StoreSaved>());
      expect(calls.last, isA<WithdrawFoodRegistrationRequest>());
      expect(await e.clearFoodRegistration(), isA<StoreSaved>());
      expect(calls.last, isA<ClearFoodRegistrationRequest>());
      expect(adopted.map((s) => s.etag), ['v3', 'v4', 'v5']);
      expect(reloads, 3);
    });

    test('回應沒有店家：不採用空的店家，仍重讀', () async {
      final e = foodEditor();
      omitStore = true;
      expect(
        await e.submitFoodRegistration('A-1', _noticeAcceptances),
        isA<StoreSaved>(),
      );
      expect(adopted, isEmpty);
      expect(reloads, 1);
    });

    test('失敗：欄位錯誤不採用店家也不重讀', () async {
      final e = foodEditor();
      failure = reasonError(Code.invalidArgument, 'INVALID_ARGUMENT');
      expect(
        await e.submitFoodRegistration('A-1', _noticeAcceptances),
        isNot(isA<StoreSaved>()),
      );
      expect(adopted, isEmpty);
      expect(reloads, 0);
    });

    test('AGREEMENT_OUTDATED：回報告知過期，不採用店家也不重讀', () async {
      final e = foodEditor();
      failure = reasonError(Code.failedPrecondition, 'AGREEMENT_OUTDATED');
      final outcome = await e.submitFoodRegistration('A-1', _noticeAcceptances);
      expect(outcome, isA<StoreSaveFailed>());
      expect((outcome as StoreSaveFailed).agreementOutdated, isTrue);
      expect(outcome.message, '條款已更新，請重新閱讀後再勾選。');
      expect(adopted, isEmpty);
      expect(reloads, 0);
    });
  });

  test('沒有要寫的欄位就不送出', () async {
    final e = editor();
    expect(await e.save(const StoreChanges()), isA<StoreSaved>());
    expect(requests, isEmpty);
    expect(reloads, 0);
  });

  test('VERSION_CONFLICT：重讀最新版本並回報衝突（輸入由呼叫端保留）', () async {
    final e = editor();
    failure = reasonError(Code.aborted, 'VERSION_CONFLICT');

    final outcome = await e.save(StoreTextField.prize.changes('得獎'));

    expect(outcome, isA<StoreSaveConflict>());
    expect((outcome as StoreSaveConflict).reloaded, isTrue);
    expect(reloads, 1);
    expect(
      storeSaveMessage(outcome),
      '商店資料已在其他地方更新\n已載入最新版本，你輸入的內容仍保留，請確認後再儲存。',
    );
    expect(
      storeSaveMessage(outcome, conflictGuidance: versionConflictImageGuidance),
      '商店資料已在其他地方更新\n已載入最新版本，請重新上傳圖片。',
    );
  });

  test('VERSION_CONFLICT 後重讀失敗：顯示無法載入最新版本', () async {
    final e = editor();
    failure = reasonError(Code.aborted, 'VERSION_CONFLICT');
    reloadOk = false;

    final outcome = await e.save(StoreTextField.prize.changes('得獎'));

    expect((outcome as StoreSaveConflict).reloaded, isFalse);
    expect(storeSaveMessage(outcome), '無法載入最新版本，請稍後再試。');
  });

  test('欄位錯誤：對應到畫面欄位的路徑', () async {
    final e = editor();
    failure = fieldError('store.contact_name', '聯絡人姓名最多 20 字');

    final outcome = await e.save(StoreTextField.contactName.changes('x'));

    expect(outcome, isA<StoreSaveFieldErrors>());
    expect(
      storeSaveMessage(
        outcome,
        fieldPaths: [StoreTextField.contactName.errorPath],
      ),
      '聯絡人姓名最多 20 字',
    );
    expect(reloads, 0);
  });

  group('StoreTextField', () {
    test('格式檢查沿用 website 的文字', () {
      expect(
        StoreTextField.contactMobile.validate('0912'),
        '請輸入 09 開頭的 10 碼手機號碼',
      );
      expect(StoreTextField.contactMobile.validate('0912345678'), isNull);
      expect(StoreTextField.supportEmail.validate('a@b'), 'Email 格式不正確');
      expect(StoreTextField.supportEmail.validate('a@b.tw'), isNull);
      expect(StoreTextField.contactName.validate('一' * 21), '聯絡人姓名最多 20 字');
      expect(
        StoreTextField.websiteUrl.validate('ftp://x.tw'),
        '請輸入有效的網站網址，例如 https://example.com',
      );
      expect(
        StoreTextField.websiteUrl.normalize('example.com'),
        'https://example.com',
      );
      expect(
        StoreTextField.contactMobile.normalize('0912-345-678'),
        '0912345678',
      );
      expect(StoreTextField.supportEmail.validate(''), isNull);
    });

    test('店名與介紹受 PayUni 鎖定，其他欄位不受', () {
      expect(
        [
          for (final f in StoreTextField.values)
            if (f.lockedByPayuni) f,
        ],
        [StoreTextField.storeName, StoreTextField.description],
      );
    });

    test('標籤用兩端一起改的名稱', () {
      expect(StoreTextField.contactPhone.label, '商店電話');
      expect(StoreTextField.supportEmail.hint, 'PayUni 會寄金流相關通知到這個信箱');
      expect(StoreTextField.contactMobile.hint, '09 開頭的 10 碼手機號碼');
    });
  });
}
