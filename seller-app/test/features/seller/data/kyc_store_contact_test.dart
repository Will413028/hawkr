import 'package:hawkr_seller/features/seller/data/kyc_store_contact.dart';
import 'package:hawkr_seller/features/seller/data/store_form.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../../support/my_store_fixtures.dart';

void main() {
  group('pendingStoreChanges', () {
    const server = StoreContactDraft(
      contactName: '王小明',
      contactMobile: '0912345678',
      supportEmail: 'shop@example.com',
      townCode: '63000010',
      addressDetail: '路一號',
    );

    test('沒有改動就不寫', () {
      expect(pendingStoreChanges(server, server), isNull);
    });

    test('前後空白不算改動', () {
      expect(
        pendingStoreChanges(server.copyWith(contactName: ' 王小明 '), server),
        isNull,
      );
    });

    test('只列改過的欄位；手機去掉橫線與空白', () {
      final changes = pendingStoreChanges(
        server.copyWith(
          contactMobile: '0912-345-679',
          supportEmail: ' a@b.co ',
        ),
        server,
      )!;

      expect(changes.paths, ['contact_mobile', 'support_email']);
      expect(changes.text[StoreTextField.contactMobile], '0912345679');
      expect(changes.text[StoreTextField.supportEmail], 'a@b.co');
      expect(changes.address, isNull);
    });

    test('地址整個換掉，用 town_code 與明細', () {
      final changes = pendingStoreChanges(
        server.copyWith(townCode: '63000020', addressDetail: '路二號'),
        server,
      )!;

      expect(changes.paths, ['address']);
      expect(changes.address?.townCode, '63000020');
      expect(changes.address?.addressDetail, '路二號');
    });

    test('地址清成空白：列進 mask 但不帶地址', () {
      final changes = pendingStoreChanges(
        server.copyWith(townCode: '', addressDetail: ''),
        server,
      )!;

      expect(changes.clearAddress, isTrue);
      expect(changes.address, isNull);
      expect(changes.paths, ['address']);
    });
  });

  group('rebaseStoreContact', () {
    const was = StoreContactDraft(
      contactName: '舊',
      supportEmail: 'old@example.com',
      townCode: '63000010',
      addressDetail: '路一號',
    );

    test('使用者沒改的欄位取最新（別處改的不當成使用者的修改寫回去），改過的保留', () {
      final rebased = rebaseStoreContact(
        draft: was.copyWith(contactName: '我改的'),
        was: was,
        latest: was.copyWith(
          contactName: '別處改的',
          supportEmail: 'new@example.com',
          addressDetail: '路九號',
        ),
      );

      expect(rebased.contactName, '我改的');
      expect(rebased.supportEmail, 'new@example.com');
      expect(rebased.addressDetail, '路九號');
    });

    test('地址改過就整組保留', () {
      final rebased = rebaseStoreContact(
        draft: was.copyWith(townCode: '63000020', addressDetail: '路二號'),
        was: was,
        latest: was.copyWith(addressDetail: '路九號'),
      );

      expect(rebased.townCode, '63000020');
      expect(rebased.addressDetail, '路二號');
    });
  });

  group('格式檢查', () {
    test('已填的欄位檢查格式，空白不報（是否必填由伺服器決定）', () {
      expect(storeContactErrors(const StoreContactDraft()).any, isFalse);

      final errors = storeContactErrors(
        const StoreContactDraft(
          contactName: '一二三四五六七八九十一二三四五六七八九十一',
          contactMobile: '123',
          supportEmail: 'nope',
        ),
      );
      expect(errors.name, '聯絡人姓名最多 20 字');
      expect(errors.mobile, '請輸入 09 開頭的 10 碼手機號碼');
      expect(errors.email, 'Email 格式不正確');
    });

    test('只有明細沒有鄉鎮市區的地址不完整', () {
      expect(
        storeAddressInputError(const StoreContactDraft(addressDetail: '路')),
        '請選擇縣市與鄉鎮市區',
      );
      expect(storeAddressInputError(const StoreContactDraft()), isNull);
    });

    test('UpdateMyStore 的欄位錯誤對應到聯絡欄位', () {
      final errors = storeContactServerErrors({
        'store.contact_name': 'a',
        'store.support_email': 'b',
        'store.description': 'ignored',
      });
      expect(errors.name, 'a');
      expect(errors.email, 'b');
      expect(errors.mobile, isNull);
    });
  });

  test('從店家讀出認證頁的輸入形狀', () {
    final store = myStore(townCode: '63000010', addressDetail: ' 路一號 ').store;
    final draft = storeContactDraftOf(store);

    expect(draft.townCode, '63000010');
    expect(draft.addressDetail, '路一號');
    expect(storeContactDraftOf(null), const StoreContactDraft());
  });
}
