import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/core/networking/dio_provider.dart';
import 'package:hawkr_buyer/features/order/data/order_model.dart';
import 'package:hawkr_buyer/features/order/presentation/widgets/order_logistics_section.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

/// 訂單詳情頁的「物流追蹤」卡片。
///
/// 這一支守住三件事：
/// 1. **沒有物流單的訂單不渲染、也不打那支 API**——否則剛下單的訂單會看到
///    一張「查詢失敗」的紅卡，而那不是失敗
/// 2. 正規化狀態文案與**異常狀態**的呈現分得開（異常沿用訂單卡那一份清單）
/// 3. 查詢失敗有可重試的錯誤格，不會靜默空白
void main() {
  if (!F.isInitialized) F.init(Flavor.dev);

  testWidgets('沒有物流單時不渲染，也不打 API', (tester) async {
    final calls = <String>[];
    await _pump(
      tester,
      order: const Order(orderSn: 'O1', status: '10'),
      onRequest: calls.add,
      body: _trackBody(),
    );

    expect(find.byKey(const ValueKey('order-logistics-section')), findsNothing);
    // 🔴 對照組：下面那幾支同樣的 harness 會命中 `/logistics`，所以
    //    「零命中」在這裡是真的沒打，不是 harness 沒接上。
    expect(calls, isEmpty);
  });

  testWidgets('宅配：顯示正規化狀態、物流單號與更新時間', (tester) async {
    final calls = <String>[];
    await _pump(
      tester,
      order: const Order(
        orderSn: 'O2',
        status: '30',
        shipmentNo: 'SP001',
        shippingMethod: '3001',
        shippingStatus: '31',
      ),
      onRequest: calls.add,
      body: _trackBody(
        shippingMethod: '3001',
        status: '31',
        statusLabel: '配送中',
        shipTradeNo: 'DLV-abc',
      ),
    );

    expect(calls, ['/app/orders/O2/logistics']);
    expect(find.byKey(const ValueKey('order-logistics-section')), findsOne);
    expect(find.text('配送中'), findsOne);
    expect(
      find.byKey(const ValueKey('order-logistics-status-normal')),
      findsOne,
    );
    expect(find.text('DLV-abc'), findsOne);
  });

  testWidgets('異常狀態走紅色橫幅，並附上物流商原文', (tester) async {
    await _pump(
      tester,
      order: const Order(
        orderSn: 'O3',
        status: '35',
        shipmentNo: 'SP002',
        shippingMethod: '3004',
        shippingStatus: '932',
      ),
      body: _trackBody(
        shippingMethod: '3004',
        // 932 物流中心驗退：本波新增進 DeliveryStatusPresentation.abnormalValues。
        status: '932',
        statusLabel: '物流中心驗退',
        providerMessage: '外包裝破損，退回物流中心',
      ),
    );

    expect(
      find.byKey(const ValueKey('order-logistics-status-abnormal')),
      findsOne,
    );
    expect(
      find.byKey(const ValueKey('order-logistics-status-normal')),
      findsNothing,
    );
    expect(find.textContaining('外包裝破損'), findsOne);
  });

  testWidgets('正常狀態不重複顯示物流商原文（同一件事不講兩次）', (tester) async {
    // 鑑別力：若把 providerMessage 無條件畫出來，上一條與這一條不會同時通過。
    await _pump(
      tester,
      order: const Order(
        orderSn: 'O4',
        status: '30',
        shipmentNo: 'SP003',
        shippingMethod: '3001',
        shippingStatus: '31',
      ),
      body: _trackBody(
        shippingMethod: '3001',
        status: '31',
        statusLabel: '配送中',
        providerMessage: '配送中',
      ),
    );

    expect(find.textContaining('物流商回報'), findsNothing);
  });

  testWidgets('查詢失敗時顯示可重試的錯誤格，而不是空白', (tester) async {
    await _pump(
      tester,
      order: const Order(
        orderSn: 'O5',
        status: '30',
        shipmentNo: 'SP004',
        shippingStatus: '31',
      ),
      body: {'code': 4004, 'message': '此訂單尚未建立物流單'},
    );

    expect(find.byKey(const ValueKey('order-logistics-error')), findsOne);
    expect(find.textContaining('此訂單尚未建立物流單'), findsOne);
    expect(find.text('重試'), findsOne);
  });
}

Map<String, dynamic> _trackBody({
  String shippingMethod = '3001',
  String status = '31',
  String statusLabel = '配送中',
  String shipTradeNo = 'SHIP0001',
  String providerMessage = '',
}) => {
  'code': 0,
  'message': 'success',
  'data': {
    'orderNo': 'O1',
    'shipTradeNo': shipTradeNo,
    'shippingMethod': shippingMethod,
    'status': status,
    'statusLabel': statusLabel,
    'providerStatus': '',
    'providerMessage': providerMessage,
    'statusTime': '2026-09-12T04:30:00Z',
    'storeName': '',
    'recipientName': '測試買家',
    'createdAt': '2026-09-12T04:00:00Z',
    'cvsStoreCode': '',
    'allowedActions': <Object>[],
  },
};

Future<void> _pump(
  WidgetTester tester, {
  required Order order,
  required Map<String, dynamic> body,
  void Function(String path)? onRequest,
}) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = const Size(390, 900);
  addTearDown(tester.view.resetDevicePixelRatio);
  addTearDown(tester.view.resetPhysicalSize);

  // 🔴 用 interceptor 而不是 httpClientAdapter：`testWidgets` 跑在 fake async 區，
  //    adapter ＋ dio transformer 那條路徑在錯誤分支上不會被 `pump` 推進（實測會停在
  //    loading），而 interceptor 的 `handler.resolve` 就在同一批 microtask 內完成。
  final dio = Dio(BaseOptions(baseUrl: 'https://example.test'))
    ..interceptors.add(
      InterceptorsWrapper(
        onRequest: (options, handler) {
          onRequest?.call(options.path);
          handler.resolve(
            Response<Map<String, dynamic>>(
              requestOptions: options,
              statusCode: 200,
              data: body,
            ),
          );
        },
      ),
    );
  addTearDown(dio.close);

  await tester.pumpWidget(
    ProviderScope(
      overrides: [dioProvider.overrideWithValue(dio)],
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, _) => MaterialApp(
          home: Scaffold(
            body: SingleChildScrollView(
              child: OrderLogisticsSection(order: order),
            ),
          ),
        ),
      ),
    ),
  );
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 50));
  await tester.pump(const Duration(milliseconds: 50));
}
