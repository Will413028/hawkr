import 'package:hawkr_api/taipei_time.dart';
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart';
import 'package:test/test.dart';

Timestamp _utc(int y, int m, int d, int h, int min) =>
    Timestamp.fromDateTime(DateTime.utc(y, m, d, h, min));

void main() {
  group('formatTaipeiTimestamp', () {
    test('UTC 16:00 是臺北隔天 00:00，日期跨日', () {
      expect(
        formatTaipeiTimestamp(_utc(2026, 7, 29, 16, 0)),
        '2026/07/30 00:00',
      );
    });

    test('UTC 15:59 仍是臺北當天 23:59', () {
      expect(
        formatTaipeiTimestamp(_utc(2026, 7, 29, 15, 59)),
        '2026/07/29 23:59',
      );
    });

    test('跨年與補零', () {
      expect(
        formatTaipeiTimestamp(_utc(2026, 12, 31, 20, 5)),
        '2027/01/01 04:05',
      );
    });

    test('不受手機時區影響（輸入是本地時間的同一個時間點）', () {
      final local = Timestamp.fromDateTime(DateTime.utc(2026, 1, 1).toLocal());
      expect(formatTaipeiTimestamp(local), '2026/01/01 08:00');
    });

    test('null 回空字串', () {
      expect(formatTaipeiTimestamp(null), '');
    });
  });

  group('formatTaipeiDateTime', () {
    test('UTC 01:05 是臺北 09:05', () {
      expect(
        formatTaipeiDateTime(DateTime.utc(2026, 10, 20, 1, 5)),
        '2026/10/20 09:05',
      );
    });

    test('UTC 16:00 跨日到臺北隔天', () {
      expect(
        formatTaipeiDateTime(DateTime.utc(2026, 12, 31, 16, 0)),
        '2027/01/01 00:00',
      );
    });

    test('不受手機時區影響（local 與 UTC 是同一個時間點）', () {
      final instant = DateTime.utc(2026, 1, 1, 0, 0);
      expect(formatTaipeiDateTime(instant.toLocal()), '2026/01/01 08:00');
      expect(formatTaipeiDateTime(instant), '2026/01/01 08:00');
    });
  });

  group('formatTaipeiDate', () {
    test('UTC 16:30 是臺北隔天', () {
      expect(
        formatTaipeiDate(DateTime.utc(2026, 10, 19, 16, 30)),
        '2026/10/20',
      );
    });

    test('UTC 15:59 仍是臺北當天', () {
      expect(
        formatTaipeiDate(DateTime.utc(2026, 10, 19, 15, 59)),
        '2026/10/19',
      );
    });

    test('跨年與補零', () {
      expect(formatTaipeiDate(DateTime.utc(2026, 12, 31, 16, 0)), '2027/01/01');
    });

    test('不受手機時區影響（local 與 UTC 是同一個時間點）', () {
      final instant = DateTime.utc(2026, 1, 1, 17, 0);
      expect(formatTaipeiDate(instant.toLocal()), '2026/01/02');
      expect(formatTaipeiDate(instant), '2026/01/02');
    });
  });

  group('TaipeiWallClock.taipei', () {
    test('欄位是臺北牆上時間，與 toLocal 的裝置時區無關', () {
      final t = DateTime.utc(2026, 7, 29, 16, 0).taipei;
      expect([t.year, t.month, t.day, t.hour, t.minute], [2026, 7, 30, 0, 0]);
      expect(t.weekday, DateTime.thursday);
    });

    test('輸入是 local 或 UTC 都以時間點計', () {
      final instant = DateTime.utc(2026, 12, 31, 20, 5);
      expect(instant.toLocal().taipei, instant.taipei);
    });
  });

  group('TaipeiWallClock.taipeiWall', () {
    test('帶時區標記（UTC）的值換成臺北', () {
      final t = DateTime.parse('2026-07-29T16:00:00Z').taipeiWall;
      expect([t.day, t.hour], [30, 0]);
    });

    test('帶位移的值以時間點計', () {
      final t = DateTime.parse('2026-07-30T00:00:00+08:00').taipeiWall;
      expect([t.day, t.hour], [30, 0]);
    });

    test('沒有時區標記的值已是臺北牆上時間，不因裝置時區位移', () {
      final t = DateTime.parse('2026-07-30 08:30:00').taipeiWall;
      expect([t.year, t.month, t.day, t.hour, t.minute], [2026, 7, 30, 8, 30]);
    });
  });
}
