import 'dart:convert';
import 'dart:io';

import 'package:connectrpc/connect.dart' show Code;
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pbenum.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:intl/intl.dart';
import 'package:protobuf/protobuf.dart' show ProtobufEnum;
import 'package:test/test.dart';

/// 生成的 MeasureKind 不認得的值（舊 client 收到新 server 才新增的成員）：protobuf 的 `valueOf(999)` 與解碼都不會造出它，
/// 所以用 `implements` 造一個 value 為 [v] 的實例，直接測 `uiText` 的查表。
class _UnknownMeasureKind extends ProtobufEnum implements MeasureKind {
  const _UnknownMeasureKind(int v) : super(v, 'UNKNOWN');
}

void main() {
  test('UiLocale 與 catalog/config.json 的 locales 一一對應', () {
    final config =
        jsonDecode(
              File(
                '../../backend/internal/platform/uitext/catalog/config.json',
              ).readAsStringSync(),
            )
            as Map<String, Object?>;
    expect(UiLocale.values.map((l) => l.tag).toList(), config['locales']);
  });

  test('名詞：UNSPECIFIED 與不認得的值是 null，「未知」由呼叫端明確用 commonUnknown', () {
    expect(MeasureKind.MEASURE_KIND_WARNING.uiText(UiLocale.zhTW), '警告');
    expect(MeasureKind.MEASURE_KIND_UNSPECIFIED.uiText(UiLocale.zhTW), isNull);
    expect(MeasureKind.MEASURE_KIND_UNSPECIFIED.uiText(UiLocale.en), isNull);
    expect(const _UnknownMeasureKind(999).uiText(UiLocale.zhTW), isNull);
    expect(const _UnknownMeasureKind(-1).uiText(UiLocale.en), isNull);
    expect(commonUnknown(UiLocale.zhTW), '未知');
    expect(commonUnknown(UiLocale.en), '未知');
  });

  group('resolveUiLocale', () {
    test('中文與沒有語系都是 zh-TW', () {
      expect(resolveUiLocale(null, [UiLocale.zhTW]), UiLocale.zhTW);
      expect(resolveUiLocale('zh', [UiLocale.zhTW]), UiLocale.zhTW);
      expect(
        resolveUiLocale('zh', [UiLocale.zhTW, UiLocale.en]),
        UiLocale.zhTW,
      );
    });

    test('其他語言在 en 未發佈時仍是 zh-TW，已發佈時是 en', () {
      expect(resolveUiLocale('en', [UiLocale.zhTW]), UiLocale.zhTW);
      expect(resolveUiLocale('ja', [UiLocale.zhTW]), UiLocale.zhTW);
      expect(resolveUiLocale('en', [UiLocale.zhTW, UiLocale.en]), UiLocale.en);
      expect(resolveUiLocale('ja', [UiLocale.zhTW, UiLocale.en]), UiLocale.en);
    });
  });

  test('select 參數：kind 不在分支裡走 other', () {
    expect(
      MeasureState.MEASURE_STATE_ACTIVE.uiText(
        UiLocale.zhTW,
        kind: MeasureKind.MEASURE_KIND_TERMINATION,
      ),
      '合作已終止',
    );
    expect(
      MeasureState.MEASURE_STATE_ACTIVE.uiText(
        UiLocale.zhTW,
        kind: MeasureKind.MEASURE_KIND_UNSPECIFIED,
      ),
      '停權中',
    );
  });

  test('名詞參數不認得時整個函式沒有文字', () {
    expect(
      sellerMeasureTitleRisk(
        UiLocale.zhTW,
        ground: RiskGround.RISK_GROUND_FRAUD,
      ),
      '風險：詐欺',
    );
    expect(
      sellerMeasureTitleRisk(
        UiLocale.zhTW,
        ground: RiskGround.RISK_GROUND_UNSPECIFIED,
      ),
      isNull,
    );
  });

  test('變體：omit 的值回 null', () {
    expect(
      KYCFlowStatus.KYC_FLOW_STATUS_APPROVED.action(UiLocale.zhTW),
      isNull,
    );
    expect(
      KYCFlowStatus.KYC_FLOW_STATUS_SUBMITTED.action(UiLocale.zhTW),
      '查看進度',
    );
    expect(
      MeasureRequestState.MEASURE_REQUEST_STATE_CLOSED.withKind(
        UiLocale.zhTW,
        kind: MeasureRequestKind.MEASURE_REQUEST_KIND_APPEAL,
      ),
      isNull,
    );
  });

  test('Connect 預設文字：canceled 沒有文字', () {
    expect(commonErrorText(Code.canceled, UiLocale.zhTW), isNull);
    expect(commonErrorText(Code.invalidArgument, UiLocale.zhTW), '參數錯誤');
    expect(commonErrorText(Code.internal, UiLocale.en), '系統錯誤');
    for (final code in Code.values.where((c) => c != Code.canceled)) {
      expect(commonErrorText(code, UiLocale.zhTW), isNotNull, reason: '$code');
    }
  });

  // golden 沒有 en 的 plural 分支（en 目錄是空的）；這裡確認生成碼依賴的 Intl.pluralLogic 行為。
  // zh 的 plural 只有 other（生成器擋下其他分支），所以生成碼對 zh 不呼叫 pluralLogic（intl 對 zh 仍會回 one，不能依賴它）。
  test('Intl.pluralLogic：en 的 1 是 one', () {
    String en(int n) =>
        Intl.pluralLogic(n, one: 'one', other: 'other', locale: 'en');
    expect([en(0), en(1), en(2), en(1234)], ['other', 'one', 'other', 'other']);
  });
}
