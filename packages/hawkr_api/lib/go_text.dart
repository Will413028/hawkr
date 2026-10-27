/// 與伺服器（Go）一致的文字規則。
///
/// TypeScript 端的孿生是 `packages/hawkr-api-ts/src/text/server-text.ts`，兩邊的公開名稱與行為一致，改一邊要同步另一邊。
library;

/// Go `unicode.IsSpace` 認定的空白：Latin-1 的 `\t \n \v \f \r`、空白、U+0085、U+00A0，加上 Unicode White_Space 的
/// U+1680、U+2000–U+200A、U+2028、U+2029、U+202F、U+205F、U+3000。
///
/// Dart 的 `String.trim` 不一樣（例如多修剪 U+FEFF）；伺服器用 Go 的 `strings.TrimSpace`，所以字數與送出的文字都用這份。
bool _isGoSpace(int unit) =>
    (unit >= 0x09 && unit <= 0x0D) ||
    unit == 0x20 ||
    unit == 0x85 ||
    unit == 0xA0 ||
    unit == 0x1680 ||
    (unit >= 0x2000 && unit <= 0x200A) ||
    unit == 0x2028 ||
    unit == 0x2029 ||
    unit == 0x202F ||
    unit == 0x205F ||
    unit == 0x3000;

/// 去掉頭尾的空白，和 Go `strings.TrimSpace` 相同（空白都在 BMP，逐 UTF-16 單位比對即可）。
String trimGoSpace(String text) {
  var start = 0;
  var end = text.length;
  while (start < end && _isGoSpace(text.codeUnitAt(start))) {
    start++;
  }
  while (end > start && _isGoSpace(text.codeUnitAt(end - 1))) {
    end--;
  }
  return text.substring(start, end);
}

/// 伺服器的字數規則：先 trim（[trimGoSpace]），再以 rune（Unicode code point）計，不是 UTF-16 單位也不是 grapheme。
int goTrimmedRuneCount(String text) => trimGoSpace(text).runes.length;
