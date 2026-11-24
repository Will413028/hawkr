import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

/// 訂單與售後回應已不帶 ATM 欄位（B2 步驟 6）：退款一律原路退回，沒有買家要填的退款資料、虛擬帳號與匯款日期。
/// 讀這些欄位的程式只會永遠拿到預設值，畫面分支變成死路；加回任何一個就讓這支測試轉紅。
final _removedFields = <RegExp>[
  RegExp(r'\bneedRefundPage\b'),
  RegExp(r'\batm(BankCode|VirtualAccount|ExpireDate)\b'),
  RegExp(r'\bestimatedRefundDate\b'),
  RegExp(r'\bbankDate\b'),
];

String _stripComments(String source) => source
    .replaceAll(RegExp(r'/\*.*?\*/', dotAll: true), '')
    .split('\n')
    .map((line) => line.replaceFirst(RegExp(r'(?<!:)//.*$'), ''))
    .join('\n');

void main() {
  test('lib/ 不讀已刪除的 ATM 欄位', () {
    final offenders = <String>[];
    var scanned = 0;
    for (final entity in Directory('lib').listSync(recursive: true)) {
      if (entity is! File || !entity.path.endsWith('.dart')) continue;
      if (entity.path.endsWith('.g.dart') ||
          entity.path.endsWith('.freezed.dart')) {
        continue;
      }
      scanned++;
      final source = _stripComments(entity.readAsStringSync());
      for (final pattern in _removedFields) {
        if (pattern.hasMatch(source)) {
          offenders.add('${entity.path} → ${pattern.pattern}');
        }
      }
    }
    expect(scanned, greaterThan(0), reason: '一個 .dart 都沒掃到，測試本身失效');
    expect(offenders, isEmpty, reason: offenders.join('\n'));
  });
}
