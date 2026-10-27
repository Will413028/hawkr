import 'package:hawkr_seller/core/session/my_store.dart';
import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:hawkr_seller/features/settings/data/shipping_settings_repository.dart';
import 'package:hawkr_seller/features/settings/data/tgos_address_service.dart';
import 'package:hawkr_seller/features/settings/presentation/shipping_settings/shipping_settings_screen.dart';
import 'package:hawkr_twaddr/hawkr_twaddr.dart' as twaddr;
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../support/my_store_fixtures.dart';

String _code(String county, String town) =>
    twaddr.townByName(county, town)!.code;

MyStore _store({
  String contactName = '王店長',
  String contactMobile = '0912345678',
  String contactPhone = '',
  String? townCode,
  String addressDetail = '中華路一段 1 號',
}) => MyStore(
  myStoreResponse(
    contactName: contactName,
    contactMobile: contactMobile,
    contactPhone: contactPhone,
    townCode: townCode ?? _code('臺東縣', '臺東市'),
    addressDetail: addressDetail,
  ),
);

Map<String, dynamic> _homeRow({
  String status = '4304',
  String name = '',
  String phone = '',
  String city = '',
  String district = '',
  String detail = '',
}) => {
  'shippingType': '3652',
  'payuniSubmitStatus': status,
  'senderName': name,
  'senderPhone': phone,
  'senderCity': city,
  'senderDistrict': district,
  'senderAddressDetail': detail,
};

Map<String, dynamic> _cvsRow(
  String goods, {
  String name = '',
  String phone = '',
  String status = '4303',
}) => {
  'shippingType': '3651',
  'goodsType': goods,
  'enabled': 'true',
  'payuniSubmitStatus': status,
  'onboardingId': 'ob-$goods',
  'senderName': name,
  'senderPhone': phone,
  'returnStoreCode': '123456',
  'returnStoreName': '測試門市',
};

class _FakeRepository implements ShippingSettingsRepository {
  _FakeRepository(this.rows);

  final List<Map<String, dynamic>> rows;
  final List<Map<String, dynamic>> homeBodies = [];

  @override
  Future<ShippingSettingsBootstrap> loadBootstrap() async =>
      ShippingSettingsBootstrap(onboardings: rows);

  @override
  Future<ShippingCommandResult> activateHomeDelivery(
    Map<String, dynamic> body,
  ) async {
    homeBodies.add(Map<String, dynamic>.of(body));
    return const ShippingCommandResult.failure('stop');
  }

  @override
  Future<Map<String, dynamic>?> loadStoreMapSession() async => null;

  @override
  Future<Map<String, dynamic>> sendMutation(
    String path,
    Map<String, dynamic> body,
  ) async => const {'code': 0};
}

class _ValidTgos extends TgosAddressService {
  _ValidTgos() : super(Dio());

  @override
  Future<AddressValidation> validate(String address) async =>
      const AddressValidation(valid: true);
}

class _StoreHolder extends Notifier<MyStore?> {
  @override
  MyStore? build() => null;

  void set(MyStore? store) => state = store;
}

final _lateStore = NotifierProvider<_StoreHolder, MyStore?>(_StoreHolder.new);

Future<_FakeRepository> _pump(
  WidgetTester tester, {
  required List<Map<String, dynamic>> rows,
  MyStore? store,
  bool lateStore = false,
  bool settle = true,
}) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = const Size(900, 4000);
  addTearDown(tester.view.resetDevicePixelRatio);
  addTearDown(tester.view.resetPhysicalSize);
  final repository = _FakeRepository(rows);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        shippingSettingsRepositoryProvider.overrideWithValue(repository),
        if (lateStore)
          myStoreSnapshotProvider.overrideWith((ref) => ref.watch(_lateStore))
        else
          myStoreSnapshotProvider.overrideWithValue(store),
        tgosAddressServiceProvider.overrideWithValue(_ValidTgos()),
      ],
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, _) => MaterialApp(
          theme: AppTheme.light,
          home: const ShippingSettingsScreen(),
        ),
      ),
    ),
  );
  if (settle) {
    await tester.pumpAndSettle();
  } else {
    // 處理中的按鈕有不會結束的 spinner。
    await tester.pump();
    await tester.pump(const Duration(seconds: 1));
  }
  return repository;
}

