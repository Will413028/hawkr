import 'dart:async';

import 'package:hawkr_seller/features/settings/data/tgos_address_service.dart';
import 'package:hawkr_seller/features/settings/presentation/shipping_settings/widgets/address_autocomplete_field.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('shipping address ignores an older query that finishes last', (
    tester,
  ) async {
    final service = _ControlledTgosService();
    final controller = TextEditingController();
    addTearDown(controller.dispose);

    await tester.pumpWidget(
      _testApp(
        service,
        AddressAutocompleteField(
          controller: controller,
          county: '臺北市',
          district: '中正區',
        ),
      ),
    );

    await tester.enterText(find.byType(TextField), '舊路名');
    await tester.pump(const Duration(milliseconds: 401));
    expect(service.requests, ['舊路名']);

    await tester.enterText(find.byType(TextField), '新路名');
    await tester.pump(const Duration(milliseconds: 401));
    expect(service.requests, ['舊路名', '新路名']);

    service.complete('新路名', [_suggestion('新結果')]);
    await tester.pump();
    expect(find.text('新結果'), findsOneWidget);

    service.complete('舊路名', [_suggestion('舊結果')]);
    await tester.pump();
    expect(find.text('新結果'), findsOneWidget);
    expect(find.text('舊結果'), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets('district change and dispose invalidate an in-flight request', (
    tester,
  ) async {
    final service = _ControlledTgosService();
    final controller = TextEditingController();
    addTearDown(controller.dispose);

    Widget field(String district) => AddressAutocompleteField(
      controller: controller,
      county: '臺北市',
      district: district,
    );

    await tester.pumpWidget(_testApp(service, field('中正區')));
    await tester.enterText(find.byType(TextField), '忠孝東路');
    await tester.pump(const Duration(milliseconds: 401));

    await tester.pumpWidget(_testApp(service, field('信義區')));
    service.complete('忠孝東路', [_suggestion('不應顯示')]);
    await tester.pump();
    expect(find.text('不應顯示'), findsNothing);

    await tester.enterText(find.byType(TextField), '松高路');
    await tester.pump(const Duration(milliseconds: 401));
    await tester.pumpWidget(_testApp(service, const SizedBox.shrink()));
    service.complete('松高路', [_suggestion('已 dispose')]);
    await tester.pump();

    expect(find.text('已 dispose'), findsNothing);
    expect(tester.takeException(), isNull);
  });
}

Widget _testApp(TgosAddressService service, Widget child) {
  return ProviderScope(
    overrides: [tgosAddressServiceProvider.overrideWithValue(service)],
    child: ScreenUtilInit(
      designSize: const Size(390, 844),
      builder: (_, __) => MaterialApp(home: Scaffold(body: child)),
    ),
  );
}

AddressSuggestion _suggestion(String detail) {
  return AddressSuggestion(
    fullAddress: '臺北市中正區$detail',
    county: '臺北市',
    district: '中正區',
    detail: detail,
    lat: 25,
    lng: 121,
  );
}

class _ControlledTgosService extends TgosAddressService {
  _ControlledTgosService() : super(Dio());

  final List<String> requests = [];
  final Map<String, Completer<List<AddressSuggestion>>> _pending = {};

  @override
  Future<List<AddressSuggestion>> getSuggestions({
    required String county,
    required String district,
    required String query,
    int limit = 8,
  }) {
    requests.add(query);
    return (_pending[query] ??= Completer<List<AddressSuggestion>>()).future;
  }

  void complete(String query, List<AddressSuggestion> suggestions) {
    _pending[query]!.complete(suggestions);
  }
}
