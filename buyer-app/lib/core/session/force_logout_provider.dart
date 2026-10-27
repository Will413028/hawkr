import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'force_logout_provider.g.dart';

/// 強制登出提醒旗標。
///
/// 當 session 失效（請求收到 401／unauthenticated）時設為 true，
/// 由 app 根層 widget 監聽此狀態顯示對話框，
/// 對話框與未登入提示一致；無論取消或前往登入都會清除失效 session。
@Riverpod(keepAlive: true)
class ForceLogout extends _$ForceLogout {
  @override
  bool build() => false;

  String? message;

  /// 觸發強制登出提醒（冪等：重複呼叫不會重複彈窗）
  void trigger({String? message}) {
    this.message = message ?? this.message;
    if (!state) state = true;
  }

  /// 對話框顯示後重置旗標
  void consume() {
    message = null;
    if (state) state = false;
  }
}
