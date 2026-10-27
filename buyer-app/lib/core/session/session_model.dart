import 'package:freezed_annotation/freezed_annotation.dart';

part 'session_model.freezed.dart';

@freezed
abstract class SessionModel with _$SessionModel {
  const factory SessionModel({
    // 帳號 ID，不透明的字串。
    required String userId,
    String? nickname,
  }) = _SessionModel;
}
