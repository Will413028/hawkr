import 'dart:convert';

import 'package:freezed_annotation/freezed_annotation.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:uuid/uuid.dart';

import 'package:hawkr_api/errors.dart'
    show ErrorAction, ErrorFacts, classifyError, errorText;
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/error_reason.pbenum.dart';
import 'package:hawkr_api/transport.dart' show ConnectException;

import '../../../core/l10n/ui_locale.dart';
import '../data/seller_quote_model.dart';
import '../data/seller_quote_repository.dart';
import 'seller_quote_validation.dart';

part 'seller_quote_controller.freezed.dart';
part 'seller_quote_controller.g.dart';

/// 畫面上那條橫幅的語氣。
enum SellerQuoteNoticeTone { info, success, warning, error }

/// 要顯示給賣家的一則訊息。
@freezed
abstract class SellerQuoteNotice with _$SellerQuoteNotice {
  const factory SellerQuoteNotice({
    required String message,
    @Default(SellerQuoteNoticeTone.info) SellerQuoteNoticeTone tone,

    /// ⛔ `STALE_SELLER_RESPONSE` 等「依據已過期」的錯誤會把它設成 true：
    /// 必須明確告知賣家報價已變動，並要求他自己重新載入，
    /// **不得靜默改成送最新版**。
    @Default(false) bool requiresReload,

    /// 伺服器的錯誤 reason 名稱，沒有時是空字串（給測試與紀錄用）。
    @Default('') String code,
  }) = _SellerQuoteNotice;
}

/// 回價表單的完整狀態。
@freezed
abstract class SellerQuoteFormState with _$SellerQuoteFormState {
  const factory SellerQuoteFormState({
    required SellerFacingPublication publication,

    /// 賣家看得到的需求內容（九欄，deny-by-default）。
    @Default(SellerFacingPayload()) SellerFacingPayload sellerFacingPayload,

    /// 表單目前的十七欄內容。
    required SellerQuotePayload quote,

    /// 草稿 CAS：`""` ＝ 還沒有草稿，下一次 PUT 會建立第一版。
    @Default('') String draftVersion,

    /// 伺服器上最新的正式回價（詳情的 `latestResponse`）；null ＝ 我還沒有正式回價。
    SellerResponseVersionResponse? latestResponse,

    /// 有命令在飛。
    @Default(false) bool busy,

    /// 目前的橫幅訊息。
    SellerQuoteNotice? notice,

    /// 已經按過送出（決定要不要把欄位錯誤紅字攤開）。
    @Default(false) bool submitAttempted,
  }) = _SellerQuoteFormState;

  const SellerQuoteFormState._();

  /// 正式提交的 CAS：`""` ＝「我還沒有正式回價」。
  ///
  /// ⛔ 值一律來自伺服器（詳情的 `latestResponse`，或上一次寫入命令自己的回應），
  /// 不是這台裝置記得的版本號。
  String get expectedSellerResponseVersion =>
      latestResponse?.sellerResponseVersion ?? '';

  /// 已經送過一版正式回價（送出按鈕要顯示「送出改版」）。
  bool get isRevision => expectedSellerResponseVersion.isNotEmpty;

  /// 依據已過期，必須先重新載入才准再送。
  bool get reloadRequired => notice?.requiresReload ?? false;
}

/// 冪等鍵配發簿，只給正式送出（`SubmitSellerResponse` 的 `client_request_id`）。
///
/// 值是 **stable client request id**：同一次使用者意圖重送要用同一個值，換一次意圖才換值。
/// 這裡把「意圖」寫成一段簽章（CAS＋payload），同簽章回同一把鍵，簽章一變就換新鍵——所以
/// 「按送出失敗→原封不動再按一次」是重送，「改了金額再送」是新意圖。撤回與婉拒本身冪等，不帶鍵。
class QuoteIdempotencyKeyBook {
  QuoteIdempotencyKeyBook({String Function()? mint})
    : _mint = mint ?? const Uuid().v4;

  final String Function() _mint;
  final Map<String, String> _keys = <String, String>{};

  /// 取這次意圖的冪等鍵。
  String keyFor(String signature) =>
      _keys.putIfAbsent(signature, () => _mint());

  /// 作廢這次意圖的鍵。
  ///
  /// 同一把鍵已經綁到另一份內容（`client_request_id` 的欄位錯誤）時，
  /// 重試同一把鍵永遠不會成功，必須換新鍵。
  void invalidate(String signature) => _keys.remove(signature);

  /// 送出成功後清掉，之後的改版是新的意圖。
  void clear() => _keys.clear();
}

@riverpod
QuoteIdempotencyKeyBook quoteIdempotencyKeyBook(Ref ref) =>
    QuoteIdempotencyKeyBook();

/// 關掉 Riverpod 3 的自動重試。
///
/// 預設會把失敗的 build 重試十次（200ms 起指數退避），而**重試期間的狀態是
/// loading 不是 error**——載入失敗的表單會轉半分鐘的圈才吐錯。回價是使用者主動
/// 發起的動作，失敗要立刻讓他看到並自己決定要不要重試。
Duration? _noAutoRetry(int retryCount, Object error) => null;

