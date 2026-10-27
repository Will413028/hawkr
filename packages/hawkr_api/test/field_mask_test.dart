import 'package:hawkr_api/field_mask.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pb.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:test/test.dart';

void main() {
  test('頂層欄位用 proto 的 snake_case 名稱', () {
    final mask = fieldMask(Profile(), ['display_name', 'birth_date']);
    expect(mask.paths, ['display_name', 'birth_date']);
  });

  test('巢狀訊息欄位用點串接', () {
    final mask = fieldMask(KYCApplication(), ['business.rep_id_card_no']);
    expect(mask.paths, ['business.rep_id_card_no']);
  });

  test('空的路徑清單得到空 mask', () {
    expect(fieldMask(Profile(), const []).paths, isEmpty);
  });

  test('未知的頂層欄位丟 ArgumentError', () {
    expect(() => fieldMask(Profile(), ['nope']), throwsArgumentError);
  });

  test('Dart 的 camelCase 名稱不是 proto 路徑', () {
    expect(() => fieldMask(Profile(), ['displayName']), throwsArgumentError);
  });

  test('未知的巢狀欄位丟 ArgumentError', () {
    expect(
      () => fieldMask(KYCApplication(), ['business.nope']),
      throwsArgumentError,
    );
  });

  test('純量欄位後面不能再接子路徑', () {
    expect(() => fieldMask(Profile(), ['display_name.x']), throwsArgumentError);
  });

  test('只要有一個路徑不合法就整個失敗', () {
    expect(
      () => fieldMask(Profile(), ['display_name', 'nope']),
      throwsArgumentError,
    );
  });

  test('oneof 的子訊息可以往下一層（個人賣家）', () {
    expect(fieldMask(KYCApplication(), ['individual.id_card_no']).paths, [
      'individual.id_card_no',
    ]);
  });

  test('非 oneof 的子訊息後面不能再接子路徑', () {
    for (final path in ['bank_book.asset_id', 'birth_date.year']) {
      expect(
        () => fieldMask(KYCApplication(), [path]),
        throwsA(
          isA<ArgumentError>().having(
            (e) => e.message,
            'message',
            contains('不是 oneof 的子訊息'),
          ),
        ),
        reason: path,
      );
    }
  });

  test('下到 oneof 分支之後，內層的子訊息欄位不能再往下', () {
    expect(fieldMask(KYCApplication(), ['business.company_address']).paths, [
      'business.company_address',
    ]);
    expect(
      () => fieldMask(KYCApplication(), ['business.company_address.town_code']),
      throwsA(
        isA<ArgumentError>().having(
          (e) => e.message,
          'message',
          contains('不是 oneof 的子訊息'),
        ),
      ),
    );
  });
}
