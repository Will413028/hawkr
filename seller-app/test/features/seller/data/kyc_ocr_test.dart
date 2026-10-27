import 'package:hawkr_seller/features/seller/data/bank_book_ocr_service.dart';
import 'package:hawkr_seller/features/seller/data/id_card_ocr_service.dart';
import 'package:hawkr_seller/features/seller/data/kyc_form.dart';
import 'package:hawkr_seller/features/seller/data/kyc_ocr.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  final id = IdCardOcrResult(
    name: '辨識姓名',
    idNumber: 'A123456789',
    birthDate: '077/06/10',
    issueDate: '107/06/10',
    issueType: '換發',
    issueLocation: '桃園市',
  );

  group('身分證 OCR', () {
    test('空白欄位預填（民國年轉成 ISO）', () {
      final form = applyIdCardOcr(
        const KycForm(),
        id,
        hasMaskedId: false,
        identityLocked: false,
      );

      expect(form.realName, '辨識姓名');
      expect(form.idCardNo, 'A123456789');
      expect(form.birthDate, '1988-06-10');
      expect(form.idCardIssueDate, '2018-06-10');
      expect(form.idCardIssueType, '換發');
      expect(form.idCardIssueLocation, '桃園市');
    });

    test('使用者填過的欄位不覆蓋', () {
      const mine = KycForm(
        realName: '我填的',
        idCardNo: 'B123456789',
        birthDate: '1990-01-01',
        idCardIssueDate: '2020-01-01',
        idCardIssueType: '初發',
        idCardIssueLocation: '台北市',
      );

      expect(
        applyIdCardOcr(mine, id, hasMaskedId: false, identityLocked: false),
        mine,
      );
    });

    test('伺服器上已有證號（只剩遮蔽值）時不預填證號；身分已鎖住時姓名與證號都不動', () {
      expect(
        applyIdCardOcr(
          const KycForm(),
          id,
          hasMaskedId: true,
          identityLocked: false,
        ).idCardNo,
        isEmpty,
      );
      final locked = applyIdCardOcr(
        const KycForm(),
        id,
        hasMaskedId: false,
        identityLocked: true,
      );
      expect(locked.realName, isEmpty);
      expect(locked.idCardNo, isEmpty);
      expect(locked.birthDate, '1988-06-10');
    });

    test('辨識不到或日期無效的欄位維持原樣', () {
      final form = applyIdCardOcr(
        const KycForm(),
        IdCardOcrResult(birthDate: '115/02/30'),
        hasMaskedId: false,
        identityLocked: false,
      );
      expect(form, const KycForm());
    });
  });

  group('存摺 OCR', () {
    final book = BankBookOcrResult(
      bankName: '台北富邦銀行',
      accountNumber: '1234-5678-9012',
    );

    test('空白欄位預填，帳號只留數字', () {
      final form = applyBankBookOcr(
        const KycForm(),
        book,
        hasMaskedAccount: false,
      );
      expect(form.bankName, '台北富邦銀行');
      expect(form.bankAccount, '123456789012');
    });

    test('已填的銀行名稱與帳號、伺服器上已有帳號時都不覆蓋', () {
      const mine = KycForm(bankName: '我填的', bankAccount: '99999999');
      expect(applyBankBookOcr(mine, book, hasMaskedAccount: false), mine);
      expect(
        applyBankBookOcr(
          const KycForm(),
          book,
          hasMaskedAccount: true,
        ).bankAccount,
        isEmpty,
      );
    });
  });
}
