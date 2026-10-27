import 'bank_book_ocr_service.dart';
import 'id_card_ocr_service.dart';
import 'kyc_form.dart';

/// 身分證辨識的結果預填進表單：只填空白的欄位，使用者輸入過（或已存檔、只剩遮蔽值）的欄位不覆蓋。身分已被 PayUni 鎖住
/// （[identityLocked]）時姓名與證號不動。[hasMaskedId] 為真表示伺服器上已經有證號。
KycForm applyIdCardOcr(
  KycForm form,
  IdCardOcrResult result, {
  required bool hasMaskedId,
  required bool identityLocked,
}) {
  String fill(String current, String? recognized) =>
      current.isEmpty && recognized != null && recognized.isNotEmpty
      ? recognized
      : current;
  String fillDate(String current, String? roc) =>
      current.isEmpty && roc != null && isoOfRocDate(roc).isNotEmpty
      ? isoOfRocDate(roc)
      : current;
  return form.copyWith(
    realName: identityLocked ? null : fill(form.realName, result.name),
    idCardNo: identityLocked || hasMaskedId
        ? null
        : fill(form.idCardNo, result.idNumber),
    birthDate: fillDate(form.birthDate, result.birthDate),
    idCardIssueDate: fillDate(form.idCardIssueDate, result.issueDate),
    idCardIssueType: fill(form.idCardIssueType, result.issueType),
    idCardIssueLocation: fill(form.idCardIssueLocation, result.issueLocation),
  );
}

/// 存摺辨識的結果預填進表單：只填空白的欄位。[hasMaskedAccount] 為真表示伺服器上已經有帳號。
KycForm applyBankBookOcr(
  KycForm form,
  BankBookOcrResult result, {
  required bool hasMaskedAccount,
}) {
  final account = result.accountNumber?.replaceAll(RegExp(r'\D'), '') ?? '';
  return form.copyWith(
    bankName: form.bankName.isEmpty && (result.bankName ?? '').isNotEmpty
        ? result.bankName
        : null,
    bankAccount:
        form.bankAccount.isEmpty && !hasMaskedAccount && account.isNotEmpty
        ? account
        : null,
  );
}
