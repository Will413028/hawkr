import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import '../../../core/networking/dio_provider.dart';
import '../../../core/exceptions/app_exception.dart';

part 'payment_repository.g.dart';

@riverpod
PaymentRepository paymentRepository(Ref ref) =>
    PaymentRepository(ref.watch(dioProvider));

class PaymentRepository {
  final Dio _dio;
  PaymentRepository(this._dio);

  /// POST /app/payments — 建立支付交易
  ///
  /// [paymentType] 支付方式，例如 'credit_card', 'atm', 'apple_pay'
  /// [orderNo] 訂單編號
  /// [userEmail] 用於電子收據（信用卡必填）
  /// [bankType] ATM 銀行代碼（ATM 付款時使用）
  /// [expireDays] ATM 繳費期限天數
  /// [applePayToken] Apple Pay token
  ///
  /// 回傳 data: { tradeNo, redirectUrl, uppFormData, paymentType, amount, orderId, status }
  Future<Map<String, dynamic>> createPayment({
    required String orderNo,
    required String paymentType,
    String? userEmail,
    String? bankType,
    String? expireDays,
    String? applePayToken,
  }) async {
    try {
      final normalizedApplePayToken = applePayToken?.trim();
      final body = <String, dynamic>{
        'orderNo': orderNo,
        'paymentType': paymentType,
        if (userEmail != null) 'userEmail': userEmail,
        if (bankType != null) 'bankType': bankType,
        if (expireDays != null) 'expireDays': expireDays,
        if (normalizedApplePayToken?.isNotEmpty ?? false)
          'applePayToken': normalizedApplePayToken,
      };
      final resp = await _dio.post<Map<String, dynamic>>(
        '/app/payments',
        data: body,
      );
      final respBody = resp.data!;
      if (respBody['code'] != 0) {
        throw ServerException((respBody['message'] ?? '付款建立失敗').toString());
      }
      return respBody['data'] as Map<String, dynamic>? ?? {};
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      final data = e.response?.data;
      if (data is Map) {
        final message = data['message'];
        if (message != null && message.toString().trim().isNotEmpty) {
          throw ServerException(message.toString());
        }
      }
      throw const NetworkException();
    }
  }

  /// GET /app/payments/{trade_no} — 查詢支付交易狀態
  Future<Map<String, dynamic>> getPaymentStatus(String tradeNo) async {
    try {
      final resp = await _dio.get<Map<String, dynamic>>(
        '/app/payments/$tradeNo',
      );
      final body = resp.data!;
      if (body['code'] != 0) {
        throw ServerException((body['message'] ?? '查詢失敗').toString());
      }
      return body['data'] as Map<String, dynamic>? ?? {};
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      throw const NetworkException();
    }
  }

  /// GET /app/payments/orders/{order_no}/history — 查詢訂單所有付款嘗試。
  ///
  /// 回傳含 CLOSED_BY_SWITCH / duplicate row，供訂單明細顯示完整付款歷程。
  Future<List<Map<String, dynamic>>> getPaymentHistory(String orderNo) async {
    try {
      final resp = await _dio.get<Map<String, dynamic>>(
        '/app/payments/orders/$orderNo/history',
        // 404「訂單無付款記錄」是正常空狀態，允許 200 + 404 不丟 DioException，
        // 避免 console 印 ERROR 噪音；下面再判斷 statusCode 處理。
        options: Options(
          validateStatus: (s) => s != null && (s == 200 || s == 404),
        ),
      );
      if (resp.statusCode == 404) return const [];
      final body = resp.data!;
      if (body['code'] != 0) {
        throw ServerException((body['message'] ?? '查詢付款歷程失敗').toString());
      }
      final data = body['data'];
      final records = data is Map ? data['records'] : data;
      if (records is List) {
        return records
            .whereType<Map>()
            .map((m) => Map<String, dynamic>.from(m))
            .toList(growable: false);
      }
      return const [];
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      // 404「訂單無付款記錄」是正常空狀態（剛建單還沒有 payment attempt），
      // 不是錯誤；直接回空 list 讓 caller 不渲染 card。
      if (e.response?.statusCode == 404) return const [];
      throw const NetworkException();
    }
  }
}
