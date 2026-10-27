import 'dart:convert';
import 'dart:io';

import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:pay/pay.dart';

import '../../config/flavor.dart';

/// Apple Pay token 取得服務（App 只發 iOS）
///
/// 封裝 `pay` 套件，負責：
/// 1. 初始化 Pay client 與檢查裝置支援度
/// 2. 彈出原生付款面板，取得授權後的 payment token
/// 3. 將 token 解析為原始 JSON 字串供後端 API 使用
///
/// **後端契約**（對齊 PayUni 金流官方 API 總覽 §4.6 / §5.6 與
/// `services/payment-service` PaymentClient.java）：
/// caller 一律送原始 paymentTokenObject JSON 字串，base64 編碼由 backend
/// 統一處理。caller 端再做 base64 會造成雙重編碼，PayUni 解碼失敗回
/// `APPLE01009「驗簽解密未提供此版本處理」`。
///
/// 使用方式：
/// ```dart
/// final service = PayTokenService();
/// await service.init();
///
/// if (service.isAvailable) {
///   final rawToken = await service.requestToken(
///     amount: '100',
///     label: 'Hawkr 訂單',
///   );
///   // 送往後端 POST /app/payments { applePayToken: rawToken }
/// }
/// ```
class PayTokenService {
  Pay? _payClient;
  bool _isApplePayAvailable = false;

  /// 裝置是否支援 Apple Pay
  bool get isAvailable => _isApplePayAvailable;

  bool get isApplePayAvailable => _isApplePayAvailable;

  /// 初始化 Pay client 並檢查裝置可用性；非 iOS 不做事。
  Future<void> init() async {
    if (!Platform.isIOS) return;

    debugPrint(
      '[PayTokenService] init Apple Pay '
      'flavor=${F.appFlavor.name} '
      'merchantId=${F.appFlavor.applePayMerchantId}',
    );
    _payClient = Pay({
      PayProvider.apple_pay: PaymentConfiguration.fromJsonString(
        _applePayConfig(),
      ),
    });
    _isApplePayAvailable = await _payClient!.userCanPay(PayProvider.apple_pay);
    debugPrint('[PayTokenService] Apple Pay available=$_isApplePayAvailable');
  }

  /// 彈出原生付款面板，取得原始 paymentTokenObject JSON 字串
  ///
  /// [amount] 付款金額（字串，如 '350'）
  /// [label]  顯示在付款面板的商品說明
  ///
  /// 回傳原始 token JSON 字串，可直接作為 `applePayToken` 傳給後端。base64 由 backend 統一處理（見 class
  /// 註解的「後端契約」說明）。
  ///
  /// 使用者取消時拋出 [PaymentCancelledException]；其他錯誤原樣拋出。
  Future<String> requestToken({
    required String amount,
    String label = 'Hawkr',
  }) async {
    assert(_payClient != null, 'Must call init() before requestToken()');

    const provider = PayProvider.apple_pay;

    final paymentItems = [
      PaymentItem(
        label: label,
        amount: amount,
        status: PaymentItemStatus.final_price,
      ),
    ];

    try {
      debugPrint(
        '[PayTokenService] showPaymentSelector '
        'provider=$provider amount=$amount',
      );
      final result = await _payClient!.showPaymentSelector(
        provider,
        paymentItems,
      );
      _debugPaymentResult(result);
      return _extractRawToken(result);
    } on PlatformException catch (e, stackTrace) {
      debugPrint(
        '[PayTokenService] PlatformException '
        'code=${e.code} message=${e.message} details=${e.details}',
      );
      debugPrintStack(stackTrace: stackTrace);
      if (_isCancelMessage('${e.code} ${e.message}')) {
        throw PaymentCancelledException();
      }
      rethrow;
    } catch (e) {
      final msg = e.toString();
      debugPrint('[PayTokenService] requestToken failed: $msg');
      if (_isCancelMessage(msg)) {
        throw PaymentCancelledException();
      }
      rethrow;
    }
  }

  /// 釋放資源（目前 pay 套件無需特殊清理，預留擴充）
  void dispose() {
    _payClient = null;
    _isApplePayAvailable = false;
  }

