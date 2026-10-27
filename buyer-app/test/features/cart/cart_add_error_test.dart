import 'package:hawkr_buyer/core/exceptions/app_exception.dart';
import 'package:hawkr_buyer/features/cart/data/cart_error_message.dart';
import 'package:hawkr_buyer/features/cart/data/cart_repository.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('maps cart error code 6006 to the own-product message', () async {
    final dio = Dio();
    dio.interceptors.add(
      InterceptorsWrapper(
        onRequest: (options, handler) {
          handler.reject(
            DioException(
              requestOptions: options,
              response: Response<Map<String, dynamic>>(
                requestOptions: options,
                statusCode: 409,
                data: const {
                  'message': '不可將自己上架的商品加入購物車',
                  'messageZh': '不可將自己上架的商品加入購物車',
                  'code': 6006,
                },
              ),
              type: DioExceptionType.badResponse,
            ),
          );
        },
      ),
    );

    await expectLater(
      CartRepository(dio).addToCart(123, 1),
      throwsA(
        isA<ServerException>().having(
          (error) => error.message,
          'message',
          cannotAddOwnProductToCartMessage,
        ),
      ),
    );
  });

  test('uses an AppException message and falls back for unknown errors', () {
    expect(
      addToCartErrorMessage(
        const ServerException(cannotAddOwnProductToCartMessage),
      ),
      cannotAddOwnProductToCartMessage,
    );
    expect(
      addToCartErrorMessage(Exception('raw transport detail')),
      '加入購物車失敗，請稍後再試',
    );
  });
}