/// 平台賣家回價表單的狀態機。
///
/// 一個 publication 一份狀態（family key ＝ `publicationId`）。
@Riverpod(retry: _noAutoRetry)
class SellerQuoteForm extends _$SellerQuoteForm {
  SellerQuoteRepository get _repo => ref.read(sellerQuoteRepositoryProvider);

  QuoteIdempotencyKeyBook get _keys =>
      ref.read(quoteIdempotencyKeyBookProvider);

  @override
  Future<SellerQuoteFormState> build(String publicationId) async {
    final detail = await _repo.getPublicationDetail(publicationId);
    return _stateFromDetail(detail);
  }

  SellerQuoteFormState _stateFromDetail(SellerPublicationDetail detail) {
    final existing = detail.latestResponse?.quote;
    final draft = detail.draft?.draft;
    return SellerQuoteFormState(
      publication: detail.publication,
      sellerFacingPayload: detail.sellerFacingPayload,
      quote: draft ?? existing ?? const SellerQuotePayload(),
      draftVersion: detail.expectedDraftVersion,
      latestResponse: detail.latestResponse,
    );
  }

  SellerQuoteFormState? get _current => state.value;

  void _set(SellerQuoteFormState next) => state = AsyncData(next);

  /// 換掉整份 payload（欄位 widget 的 onChanged 都走這裡）。
  ///
  /// 內容一變就把過期橫幅收起來——但**不代表**可以重送：`reloadRequired` 的橫幅
  /// 只有 [reload] 會清掉。
  void updateQuote(SellerQuotePayload Function(SellerQuotePayload) edit) {
    final current = _current;
    if (current == null) return;
    final notice = current.notice;
    _set(
      current.copyWith(
        quote: edit(current.quote),
        notice: (notice != null && notice.requiresReload) ? notice : null,
      ),
    );
  }

  /// 收掉橫幅（使用者按 ✕）。
  void dismissNotice() {
    final current = _current;
    if (current == null) return;
    _set(current.copyWith(notice: null));
  }

  /// 重新載入最新版本——`STALE_*` 系列錯誤的唯一出路。
  ///
  /// 會用 server 上的最新草稿／回價覆蓋畫面，所以只在使用者明確按下「重新載入」
  /// 時呼叫。
  Future<void> reload() async {
    state = const AsyncLoading();
    state = await AsyncValue.guard(() async {
      _keys.clear();
      final detail = await _repo.getPublicationDetail(publicationId);
      return _stateFromDetail(detail).copyWith(
        notice: const SellerQuoteNotice(
          message: '已載入最新版本，請確認內容後再送出。',
          tone: SellerQuoteNoticeTone.info,
        ),
      );
    });
  }

  /// 存草稿。可不限次改；不計正式一次，也不驗金額恆等式。
  ///
  /// CAS 是 `expectedDraftVersion`（`""` ＝ 建立第一版），**不帶冪等鍵**。
  Future<bool> saveDraft() async {
    final current = _current;
    if (current == null || current.busy) return false;
    _set(current.copyWith(busy: true, notice: null));
    try {
      final saved = await _repo.saveDraft(
        publicationId: publicationId,
        draft: current.quote,
        expectedDraftVersion: current.draftVersion,
      );
      _set(
        current.copyWith(
          busy: false,
          draftVersion: saved.draftVersion,
          notice: const SellerQuoteNotice(
            message: '草稿已儲存。草稿不會送到買家手上，也不算一次正式回價。',
            tone: SellerQuoteNoticeTone.success,
          ),
        ),
      );
      return true;
    } catch (error) {
      _set(current.copyWith(busy: false, notice: _noticeFor(error)));
      return false;
    }
  }

  /// 正式提交／改版（同一支端點）。
  ///
  /// 送出前先跑 [validateSellerQuote]：品相沒填、金額恆等式不成立時直接擋下，
  /// 不浪費一把冪等鍵去換一個必然的 422。
  Future<SellerResponseVersionResponse?> submit({DateTime? now}) async {
    final current = _current;
    if (current == null || current.busy) return null;
    if (current.reloadRequired) return null;

    final validation = validateSellerQuote(current.quote, now: now);
    if (!validation.isSubmittable) {
      _set(
        current.copyWith(
          submitAttempted: true,
          notice: SellerQuoteNotice(
            message: _blockedMessage(validation),
            tone: SellerQuoteNoticeTone.warning,
          ),
        ),
      );
      return null;
    }

    final signature = _submitSignature(current);
    final key = _keys.keyFor(signature);
    _set(current.copyWith(busy: true, submitAttempted: true, notice: null));
    try {
      final response = await _repo.submitResponse(
        publicationId: publicationId,
        quote: current.quote,
        expectedPublicationVersion: current.publication.publicationVersion,
        expectedSellerFacingPayloadHash:
            current.publication.sellerFacingPayloadHash,
        expectedSellerResponseVersion: current.expectedSellerResponseVersion,
        clientRequestId: key,
      );
      _keys.clear();
      _set(
        current.copyWith(
          busy: false,
          latestResponse: response,
          quote: response.quote,
          notice: SellerQuoteNotice(
            message: _submitSuccessMessage(response),
            tone: SellerQuoteNoticeTone.success,
          ),
        ),
      );
      return response;
    } catch (error) {
      _afterCommandFailure(current, error, signature);
      return null;
    }
  }

