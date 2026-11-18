import 'package:hooks_riverpod/hooks_riverpod.dart';

import 'session_provider.dart';

/// 目前的觀看者；登入、登出與換帳號時才變，session 的其他變化不會通知。
final viewerIdProvider = Provider<String?>(
  (ref) => ref.watch(sessionProvider.select((s) => s.value?.userId)),
);
