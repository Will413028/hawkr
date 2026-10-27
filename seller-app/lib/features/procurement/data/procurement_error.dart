import 'package:dio/dio.dart';

import '../../../core/codemeta/code_meta.dart';
import '../../../core/exceptions/app_exception.dart';

/// 賣家回價命令失敗後，畫面該怎麼收拾。
enum ProcurementRecovery {
  /// 使用者可以直接改欄位重送（同一次意圖沿用同一把冪等鍵）。
  editAndRetry,

  /// 手上的依據已經過期，必須先重新載入最新版本再決定要不要重送。
  ///
  /// ⛔ `STALE_SELLER_RESPONSE` 走這條：必須明確告知賣家報價已變動，
  /// 不得靜默改成送最新版。
  reloadRequired,

  /// 這條路已經走不通（已成交、已停止寫入…），只能離開。
  terminal,

  /// 需要換一把新的冪等鍵重下命令。
  newIdempotencyKey,
}

/// 錯誤碼 → 賣家看得懂的訊息與處置。
class ProcurementErrorPresentation {
  const ProcurementErrorPresentation({
    required this.code,
    required this.message,
    required this.recovery,
  });

  /// CodeMeta 上的錯誤碼。
  final String code;

  /// 直接顯示給賣家的訊息（已含「接下來要做什麼」）。
  final String message;

  final ProcurementRecovery recovery;

  /// 這個錯誤是否要求畫面重新載入最新版本後才准再送。
  bool get requiresReload => recovery == ProcurementRecovery.reloadRequired;
}

/// 十個 #2538 新碼 ＋ 四個沿用既有碼的對照表。
///
/// 鍵一律取自 [ProcurementErrorReason]（生成物 `code_meta.dart`），不手寫字面值，
/// 這樣生成物改名時是編譯失敗而不是靜默失配。
final Map<String, ProcurementErrorPresentation> kProcurementErrorPresentations =
    <String, ProcurementErrorPresentation>{
      for (final entry
          in <(ProcurementErrorReason, String, ProcurementRecovery)>[
            (
              ProcurementErrorReason.staleRfqSnapshot,
              '買家的需求內容已經更新，這張詢價的依據不再是你手上這份。請重新載入後再回價。',
              ProcurementRecovery.reloadRequired,
            ),
            (
              ProcurementErrorReason.stalePublication,
              '這則詢價已經更新過，你手上的版本不是最新的。請重新載入最新內容後再送出。',
              ProcurementRecovery.reloadRequired,
            ),
            (
              ProcurementErrorReason.staleSellerResponse,
              '你的報價已經變動——可能在另一台裝置改過價或已撤回，你手上這一版不是目前的版本。'
                  '請先重新載入看目前是哪一版，確認後再決定要不要改版。',
              ProcurementRecovery.reloadRequired,
            ),
            (
              ProcurementErrorReason.rfqResponseAlreadySubmitted,
              '這則詢價你已經送過一版正式回價了。要調整請走「改版」，不是重新送出。',
              ProcurementRecovery.reloadRequired,
            ),
            (
              ProcurementErrorReason.rfqResponseExpired,
              '這一版報價已經超過你設定的有效期限。請重新填一版並把有效期限往後設。',
              ProcurementRecovery.reloadRequired,
            ),
            (
              ProcurementErrorReason.rfqRecipientClosed,
              '這則詢價對你已經停止收件——你先前拒絕過，或它已經結案。',
              ProcurementRecovery.terminal,
            ),
            (
              ProcurementErrorReason.rfqCheckoutAlreadyActive,
              '買家已經對這則詢價進入成交流程，現在不能再改報價。',
              ProcurementRecovery.terminal,
            ),
            (
              ProcurementErrorReason.rfqAlreadyAwarded,
              '這則詢價已經成交，不再接受回價。',
              ProcurementRecovery.terminal,
            ),
            (
              ProcurementErrorReason.quoteAmountInconsistent,
              '金額對不起來：總金額必須等於貨款＋運費＋稅額。請檢查四個金額欄位。',
              ProcurementRecovery.editAndRetry,
            ),
            (
              ProcurementErrorReason.quoteQualityRequired,
              '請填寫這批貨的實際品相等級與驗收說明——這一欄只能由你自己填。',
              ProcurementRecovery.editAndRetry,
            ),
            (
              ProcurementErrorReason.idempotencyKeyConflict,
              '這次送出和先前一次送出撞在同一把識別碼上，內容卻不一樣。請重新送出一次。',
              ProcurementRecovery.newIdempotencyKey,
            ),
            (
              ProcurementErrorReason.staleRevision,
              '你手上的版本已經過期。請重新載入最新內容後再送出。',
              ProcurementRecovery.reloadRequired,
            ),
            (
              ProcurementErrorReason.resourceNotFound,
              '找不到這則詢價，它可能已經被撤下。',
              ProcurementRecovery.terminal,
            ),
            (
              ProcurementErrorReason.capabilityUnavailable,
              '目前無法使用回價功能，請稍後再試。',
              ProcurementRecovery.editAndRetry,
            ),
          ])
        entry.$1.code: ProcurementErrorPresentation(
          code: entry.$1.code,
          message: entry.$2,
          recovery: entry.$3,
        ),
    };

