import 'package:hawkr_seller/core/exceptions/app_exception.dart';
import 'package:hawkr_seller/features/notification/data/notification_repository.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

/// Gateway envelope 的訊息欄名只有 `message`（backend#2198 起 `msg` 已從 wire 移除，
/// backend#2231 收斂 App 側殘留）。這裡過去只讀 `msg`，於是任何非零業務碼都掉回
/// 通用字串，賣家看不到後端給的真正原因。
void main() {
  test('非零業務碼透傳 envelope 的 message', () {
    final repository = NotificationRepository(
      _dioResolving({'code': 7102, 'message': '通知服務暫時無法使用'}),
    );

    return expectLater(
      repository.getNotifications(),
      throwsA(
        isA<AppException>().having(
          (error) => error.message,
          'message',
          '通知服務暫時無法使用',
        ),
      ),
    );
  });

  test('舊 msg 欄名不再被讀取（沒有相容 fallback）', () {
    // 鑑別力：上一條若靠 `data['msg'] ?? data['message']` 通過，這條會失敗。
    final repository = NotificationRepository(
      _dioResolving({'code': 7102, 'msg': '通知服務暫時無法使用'}),
    );

    return expectLater(
      repository.getNotifications(),
      throwsA(
        isA<AppException>().having(
          (error) => error.message,
          'message',
          '獲取通知列表失敗',
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
