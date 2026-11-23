import 'package:hawkr_api/taipei_time.dart';
import 'package:test/test.dart';

Timestamp _utc(int y, int m, int d, int h, int min) =>
    Timestamp.fromDateTime(DateTime.utc(y, m, d, h, min));

void main() {
  group('plainTimestamp', () {
    test('UTC 16:00 是臺北隔天 00:00，日期跨日', () {
      expect(
        TaipeiDisplay.plainTimestamp(_utc(2026, 7, 29, 16, 0)),
        '2026/07/30 00:00',
      );
    });

    test('UTC 15:59 仍是臺北當天 23:59', () {
      expect(
        TaipeiDisplay.plainTimestamp(_utc(2026, 7, 29, 15, 59)),
        '2026/07/29 23:59',
      );
    });

    test('跨年與補零', () {
      expect(
        TaipeiDisplay.plainTimestamp(_utc(2026, 12, 31, 20, 5)),
        '2027/01/01 04:05',
      );
    });

    test('不受手機時區影響（輸入是本地時間的同一個時間點）', () {
      final local = Timestamp.fromDateTime(DateTime.utc(2026, 1, 1).toLocal());
      expect(TaipeiDisplay.plainTimestamp(local), '2026/01/01 08:00');
    });

    test('null 回空字串', () {
      expect(TaipeiDisplay.plainTimestamp(null), '');
    });
  });

  group('plain', () {
    test('UTC 01:05 是臺北 09:05', () {
      expect(
        TaipeiDisplay.plain(DateTime.utc(2026, 10, 20, 1, 5)),
        '2026/10/20 09:05',
      );
    });

    test('UTC 16:00 跨日到臺北隔天', () {
      expect(
        TaipeiDisplay.plain(DateTime.utc(2026, 12, 31, 16, 0)),
        '2027/01/01 00:00',
      );
    });

    test('不受手機時區影響（local 與 UTC 是同一個時間點）', () {
      final instant = DateTime.utc(2026, 1, 1, 0, 0);
      expect(TaipeiDisplay.plain(instant.toLocal()), '2026/01/01 08:00');
      expect(TaipeiDisplay.plain(instant), '2026/01/01 08:00');
    });
  });

  group('plain date', () {
    test('UTC 16:30 是臺北隔天', () {
      expect(
        TaipeiDisplay.plain(
          DateTime.utc(2026, 10, 19, 16, 30),
          TaipeiFormat.date,
        ),
        '2026/10/20',
      );
    });

    test('UTC 15:59 仍是臺北當天', () {
      expect(
        TaipeiDisplay.plain(
          DateTime.utc(2026, 10, 19, 15, 59),
          TaipeiFormat.date,
        ),
        '2026/10/19',
      );
    });

    test('跨年與補零', () {
      expect(
        TaipeiDisplay.plain(
          DateTime.utc(2026, 12, 31, 16, 0),
          TaipeiFormat.date,
        ),
        '2027/01/01',
      );
    });

    test('不受手機時區影響（local 與 UTC 是同一個時間點）', () {
      final instant = DateTime.utc(2026, 1, 1, 17, 0);
      expect(
        TaipeiDisplay.plain(instant.toLocal(), TaipeiFormat.date),
        '2026/01/02',
      );
      expect(TaipeiDisplay.plain(instant, TaipeiFormat.date), '2026/01/02');
    });
  });

  group('plain (instant semantics)', () {
    test('fields are Taipei wall clock, independent of the device zone', () {
      expect(
        TaipeiDisplay.plain(DateTime.utc(2026, 7, 29, 16, 0)),
        '2026/07/30 00:00',
      );
    });

    test('local or UTC input is the same instant', () {
      final instant = DateTime.utc(2026, 12, 31, 20, 5);
      expect(
        TaipeiDisplay.plain(instant.toLocal()),
        TaipeiDisplay.plain(instant),
      );
    });
  });

  group('plainIso (string semantics)', () {
    test('marked UTC strings convert to Taipei', () {
      expect(
        TaipeiDisplay.plainIso('2026-07-29T16:00:00Z'),
        '2026/07/30 00:00',
      );
    });

    test('offset strings are instants', () {
      expect(
        TaipeiDisplay.plainIso('2026-07-30T00:00:00+08:00'),
        '2026/07/30 00:00',
      );
    });

    test('unmarked strings are already Taipei wall clock', () {
      expect(TaipeiDisplay.plainIso('2026-07-30 08:30:00'), '2026/07/30 08:30');
    });

    test('empty and unparseable are null', () {
      expect(TaipeiDisplay.plainIso(''), isNull);
      expect(TaipeiDisplay.plainIso('not a time'), isNull);
    });

    test('10-digit seconds and 13-digit milliseconds are epoch', () {
      // 1785398400 = 2026-07-30T08:00:00Z
      expect(TaipeiDisplay.plainIso('1785398400'), '2026/07/30 16:00');
      expect(TaipeiDisplay.plainIso('1785398400000'), '2026/07/30 16:00');
    });

    test('slash dates parse as Taipei wall clock', () {
      expect(TaipeiDisplay.plainIso('2026/07/30 08:30'), '2026/07/30 08:30');
      expect(
        TaipeiDisplay.plainIso('2026/07/30', TaipeiFormat.dashDate),
        '2026-07-30',
      );
    });
  });

  group('TaipeiFormat', () {
    final t = DateTime.utc(2026, 7, 4, 16, 5); // Taipei 2026-07-05 00:05
    test('each layout', () {
      String f(TaipeiFormat x) => TaipeiDisplay.plain(t, x);
      expect(f(TaipeiFormat.dateTime), '2026/07/05 00:05');
      expect(f(TaipeiFormat.date), '2026/07/05');
      expect(f(TaipeiFormat.dashDate), '2026-07-05');
      expect(f(TaipeiFormat.dashDateTime), '2026-07-05 00:05');
      expect(f(TaipeiFormat.monthDayTime), '07/05 00:05');
      expect(f(TaipeiFormat.monthDayTimeUnpadded), '7/5 00:05');
      expect(f(TaipeiFormat.cjkDateTime), '2026年07月05日 00:05');
      expect(f(TaipeiFormat.time), '00:05');
      expect(f(TaipeiFormat.meridiemTime), '上午12:05');
    });

    test('meridiem around noon', () {
      String f(int utcHour) => TaipeiDisplay.plain(
        DateTime.utc(2026, 7, 5, utcHour, 30),
        TaipeiFormat.meridiemTime,
      );
      expect(f(3), '上午11:30');
      expect(f(4), '下午12:30');
      expect(f(5), '下午01:30');
    });
  });

  group('plainSeparator / isBeforeTaipeiToday', () {
    // now = 2026-07-31 00:10 Taipei.
    final now = DateTime.utc(2026, 7, 30, 16, 10);
    test('same Taipei day shows the time only, else month/day', () {
      expect(
        TaipeiDisplay.plainSeparator(
          DateTime.utc(2026, 7, 30, 16, 5),
          now: now,
        ),
        '00:05',
      );
      expect(
        TaipeiDisplay.plainSeparator(
          DateTime.utc(2026, 7, 30, 15, 59),
          now: now,
        ),
        '7/30 23:59',
      );
    });

    test('compares Taipei calendar days', () {
      expect(
        TaipeiDisplay.isBeforeTaipeiToday('2026-07-30T15:59:00Z', now: now),
        isTrue,
      );
      expect(
        TaipeiDisplay.isBeforeTaipeiToday('2026-07-30T16:00:00Z', now: now),
        isFalse,
      );
      expect(TaipeiDisplay.isBeforeTaipeiToday('garbage', now: now), isFalse);
    });
  });

  group('TaipeiDisplay label', () {
    const foreign = TaipeiDisplay(
      label: '台北時間',
      deviceOffset: Duration(hours: -5),
    );
    const home = TaipeiDisplay(label: '台北時間', deviceOffset: Duration(hours: 8));
    final instant = DateTime.utc(2026, 7, 30, 7, 5);

    test('+8 device: no label; other offsets append it', () {
      expect(home.format(instant), '2026/07/30 15:05');
      expect(foreign.format(instant), '2026/07/30 15:05 (台北時間)');
      expect(
        foreign.formatIso('2026-07-30T07:05:00Z', TaipeiFormat.date),
        '2026/07/30 (台北時間)',
      );
    });

    test('unparseable and empty strings are returned without a label', () {
      expect(foreign.formatIso('not a time'), 'not a time');
      expect(foreign.formatIso(''), '');
    });

    test('labelIfNeeded', () {
      expect(home.labelIfNeeded, isNull);
      expect(foreign.labelIfNeeded, '台北時間');
    });

    test('timeAgo labels only the date forms', () {
      final now = DateTime.utc(2026, 8, 1, 2);
      String text(TimeAgo a) => 'x';
      expect(
        foreign.timeAgo(now.subtract(const Duration(hours: 2)), text, now: now),
        'x',
      );
      expect(
        foreign.timeAgo(DateTime.utc(2026, 7, 1), text, now: now),
        'x (台北時間)',
      );
    });
  });

  group('picker conversions', () {
    test('initial is one day after now in Taipei wall clock', () {
      final now = DateTime.utc(2026, 7, 30, 16, 0); // Taipei 07-31 00:00
      final base = taipeiPickerInitial(null, now);
      expect([base.year, base.month, base.day, base.hour], [2026, 8, 1, 0]);
    });

    test('picked wall fields convert back to the instant', () {
      expect(
        taipeiInstantOfPicked(2026, 8, 1, 0, 0),
        DateTime.utc(2026, 7, 31, 16, 0),
      );
    });
  });

  group('taipeiCalendarDaysAgo / formatTaipeiTimeAgo', () {
    // 2026-07-30 16:00Z is 2026-07-31 00:00 in Taipei.
    final before = DateTime.utc(2026, 7, 30, 15, 59);
    final after = DateTime.utc(2026, 7, 30, 16, 1);

    test('counts Taipei calendar days across the UTC 16:00 boundary', () {
      final now = DateTime.utc(2026, 7, 31, 1);
      expect(taipeiCalendarDaysAgo(before, now: now), 1);
      expect(taipeiCalendarDaysAgo(after, now: now), 0);
    });

    test('labels use elapsed time under a day and calendar days after', () {
      final now = DateTime.utc(2026, 8, 1, 2);
      expect(
        taipeiTimeAgo(now.subtract(const Duration(seconds: 5)), now: now),
        const TimeAgoJustNow(),
      );
      expect(
        taipeiTimeAgo(now.subtract(const Duration(seconds: 59)), now: now),
        const TimeAgoJustNow(),
      );
      expect(
        taipeiTimeAgo(now.subtract(const Duration(seconds: 60)), now: now),
        const TimeAgoMinutes(1),
      );
      expect(
        taipeiTimeAgo(
          now.subtract(const Duration(minutes: 59, seconds: 59)),
          now: now,
        ),
        const TimeAgoMinutes(59),
      );
      expect(
        taipeiTimeAgo(now.subtract(const Duration(hours: 1)), now: now),
        const TimeAgoHours(1),
      );
      expect(
        taipeiTimeAgo(
          now.subtract(const Duration(hours: 23, minutes: 59)),
          now: now,
        ),
        const TimeAgoHours(23),
      );
      expect(
        taipeiTimeAgo(now.add(const Duration(minutes: 5)), now: now),
        const TimeAgoJustNow(),
      );
      expect(taipeiTimeAgo(before, now: now), const TimeAgoDays(2));
      expect(
        taipeiTimeAgo(DateTime.utc(2026, 7, 1), now: now),
        const TimeAgoMonthDay(7, 1),
      );
      expect(
        taipeiTimeAgo(DateTime.utc(2025, 7, 1), now: now),
        const TimeAgoFullDate(2025, 7, 1),
      );
    });

    test('over 24h but one Taipei calendar day ago across midnight', () {
      // now = 2026-07-31 00:10 Taipei; time = 2026-07-30 00:05 Taipei (24h05 earlier).
      final now = DateTime.utc(2026, 7, 30, 16, 10);
      expect(
        taipeiTimeAgo(DateTime.utc(2026, 7, 29, 16, 5), now: now),
        const TimeAgoDays(1),
      );
      // 7 days boundary: 2026-07-24 00:05 Taipei is 7 calendar days ago.
      expect(
        taipeiTimeAgo(DateTime.utc(2026, 7, 23, 16, 5), now: now),
        const TimeAgoMonthDay(7, 24),
      );
      expect(
        taipeiTimeAgo(DateTime.utc(2026, 7, 24, 16, 5), now: now),
        const TimeAgoDays(6),
      );
    });

    test('year is the Taipei year, not the UTC year', () {
      // 2026-01-01 00:30 Taipei is still 2025-12-31 UTC.
      final now = DateTime.utc(2025, 12, 31, 16, 30);
      expect(
        taipeiTimeAgo(DateTime.utc(2025, 12, 1), now: now),
        const TimeAgoFullDate(2025, 12, 1),
      );
    });
  });

  group('tryParseTaipeiInstant', () {
    test('unmarked strings are Taipei wall clock', () {
      expect(
        tryParseTaipeiInstant('2026-07-30 08:30:00'),
        DateTime.utc(2026, 7, 30, 0, 30),
      );
      expect(
        tryParseTaipeiInstant('2026-07-30T00:10:00'),
        DateTime.utc(2026, 7, 29, 16, 10),
      );
    });

    test('marked strings are kept as instants', () {
      expect(
        tryParseTaipeiInstant('2026-07-30T00:30:00Z'),
        DateTime.utc(2026, 7, 30, 0, 30),
      );
      expect(
        tryParseTaipeiInstant('2026-07-30T08:30:00+08:00'),
        DateTime.utc(2026, 7, 30, 0, 30),
      );
    });

    test('unparseable input is null', () {
      expect(tryParseTaipeiInstant('not a time'), isNull);
    });

    test(
      'an unmarked wall clock reads the same relative label from a UTC-5 device',
      () {
        // The device offset never enters the pure function: the same wall clock string gives the same instant.
        final instant = tryParseTaipeiInstant('2026-07-30 08:30:00')!;
        final now = DateTime.utc(
          2026,
          7,
          30,
          2,
          30,
        ); // 10:30 Taipei, 21:30 the day before at UTC-5.
        expect(taipeiTimeAgo(instant, now: now), const TimeAgoHours(2));
      },
    );
  });

  group('isTaipeiOffset', () {
    test('only +8 is Taipei', () {
      expect(isTaipeiOffset(const Duration(hours: 8)), isTrue);
      expect(isTaipeiOffset(const Duration(hours: -5)), isFalse);
      expect(isTaipeiOffset(Duration.zero), isFalse);
      expect(isTaipeiOffset(const Duration(hours: 9)), isFalse);
    });
  });

  group('withTaipeiLabel', () {
    const label = '台北時間';
    const text = '2026/07/30 15:05';

    test('+8 offset leaves the text alone', () {
      expect(
        withTaipeiLabelAtOffset(text, label, const Duration(hours: 8)),
        text,
      );
    });

    test('non +8 offsets append the label', () {
      for (final h in [-5, 0, 9]) {
        expect(
          withTaipeiLabelAtOffset(text, label, Duration(hours: h)),
          '$text ($label)',
        );
      }
    });

    test('empty text stays empty for any offset', () {
      expect(withTaipeiLabelAtOffset('', label, Duration.zero), '');
    });

    test('now injects the device offset (UTC now is offset 0)', () {
      expect(
        withTaipeiLabel(text, label, now: DateTime.utc(2026, 7, 30)),
        '$text ($label)',
      );
    });
  });
}