/// 依錯誤碼查對照，找不到回傳 null。
ProcurementErrorPresentation? procurementErrorPresentation(String? code) {
  if (code == null || code.isEmpty) return null;
  return kProcurementErrorPresentations[code];
}

/// 採購線命令失敗。
///
/// 已知錯誤碼帶 [presentation]；未知碼只帶 server 的原文，畫面照樣顯示得出來。
class ProcurementException implements Exception {
  const ProcurementException({
    required this.message,
    this.code = '',
    this.statusCode,
    this.presentation,
    this.originalError,
  });

  /// 直接顯示給使用者的訊息。
  final String message;

  /// CodeMeta 錯誤碼；server 沒給時是空字串。
  final String code;

  final int? statusCode;

  final ProcurementErrorPresentation? presentation;

  final Object? originalError;

  /// 是否必須先重新載入最新版本才准再送。
  bool get requiresReload => presentation?.requiresReload ?? false;

  /// 是否要換一把新的冪等鍵才重下命令。
  bool get requiresNewIdempotencyKey =>
      presentation?.recovery == ProcurementRecovery.newIdempotencyKey;

  /// 這條路已經走不通。
  bool get isTerminal => presentation?.recovery == ProcurementRecovery.terminal;

  /// 從 gateway 回應轉成採購線例外。
  ///
  /// 非 4xx／5xx 的傳輸層錯誤（斷線、逾時）交回 [AppException.fromDioException]，
  /// 那一層的訊息已經夠用，不必在這裡重寫。[AppException] 是 sealed，本類別
  /// 因此不掛在它底下，只保證同樣是 `Exception` 且同樣有 `message`。
  static Exception from(DioException error) {
    final response = error.response;
    final body = response?.data;
    if (body is! Map) return AppException.fromDioException(error);

    // gateway 的 envelope 有兩種：錯誤碼直接掛在頂層，或包在 `error`／`data`
    // 底下。`code` 在成功 envelope 是數字，所以字面錯誤碼優先看 `reason`。
    final scopes = <Map<dynamic, dynamic>>[
      body,
      if (body['error'] is Map) body['error'] as Map,
      if (body['data'] is Map) body['data'] as Map,
    ];
    var code = '';
    for (final scope in scopes) {
      code = _readString(scope, const ['reason', 'errorCode', 'code']);
      if (code.isNotEmpty) break;
    }
    final presentation = procurementErrorPresentation(code);
    var serverMessage = '';
    for (final scope in scopes) {
      serverMessage = _readString(scope, const [
        'messageZh',
        'message',
        'errorMessage',
      ]);
      if (serverMessage.isNotEmpty) break;
    }

    if (presentation == null && serverMessage.isEmpty) {
      return AppException.fromDioException(error);
    }

    return ProcurementException(
      message: presentation?.message ?? serverMessage,
      code: code,
      statusCode: response?.statusCode,
      presentation: presentation,
      originalError: error,
    );
  }

  static String _readString(Map<dynamic, dynamic> body, List<String> keys) {
    for (final key in keys) {
      final value = body[key];
      if (value is String && value.trim().isNotEmpty) return value.trim();
    }
    return '';
  }

  @override
  String toString() =>
      'ProcurementException(${statusCode ?? '-'}, $code): $message';
}