  // ---------------------------------------------------------------------------
  // Token 解析
  // ---------------------------------------------------------------------------

  String _extractRawToken(Map<String, dynamic> result) {
    final transactionIdentifier = result['transactionIdentifier']?.toString();
    // pay_ios 回傳 { "token": "<PKPaymentToken.paymentData JSON>", ... }。
    final tokenString = _firstNonEmptyString([
      result['token'],
      result['paymentData'],
    ]);

    if (tokenString == null) {
      debugPrint(
        '[PayTokenService] ERROR: payment token is empty; '
        'result keys=${result.keys.join(',')} '
        'transactionIdentifier=$transactionIdentifier',
      );
      throw PaymentTokenUnavailableException(
        _emptyTokenMessage(transactionIdentifier),
      );
    }

    debugPrint(
      '[PayTokenService] token extracted (${tokenString.length} chars)',
    );
    // PayUni 期望 caller 送原始 paymentTokenObject JSON 字串；base64 由
    // backend (PaymentClient.java) 統一處理。caller 端再 base64 會造成
    // 雙重編碼，PayUni 解碼失敗。
    return tokenString;
  }

  void _debugPaymentResult(Map<String, dynamic> result) {
    final token = result['token'];
    final transactionIdentifier = result['transactionIdentifier'];
    final paymentMethod = result['paymentMethod'];
    final paymentMethodText = paymentMethod is Map
        ? 'network=${paymentMethod['network']} '
              'type=${paymentMethod['type']} '
              'displayName=${paymentMethod['displayName']}'
        : 'type=${paymentMethod.runtimeType}';
    debugPrint(
      '[PayTokenService] payment result '
      'keys=${result.keys.join(',')} '
      'tokenLength=${token is String ? token.length : 'n/a'} '
      'transactionIdentifier=${transactionIdentifier is String ? transactionIdentifier : 'n/a'} '
      'paymentMethod=$paymentMethodText',
    );
  }

  String _emptyTokenMessage(String? transactionIdentifier) {
    if (transactionIdentifier == 'Simulated Identifier') {
      return 'Apple Pay 目前回傳的是模擬付款，未產生可送往 PayUni 的付款憑證。請用實機 Wallet Sandbox 測試卡，或改用信用卡/ATM。';
    }
    return 'Apple Pay 未回傳付款憑證，請確認 Wallet 卡片支援線上 Apple Pay 交易，或改用其他付款方式。';
  }

  bool _isCancelMessage(String message) {
    final normalized = message.toLowerCase();
    return normalized.contains('canceled') ||
        normalized.contains('cancelled') ||
        normalized.contains('cancel');
  }

  String? _firstNonEmptyString(Iterable<dynamic> values) {
    for (final value in values) {
      if (value is String && value.trim().isNotEmpty) {
        return value.trim();
      }
    }
    return null;
  }

  // ---------------------------------------------------------------------------
  // Pay configuration JSON builders
  // ---------------------------------------------------------------------------

  static String _applePayConfig() => jsonEncode({
    'provider': 'apple_pay',
    'data': {
      'merchantIdentifier': F.appFlavor.applePayMerchantId,
      'displayName': 'Hawkr',
      'merchantCapabilities': ['3DS', 'debit', 'credit'],
      // PayUni Taiwan Apple Pay test should stay on locally supported
      // networks. AMEX sandbox cards are UK/US-region only and can produce
      // misleading provider rejections in the current PayUni channel.
      'supportedNetworks': ['visa', 'masterCard', 'jcb'],
      'countryCode': 'TW',
      'currencyCode': 'TWD',
      'requiredBillingContactFields': <String>[],
      'requiredShippingContactFields': <String>[],
    },
  });
}

/// 使用者取消付款時拋出
class PaymentCancelledException implements Exception {
  @override
  String toString() => '使用者取消支付';
}

/// 原生付款面板完成授權，但沒有回傳可送後端的付款憑證。
class PaymentTokenUnavailableException implements Exception {
  const PaymentTokenUnavailableException([this.message = '未取得付款憑證']);

  final String message;

  @override
  String toString() => message;
}
