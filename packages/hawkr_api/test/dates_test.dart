import 'package:hawkr_api/dates.dart';
import 'package:hawkr_api/gen/google/type/date.pb.dart';
import 'package:test/test.dart';

void main() {
  group('parseDate', () {
    test('合法日期', () {
      final d = parseDate('1990-05-07')!;
      expect((d.year, d.month, d.day), (1990, 5, 7));
    });

    test('閏日只在閏年存在', () {
      expect(parseDate('2024-02-29'), isNotNull);
      expect(parseDate('2023-02-29'), isNull);
    });

    test('不存在的月與日', () {
      expect(parseDate('2023-13-01'), isNull);
      expect(parseDate('2023-04-31'), isNull);
      expect(parseDate('2023-00-10'), isNull);
      expect(parseDate('2023-01-00'), isNull);
    });

    test('年份 0000 是 google.type.Date 的「沒有年份」，拒絕', () {
      expect(parseDate('0000-01-01'), isNull);
      expect(parseDate('0001-01-01'), isNotNull);
      expect(parseDate('9999-12-31'), isNotNull);
    });

    test('格式不符一律 null', () {
      for (final v in [
        '',
        '1990-5-7',
        '1990/05/07',
        '19900507',
        ' 1990-05-07',
        '1990-05-07 ',
        'abcd-ef-gh',
      ]) {
        expect(parseDate(v), isNull, reason: v);
      }
    });
  });

  group('dateToDateTime／dateFromDateTime', () {
    test('完整的日期往返', () {
      final d = Date(year: 1990, month: 5, day: 20);
      final t = dateToDateTime(d)!;
      expect([t.year, t.month, t.day], [1990, 5, 20]);
      expect(dateFromDateTime(t), d);
    });

    test('null 與任何欄位為 0 回 null，規則同 formatDate', () {
      for (final d in [
        null,
        Date(),
        Date(year: 0, month: 5, day: 7),
        Date(year: 1990, month: 0, day: 7),
        Date(year: 1990, month: 5, day: 0),
      ]) {
        expect(dateToDateTime(d), isNull, reason: '$d');
        expect(isDateSet(d), isFalse, reason: '$d');
        expect(formatDate(d), '', reason: '$d');
      }
    });
  });

  group('formatDate', () {
    test('補零成 YYYY-MM-DD', () {
      expect(formatDate(Date(year: 990, month: 5, day: 7)), '0990-05-07');
      expect(formatDate(Date(year: 2023, month: 12, day: 31)), '2023-12-31');
    });

    test('null 與任何欄位為 0 回空字串', () {
      expect(formatDate(null), '');
      expect(formatDate(Date()), '');
      expect(formatDate(Date(year: 0, month: 5, day: 7)), '');
      expect(formatDate(Date(year: 1990, month: 0, day: 7)), '');
      expect(formatDate(Date(year: 1990, month: 5, day: 0)), '');
    });

    test('與 parseDate 往返', () {
      expect(formatDate(parseDate('2024-02-29')), '2024-02-29');
    });
  });
}
