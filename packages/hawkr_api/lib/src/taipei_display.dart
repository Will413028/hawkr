part of '../taipei_time.dart';

/// 臺北時間顯示的版面；每個值對應一種 App 既有的版面，不新增樣式。
enum TaipeiFormat {
  /// `2026/07/30 15:05`
  dateTime,

  /// `2026/07/30`
  date,

  /// `2026-07-30`
  dashDate,

  /// `2026-07-30 15:05`
  dashDateTime,

  /// `07/30 15:05`
  monthDayTime,

  /// `7/30 15:05`（月日不補零）
  monthDayTimeUnpadded,

  /// `2026年07月30日 15:05`
  cjkDateTime,

  /// `15:05`
  time,

  /// `下午03:05`（12 小時制，午別是中文）
  meridiemTime,
}

String _render(DateTime t, TaipeiFormat format) {
  final mm = _pad(t.month);
  final dd = _pad(t.day);
  final hm = '${_pad(t.hour)}:${_pad(t.minute)}';
  return switch (format) {
    TaipeiFormat.dateTime => '${t.year}/$mm/$dd $hm',
    TaipeiFormat.date => '${t.year}/$mm/$dd',
    TaipeiFormat.dashDate => '${t.year}-$mm-$dd',
    TaipeiFormat.dashDateTime => '${t.year}-$mm-$dd $hm',
    TaipeiFormat.monthDayTime => '$mm/$dd $hm',
    TaipeiFormat.monthDayTimeUnpadded => '${t.month}/${t.day} $hm',
    TaipeiFormat.cjkDateTime => '${t.year}年$mm月$dd日 $hm',
    TaipeiFormat.time => hm,
    TaipeiFormat.meridiemTime =>
      '${t.hour < 12 ? '上午' : '下午'}'
          '${_pad(t.hour == 0 ? 12 : (t.hour > 12 ? t.hour - 12 : t.hour))}'
          ':${_pad(t.minute)}',
  };
}

/// 時間字串解析成臺北牆上時間；無法解析回 null。
///
/// 接受 ISO 8601（帶時區標記的是時間點，沒有標記的本來就是臺北牆上時間）、斜線日期（`2026/07/30 15:05`），
/// 與純數字的 Unix 時間（13 位以上取前 13 位當毫秒，10 至 12 位取前 10 位當秒），文字裡夾的 10 至 13 位數字也算。純數字先判斷，
/// 否則 `DateTime.tryParse` 會把 10 位秒數讀成西元 171238 年。
DateTime? _parseWall(String raw) {
  final text = raw.trim();
  if (text.isEmpty) return null;
  if (RegExp(r'^\d+$').hasMatch(text)) {
    if (text.length >= 13) {
      return DateTime.fromMillisecondsSinceEpoch(
        int.parse(text.substring(0, 13)),
        isUtc: true,
      )._taipei;
    }
    if (text.length >= 10) {
      return DateTime.fromMillisecondsSinceEpoch(
        int.parse(text.substring(0, 10)) * 1000,
        isUtc: true,
      )._taipei;
    }
  }
  final parsed =
      DateTime.tryParse(text) ??
      DateTime.tryParse(text.replaceFirst(' ', 'T').replaceAll('/', '-'));
  if (parsed != null) return parsed._taipeiWall;
  // 夾在文字裡的 Unix 時間（`ts=1785429000`）。
  final embedded = RegExp(r'\d{10,13}').firstMatch(text)?.group(0);
  return embedded == null ? null : _parseWall(embedded);
}

/// 臺北時間的顯示：格式化與「台北時間」標籤都只走這裡（ADR 多語系架構 D5）。
///
/// 裝置偏移不是 +8 時，顯示出來的臺北時間後面附標籤；一行裡有多個時間時，用 `plain*` 組好整行，再用 [labeled] 附一次。
/// 臺北牆上時間的欄位不對 App 公開，App 只能經這個類別取字串。`toLocal` 編譯器管不到，由 CI 的
/// `scripts/check-taipei-display.py` 擋。
class TaipeiDisplay {
  const TaipeiDisplay({required this.label, this.deviceOffset});

  /// 各 App ARB 的 `taipeiTimeLabel`。
  final String label;

  /// 裝置的 UTC 偏移；null 取 `DateTime.now()` 的偏移。測試注入用。
  final Duration? deviceOffset;

  Duration get _offset => deviceOffset ?? DateTime.now().timeZoneOffset;

  /// 偏移不是 +8 時是 [label]，否則 null（給輸入框的 helperText 這類獨立顯示的位置）。
  String? get labelIfNeeded => isTaipeiOffset(_offset) ? null : label;

