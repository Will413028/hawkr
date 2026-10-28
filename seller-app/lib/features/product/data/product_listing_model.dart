import 'package:freezed_annotation/freezed_annotation.dart';

part 'product_listing_model.freezed.dart';
part 'product_listing_model.g.dart';

/// AI 對話上架助手累積的商品草稿。
/// 各 step screen 進入時 useEffect 把欄位套進自己的 local state。
@freezed
abstract class PendingAiDraft with _$PendingAiDraft {
  const factory PendingAiDraft({
    String? name,
    String? origin,
    String? categoryId,
    String? description,
    @Default([]) List<String> tags,
    @Default([]) List<SpecDraftPending> skus,
    @Default({}) Set<String> filledFields,
    @Default(0) int patchVersion,
  }) = _PendingAiDraft;

  factory PendingAiDraft.fromJson(Map<String, dynamic> json) =>
      _$PendingAiDraftFromJson(json);
}

@freezed
abstract class SpecDraftPending with _$SpecDraftPending {
  const factory SpecDraftPending({
    required String skuName,
    @Default(0) int price,
    // PayUni #758: AI 助手可建議特價（0/空=無特價）
    @Default(0) int salePrice,
    @Default('') String unit,
    @Default(0) int weight,
    @Default(0) int stockQty,
  }) = _SpecDraftPending;

  factory SpecDraftPending.fromJson(Map<String, dynamic> json) =>
      _$SpecDraftPendingFromJson(json);
}

/// SSE 單一事件（text / draft_patch / tool_result / done / error）
///
/// [terminal] 表示這個事件是否代表串流真正結束——只有當底層
/// `AgUiEvent.terminal`（見 `core/networking/agui_sse.dart`）為 true，或
/// repository 端判斷串流已無法繼續（例如 `resp.data?.stream == null`）時
/// 才會是 true。[type] 對 `tool_result`/`draft_patch` 事件而言可能是後端
/// 可控的自由字串，呼叫端判斷串流是否該停止消費一律看 [terminal]，不要用
/// [type] 字串比對（例如 `type == 'done'`）——哪天有個工具或自訂事件剛好
/// 叫 done/error，字串比對會把它誤判成控制流訊號，把整段對話提前截斷。
class SseEvent {
  SseEvent({required this.type, required this.data, this.terminal = false});
  final String type;
  final String data;
  final bool terminal;
}

/// 對話單則訊息（user / assistant）
@freezed
abstract class ChatMessage with _$ChatMessage {
  const factory ChatMessage({
    required String role, // 'user' | 'assistant'
    @Default('') String text,
  }) = _ChatMessage;
}