  /// 撤回目前這一版正式回價。撤回本身冪等，不帶 request id；成功後重讀詳情取得新狀態。
  Future<bool> withdraw() async {
    final current = _current;
    final response = current?.latestResponse;
    if (current == null || response == null || current.busy) return false;

    _set(current.copyWith(busy: true, notice: null));
    try {
      await _repo.withdrawResponse(
        publicationId: publicationId,
        sellerResponseId: response.sellerResponseId,
        expectedSellerResponseVersion: response.sellerResponseVersion,
      );
      _keys.clear();
      final detail = await _repo.getPublicationDetail(publicationId);
      _set(
        current.copyWith(
          busy: false,
          latestResponse: detail.latestResponse,
          notice: const SellerQuoteNotice(
            message: '已撤回這一版報價。',
            tone: SellerQuoteNoticeTone.success,
          ),
        ),
      );
      return true;
    } catch (error) {
      _afterCommandFailure(current, error, '');
      return false;
    }
  }

  /// 明確拒絕這則詢價；[reasonCode] 空字串表示沒有填。婉拒本身冪等，不帶 request id。
  Future<bool> decline({String reasonCode = ''}) async {
    final current = _current;
    if (current == null || current.busy) return false;

    _set(current.copyWith(busy: true, notice: null));
    try {
      await _repo.declinePublication(
        publicationId: publicationId,
        reasonCode: reasonCode,
      );
      _set(
        current.copyWith(
          busy: false,
          notice: const SellerQuoteNotice(
            message: '已拒絕這則詢價，之後不會再收到它的回價提醒。',
            tone: SellerQuoteNoticeTone.success,
          ),
        ),
      );
      return true;
    } catch (error) {
      _afterCommandFailure(current, error, '');
      return false;
    }
  }

  void _afterCommandFailure(
    SellerQuoteFormState current,
    Object error,
    String signature,
  ) {
    if (_isRequestIdConflict(error)) _keys.invalidate(signature);
    _set(current.copyWith(busy: false, notice: _noticeFor(error)));
  }

  /// 「這一次意圖」的簽章。CAS 與 payload 任一改變就是新意圖。
  String _submitSignature(SellerQuoteFormState current) {
    return jsonEncode(<String, dynamic>{
      'publicationId': publicationId,
      'expectedPublicationVersion': current.publication.publicationVersion,
      'expectedSellerFacingPayloadHash':
          current.publication.sellerFacingPayloadHash,
      'expectedSellerResponseVersion': current.expectedSellerResponseVersion,
      'quote': quoteToProto(current.quote).writeToJson(),
    });
  }

  static String _blockedMessage(SellerQuoteValidation validation) {
    if (!validation.amountIdentityHolds &&
        validation.computedTotal != null &&
        validation.fieldErrors.containsKey(SellerQuoteField.totalAmount)) {
      return '總金額必須等於貨款＋運費＋稅額（三項合計為 ${validation.computedTotal}）。';
    }
    final missing = validation.missingRequiredFields.length;
    return '還有 $missing 個必填欄位沒完成，請看紅字提示。';
  }

  static String _submitSuccessMessage(SellerResponseVersionResponse response) {
    final version = response.sellerResponseVersion;
    final base = response.replayed
        ? '這次送出與先前那一次是同一筆，已回到第 $version 版。'
        : '已送出第 $version 版報價。';
    if (!response.outsideBuyerRequestedRange) return base;
    return '$base你的報價落在買家原本說明的範圍之外，仍然已經送出，買家會看到它。';
  }

  SellerQuoteNotice _noticeFor(Object error) {
    final e = ConnectException.from(error);
    final facts = ErrorFacts.fromException(e);
    final decision = classifyError(facts);
    final reason = decision.action == ErrorAction.reason
        ? decision.reason
        : null;
    return SellerQuoteNotice(
      message: errorText(e, defaults: ref.read(defaultErrorTextProvider)),
      tone: SellerQuoteNoticeTone.error,
      requiresReload:
          reason == ErrorReason.VERSION_CONFLICT ||
          reason == ErrorReason.RFQ_RESPONSE_EXPIRED,
      code: reason?.name ?? '',
    );
  }

  /// 同一個 `client_request_id` 已綁到另一份內容：欄位錯誤指在 `client_request_id`。
  static bool _isRequestIdConflict(Object error) => ErrorFacts.fromException(
    ConnectException.from(error),
  ).fieldErrors.any((f) => f.field == 'client_request_id');
}
