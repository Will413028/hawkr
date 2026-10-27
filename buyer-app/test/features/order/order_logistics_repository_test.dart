import 'package:hawkr_buyer/core/exceptions/app_exception.dart';
import 'package:hawkr_buyer/features/order/data/logistics_track.dart';
import 'package:hawkr_buyer/features/order/data/order_repository.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

/// `OrderRepository.getOrderLogistics` 的 wire 契約。
///
/// 🔴 欄名逐欄抄自 `gateway/internal/handler/shipping.go` 的 `OrderLogisticsResp`：
/// `orderNo`／`shipTradeNo`／`shippingMethod`／`status`／`statusLabel`／
/// `providerStatus`／`providerMessage`／`statusTime`／`storeName`／
/// `recipientName`／`createdAt`／`cvsStoreCode`／`allowedActions`。
void main() {
  test('逐欄解出買家物流追蹤', () async {
    RequestOptions? captured;
    final repository = OrderRepository(
      _dioResolving({
        'code': 0,
        'message': 'success',
        'data': {
          'orderNo': 'O20260912001',
          'shipTradeNo': 'SHIP123456',
          'shippingMethod': '3001',
          'status': '930',
          'statusLabel': '已交運',
          'providerStatus': 'shipped',
          'providerMessage': '包裹已交運',
          'statusTime': '2026-09-12T04:30:00Z',
          'storeName': '',
          'recipientName': '測試買家',
          'createdAt': '2026-09-12T04:00:00Z',
          'cvsStoreCode': '',
          'allowedActions': [],
        },
      }, onRequest: (options) => captured = options),
    );

    final track = await repository.getOrderLogistics('O20260912001');

    expect(captured?.path, '/app/orders/O20260912001/logistics');
    expect(captured?.method, 'GET');
    expect(track.shipTradeNo, 'SHIP123456');
    expect(track.shippingMethod, '3001');
    expect(track.status, '930');
    expect(track.statusLabel, '已交運');
    expect(track.providerMessage, '包裹已交運');
    expect(track.recipientName, '測試買家');
  });

  test('超商單解出門市名稱與門市代號', () async {
    final repository = OrderRepository(
      _dioResolving({
        'code': 0,
        'data': {
          'orderNo': 'O1',
          'shippingMethod': '3002',
          'status': '32',
          'statusLabel': '待取貨',
          'storeName': '統一超商測試門市',
          // 🔴 `cvsStoreCode` 是 `store_id` 的正名（proto 已 reserved 舊欄）。
          //    讀錯名字的症狀是門市代號永遠空白，而沒有任何東西會報錯。
          'cvsStoreCode': '252698',
        },
      }),
    );

    final track = await repository.getOrderLogistics('O1');
    expect(track.storeName, '統一超商測試門市');
    expect(track.cvsStoreCode, '252698');
  });

  test('非零業務碼透傳 envelope 的 message', () {
    final repository = OrderRepository(
      _dioResolving({'code': 4004, 'message': '此訂單尚未建立物流單'}),
    );
    return expectLater(
      repository.getOrderLogistics('O1'),
      throwsA(
        isA<ServerException>().having(
          (error) => error.message,
          'message',
          '此訂單尚未建立物流單',
        ),
      ),
    );
  });

  group('normalizedStatusLabel 三層回退', () {
    test('後端給了 statusLabel 就用它（後端是權威）', () {
      const track = LogisticsTrack(
        orderNo: 'O1',
        shipTradeNo: '',
        shippingMethod: '3001',
        status: '930',
        statusLabel: '後端說了算',
        providerMessage: '',
        statusTime: '',
        storeName: '',
        cvsStoreCode: '',
        recipientName: '',
        createdAt: '',
      );
      expect(track.normalizedStatusLabel, '後端說了算');
    });

    test('後端漏填時回退到 App 的 CodeMeta 拷貝', () {
      const track = LogisticsTrack(
        orderNo: 'O1',
        shipTradeNo: '',
        shippingMethod: '3001',
        status: '930',
        statusLabel: '',
        providerMessage: '',
        statusTime: '',
        storeName: '',
        cvsStoreCode: '',
        recipientName: '',
        createdAt: '',
      );
      expect(track.normalizedStatusLabel, '已交運');
      expect(track.isKnownStatus, isTrue);
    });

    test('🔴 碼值不在 App 的值域內時原樣顯示，不猜一個最接近的文案', () {
      const track = LogisticsTrack(
        orderNo: 'O1',
        shipTradeNo: '',
        shippingMethod: '3001',
        status: '9999',
        statusLabel: '',
        providerMessage: '',
        statusTime: '',
        storeName: '',
        cvsStoreCode: '',
        recipientName: '',
        createdAt: '',
      );
      // App 的 CodeMeta 是拷貝，後端新增狀態時它一定比較舊。
      // 那時猜「配送中」等於對使用者陳述一件沒有發生的事。
      expect(track.normalizedStatusLabel, '狀態 9999');
      expect(track.isKnownStatus, isFalse);
    });
  });
}

Dio _dioResolving(
  Map<String, dynamic> body, {
  void Function(RequestOptions options)? onRequest,
}) {
  final dio = Dio(BaseOptions(baseUrl: 'https://example.invalid'));
  dio.interceptors.add(
    InterceptorsWrapper(
      onRequest: (options, handler) {
        onRequest?.call(options);
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
  return dio;
}