Finder _inCard(String key) => find.descendant(
  of: find.byKey(ValueKey(key)),
  matching: find.byType(TextField),
);

/// 黑貓卡的輸入框依序是姓名、手機、路名與門牌；7-11 卡是姓名、手機。
Finder _field(String label) => switch (label) {
  '聯絡人姓名' => _inCard('shipping-taqbin-card').at(0),
  '手機號碼' => _inCard('shipping-taqbin-card').at(1),
  '路名與門牌' => _inCard('shipping-taqbin-card').at(2),
  '退貨人姓名' => _inCard('shipping-cvs-card').at(0),
  '退貨手機' => _inCard('shipping-cvs-card').at(1),
  _ => throw ArgumentError(label),
};

String _text(WidgetTester tester, String label) =>
    tester.widget<TextField>(_field(label)).controller!.text;

bool _enabled(WidgetTester tester, String label) =>
    tester.widget<TextField>(_field(label)).enabled ?? true;

void main() {
  testWidgets('沒有紀錄時黑貓與 7-11 都由店家預填', (tester) async {
    await _pump(tester, rows: const [], store: _store());

    expect(_text(tester, '聯絡人姓名'), '王店長');
    expect(_text(tester, '手機號碼'), '0912345678');
    expect(_text(tester, '退貨人姓名'), '王店長');
    expect(_text(tester, '退貨手機'), '0912345678');
    expect(find.text('台東縣'), findsOneWidget);
    expect(find.text('台東市'), findsOneWidget);
    expect(_text(tester, '路名與門牌'), '中華路一段 1 號');
  });

  testWidgets('店家快照晚到時仍預填，且不蓋掉已輸入的欄位', (tester) async {
    await _pump(tester, rows: const [], lateStore: true);
    expect(_text(tester, '聯絡人姓名'), '');
    await tester.enterText(_field('手機號碼'), '0900000000');

    final container = ProviderScope.containerOf(
      tester.element(find.byType(ShippingSettingsScreen)),
    );
    container.read(_lateStore.notifier).set(_store());
    await tester.pumpAndSettle();

    expect(_text(tester, '聯絡人姓名'), '王店長');
    expect(_text(tester, '手機號碼'), '0900000000');
    expect(_text(tester, '退貨人姓名'), '王店長');
    expect(_text(tester, '路名與門牌'), '中華路一段 1 號');
  });

  testWidgets('多筆黑貓紀錄時寄件人、鎖定與狀態都取第一筆', (tester) async {
    await _pump(
      tester,
      rows: [
        _homeRow(status: '4304', name: '第一筆', phone: '0911111111'),
        _homeRow(status: '4303', name: '第二筆', phone: '0922222222'),
      ],
      store: _store(),
    );
    expect(_text(tester, '聯絡人姓名'), '第一筆');
    expect(_enabled(tester, '聯絡人姓名'), isTrue);
    expect(find.text('重新嘗試'), findsOneWidget);
  });

  testWidgets('多筆黑貓紀錄時第一筆已核准就鎖住並顯示它的寄件人', (tester) async {
    await _pump(
      tester,
      rows: [
        _homeRow(status: '4303', name: '第一筆', phone: '0911111111'),
        _homeRow(status: '4304', name: '第二筆', phone: '0922222222'),
      ],
      store: _store(),
    );
    expect(_text(tester, '聯絡人姓名'), '第一筆');
    expect(_enabled(tester, '聯絡人姓名'), isFalse);
  });

  testWidgets('4304 有寄件人姓名的 7-11 紀錄不算已開通', (tester) async {
    await _pump(
      tester,
      rows: [_cvsRow('3201', name: '失敗人', phone: '0911111111', status: '4304')],
      store: _store(),
    );
    expect(_text(tester, '退貨人姓名'), '失敗人');
    expect(find.text('開通7-11物流'), findsOneWidget);
    expect(find.text('更新7-11物流'), findsNothing);
  });

  testWidgets('4303 且有 onboardingId 的 7-11 紀錄算已開通', (tester) async {
    await _pump(
      tester,
      rows: [_cvsRow('3201', name: '成功人', phone: '0911111111')],
      store: _store(),
    );
    expect(find.text('開通7-11物流'), findsNothing);
    expect(find.text('更新7-11物流'), findsNothing);
    expect(find.text('已開通，可調整配送溫層'), findsOneWidget);
  });

  testWidgets('7-11 紀錄 4302 顯示處理中、鎖寄件人且不能送出', (tester) async {
    await _pump(
      tester,
      rows: [_cvsRow('3201', name: '處理人', phone: '0911111111', status: '4302')],
      store: _store(),
      settle: false,
    );
    final spinner = find.byType(CircularProgressIndicator);
    expect(spinner, findsOneWidget);
    expect(
      tester
          .widget<FilledButton>(
            find.ancestor(of: spinner, matching: find.byType(FilledButton)),
          )
          .onPressed,
      isNull,
    );
    expect(find.text('開通7-11物流'), findsNothing);
    expect(_enabled(tester, '退貨人姓名'), isFalse);
    expect(_enabled(tester, '退貨手機'), isFalse);
  });

  testWidgets('7-11 紀錄 4303 鎖寄件人欄位', (tester) async {
    await _pump(
      tester,
      rows: [_cvsRow('3201', name: '成功人', phone: '0911111111')],
      store: _store(),
    );
    expect(_enabled(tester, '退貨人姓名'), isFalse);
    expect(_enabled(tester, '退貨手機'), isFalse);
  });

  testWidgets('7-11 紀錄 4304 與沒有紀錄時寄件人欄位可編輯', (tester) async {
    await _pump(
      tester,
      rows: [_cvsRow('3201', name: '失敗人', status: '4304')],
      store: _store(),
    );
    expect(_enabled(tester, '退貨人姓名'), isTrue);
    expect(_enabled(tester, '退貨手機'), isTrue);
    expect(find.byType(CircularProgressIndicator), findsNothing);
  });

  testWidgets('7-11 常溫紀錄沒有寄件人值時用店家，不翻冷凍紀錄', (tester) async {
    await _pump(
      tester,
      rows: [
        _cvsRow('3201'),
        _cvsRow('3203', name: '冷凍人', phone: '0922222222'),
      ],
      store: _store(),
    );
    expect(_text(tester, '退貨人姓名'), '王店長');
  });

  testWidgets('店家缺的欄位留空，不退回商店電話', (tester) async {
    await _pump(
      tester,
      rows: const [],
      store: _store(
        contactName: '',
        contactMobile: '',
        contactPhone: '02-12345678',
      ),
    );

    expect(find.text('02-12345678'), findsNothing);
    expect(_text(tester, '聯絡人姓名'), '');
    expect(_text(tester, '手機號碼'), '');
    expect(_text(tester, '退貨人姓名'), '');
    expect(_text(tester, '退貨手機'), '');
  });

  testWidgets('失敗的黑貓紀錄照用且不鎖定，店家的值不混進來', (tester) async {
    await _pump(
      tester,
      rows: [
        _homeRow(
          status: '4304',
          name: '舊寄件人',
          phone: '0988000111',
          city: '高雄市',
          district: '前鎮區',
          detail: '成功路 2 號',
        ),
      ],
      store: _store(),
    );

    expect(_text(tester, '聯絡人姓名'), '舊寄件人');
    expect(_text(tester, '手機號碼'), '0988000111');
    expect(find.text('高雄市'), findsOneWidget);
    expect(find.text('前鎮區'), findsOneWidget);
    expect(find.text('台東市'), findsNothing);
    expect(_text(tester, '路名與門牌'), '成功路 2 號');
    expect(_enabled(tester, '聯絡人姓名'), isTrue);
    expect(find.text('重新嘗試'), findsOneWidget);
  });

  testWidgets('只有 4303 的黑貓紀錄鎖定；有寄件人姓名的 4304 不鎖', (tester) async {
    await _pump(
      tester,
      rows: [_homeRow(status: '4303', name: '已開通', phone: '0988000111')],
      store: _store(),
    );
    final locked = tester.widget<AbsorbPointer>(
      find
          .descendant(
            of: find.byKey(const ValueKey('shipping-taqbin-card')),
            matching: find.byType(AbsorbPointer),
          )
          .first,
    );
    expect(locked.absorbing, isTrue);
    expect(_enabled(tester, '聯絡人姓名'), isFalse);
    expect(find.text('開通黑貓物流'), findsNothing);
  });

  testWidgets('7-11 兩筆紀錄時常溫優先', (tester) async {
    await _pump(
      tester,
      rows: [
        _cvsRow('3203', name: '冷凍人', phone: '0922222222'),
        _cvsRow('3201', name: '常溫人', phone: '0911111111'),
      ],
      store: _store(),
    );

    expect(_text(tester, '退貨人姓名'), '常溫人');
    expect(_text(tester, '退貨手機'), '0911111111');
  });

  testWidgets('7-11 姓名依顯示長度即時提示', (tester) async {
    await _pump(
      tester,
      rows: const [],
      store: _store(contactName: '一二三四五六'),
    );

    expect(find.textContaining('退貨人姓名長度過長'), findsOneWidget);
    expect(find.textContaining('聯絡人姓名長度過長'), findsNothing);
  });

  testWidgets('黑貓姓名超過顯示長度 30 才提示', (tester) async {
    await _pump(
      tester,
      rows: const [],
      store: _store(contactName: '一二三四五六七八九十一二三四五'),
    );
    expect(find.textContaining('聯絡人姓名長度過長'), findsNothing);

    await tester.enterText(_field('聯絡人姓名'), '一二三四五六七八九十一二三四五六');
    await tester.pump();
    expect(find.textContaining('聯絡人姓名長度過長'), findsOneWidget);
  });

  Future<void> activateHome(WidgetTester tester) async {
    await tester.tap(find.text('冷凍').first);
    await tester.pump();
    await tester.tap(find.text('驗證地址'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('開通黑貓物流'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('確認開通'));
    await tester.pumpAndSettle();
  }

  for (final (county, town, postal) in [
    ('臺東縣', '臺東市', '950'),
    ('雲林縣', '臺西鄉', '638'),
    ('屏東縣', '霧臺鄉', '902'),
    ('臺北市', '中正區', '100'),
  ]) {
    testWidgets('送出的郵遞區號由所選的區推導：$county$town', (tester) async {
      final repository = await _pump(
        tester,
        rows: const [],
        store: _store(townCode: _code(county, town)),
      );
      await activateHome(tester);

      await tester.pump(const Duration(seconds: 3));
      expect(repository.homeBodies, hasLength(1));
      expect(repository.homeBodies.single['senderPostalCode'], postal);
      expect(repository.homeBodies.single['senderName'], '王店長');
      expect(repository.homeBodies.single['senderPhone'], '0912345678');
      expect(
        repository.homeBodies.single['senderCity'],
        county.replaceAll('臺', '台'),
      );
      expect(
        repository.homeBodies.single['senderDistrict'],
        town.replaceAll('臺', '台'),
      );
    });
  }

  testWidgets('區對不到代碼時擋下送出並提示重新選擇行政區', (tester) async {
    final repository = await _pump(
      tester,
      rows: [
        _homeRow(
          status: '4301',
          name: '舊寄件人',
          phone: '0988000111',
          city: '高雄市',
          district: '不存在區',
          detail: '成功路 2 號',
        ),
      ],
      store: _store(),
    );
    await activateHome(tester);

    expect(repository.homeBodies, isEmpty);
    expect(find.text('請重新選擇行政區'), findsOneWidget);
    await tester.pump(const Duration(seconds: 3));
  });
}