  /// 附標籤（偏移是 +8 或 [formatted] 為空時原樣）。
  String labeled(String formatted) =>
      withTaipeiLabelAtOffset(formatted, label, _offset);

  // ── 不附標籤的字串（組整行用；整行再用 [labeled] 附一次）──

  /// [timestamp] 為 null 時空字串。proto3 訊息欄位沒設時讀到 epoch，呼叫端先用 `has*()` 確認。
  static String plainTimestamp(Timestamp? timestamp) =>
      _formatTaipeiTimestamp(timestamp);

  /// [instant] 的臺北時間；local 或 UTC 都以時間點計。
  static String plain(DateTime instant, [TaipeiFormat? format]) =>
      _render(instant._taipei, format ?? TaipeiFormat.dateTime);

  /// 時間字串 [raw] 的臺北時間（解析規則見 [_parseWall]）；空字串或解析不了回 null。
  static String? plainIso(String raw, [TaipeiFormat? format]) {
    final wall = _parseWall(raw);
    return wall == null ? null : _render(wall, format ?? TaipeiFormat.dateTime);
  }

  /// [time] 是臺北的今天則 `15:05`，否則 `7/30 15:05`。
  static String plainSeparator(DateTime time, {DateTime? now}) {
    final t = time._taipei;
    final today = (now ?? DateTime.now())._taipei;
    final isToday =
        t.year == today.year && t.month == today.month && t.day == today.day;
    return _render(
      t,
      isToday ? TaipeiFormat.time : TaipeiFormat.monthDayTimeUnpadded,
    );
  }

  /// [raw] 的臺北日期是否早於臺北的今天；解析不了回 false。
  static bool isBeforeTaipeiToday(String raw, {DateTime? now}) {
    final wall = _parseWall(raw);
    if (wall == null) return false;
    final today = (now ?? DateTime.now())._taipei;
    return DateTime.utc(
      wall.year,
      wall.month,
      wall.day,
    ).isBefore(DateTime.utc(today.year, today.month, today.day));
  }

  // ── 必要時附標籤的字串 ──

  /// `2026/07/30 15:05`，必要時附標籤；[timestamp] 為 null 時空字串。
  String timestamp(Timestamp? timestamp) => labeled(plainTimestamp(timestamp));

  /// [instant] 依 [format]（預設 `2026/07/30 15:05`）。
  String format(DateTime instant, [TaipeiFormat? format]) =>
      labeled(plain(instant, format));

  String dateTime(DateTime instant) => format(instant, TaipeiFormat.dateTime);

  String date(DateTime instant) => format(instant, TaipeiFormat.date);

  /// 時間字串 [raw]：空字串回空字串，解析不了原樣回傳（不附標籤，它不是臺北時間）。
  String formatIso(String raw, [TaipeiFormat? format]) {
    final text = plainIso(raw, format);
    return text == null ? raw : labeled(text);
  }

  /// 同 [plainSeparator]，必要時附標籤。
  String separator(DateTime time, {DateTime? now}) =>
      labeled(plainSeparator(time, now: now));

  /// 相對時間文字 [text]（由 [ago] 產生）；只有一天以上的臺北日期（月日、年月日）附標籤，「N 分鐘前」這類經過時間不附。
  String agoText(TimeAgo ago, String text) =>
      ago is TimeAgoMonthDay || ago is TimeAgoFullDate ? labeled(text) : text;

  /// [time] 的相對時間（[taipeiTimeAgo]）；文字由 [textOf] 從 App 的 ARB 組出，日期形時附標籤。
  String timeAgo(
    DateTime time,
    String Function(TimeAgo ago) textOf, {
    DateTime? now,
  }) {
    final ago = taipeiTimeAgo(time, now: now);
    return agoText(ago, textOf(ago));
  }
}

/// 日期時間選擇器的起始值：已有值用它，沒有值是 [now] 起算一天後。回傳值的欄位是臺北牆上時間，只餵給選擇器；
/// 選好的欄位用 [taipeiInstantOfPicked] 換回時間點。不要拿它顯示（顯示走 [TaipeiDisplay]）。
DateTime taipeiPickerInitial(DateTime? current, DateTime now) =>
    (current ?? now.toUtc().add(const Duration(days: 1)))._taipeiWall;

/// 選擇器選出的臺北牆上日期與時間對應的時間點（UTC）。
DateTime taipeiInstantOfPicked(
  int year,
  int month,
  int day,
  int hour,
  int minute,
) => DateTime.utc(year, month, day, hour, minute).subtract(_taipeiUtcOffset);
