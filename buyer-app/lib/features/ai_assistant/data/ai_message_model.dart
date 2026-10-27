import 'package:freezed_annotation/freezed_annotation.dart';

part 'ai_message_model.freezed.dart';

@freezed
abstract class AiOrderItem with _$AiOrderItem {
  const factory AiOrderItem({
    required String orderId,
    required String status,
    required int amountTwd,
    @Default(0) int createTimeMs,
  }) = _AiOrderItem;
}

@freezed
abstract class AiMessage with _$AiMessage {
  const factory AiMessage({
    required String role, // 'user' | 'assistant'
    required String content,
    required DateTime timestamp,
    List<AiOrderItem>? orderList,
  }) = _AiMessage;
}
