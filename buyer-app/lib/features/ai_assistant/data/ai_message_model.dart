import 'package:freezed_annotation/freezed_annotation.dart';

part 'ai_message_model.freezed.dart';

@freezed
abstract class AiMessage with _$AiMessage {
  const factory AiMessage({
    required String role, // 'user' | 'assistant'
    required String content,
    required DateTime timestamp,
  }) = _AiMessage;
}
