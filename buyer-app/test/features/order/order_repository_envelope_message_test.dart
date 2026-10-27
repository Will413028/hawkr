import 'package:hawkr_buyer/core/exceptions/app_exception.dart';
import 'package:hawkr_buyer/features/order/data/order_repository.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

/// Gateway envelope 的訊息欄名只有 `message`（backend#2198 起 `msg` 已從 wire 移除，
/// backend#2231 收斂 App 側殘留）。`getRefundOrders` 過去只讀 `msg`，
/// 於是任何非零業務碼都掉回通用字串，使用者看不到後端給的真正原因。
void main() {
  test('非零業務碼透傳 envelope 的 message', () {
    final repository = OrderRepository(
      _dioResolving({'code': 4091, 'message': '訂單已結案，無法申請售後'}),
    );

    return expectLater(
      repository.getRefundOrders(),
      throwsA(
        isA<ServerException>().having(
          (error) => error.message,
          'message',
          '訂單已結案，無法申請售後',
        ),
      ),
    );
  });

  test('舊 msg 欄名不再被讀取（沒有相容 fallback）', () {
    // 鑑別力：上一條若靠 `body['msg'] ?? body['message']` 通過，這條會失敗。
    final repository = OrderRepository(
      _dioResolving({'code': 4091, 'msg': '訂單已結案，無法申請售後'}),
    );

    return expectLater(
      repository.getRefundOrders(),
      throwsA(
        isA<ServerException>().having(
          (error) => error.message,
          'message',
          '查詢售後訂單失敗',
        ),
      ),
    );
  });
}

Dio _dioResolving(Map<String, dynamic> body) {
  final dio = Dio(BaseOptions(baseUrl: 'https://example.invalid'));
  dio.interceptors.add(
    InterceptorsWrapper(
      onRequest: (options, handler) {
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
