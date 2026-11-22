// 聊天列表 row 與訊息的顯示文字 helper。預覽是伺服器回的結構化 `Preview`，句子取自共用文案目錄
// （`hawkr_ui_text` 的 `chatPreview*`），不在這裡偵測內容。
import 'package:hawkr_api/gen/hawkr/chat/v1alpha1/chat.pb.dart' as pb;
import 'package:hawkr_ui_text/hawkr_ui_text.dart';

import '../data/chat_model.dart';

/// 相對時間字樣（剛剛 / N 分鐘前 / N 小時前 / N 天前 / 日期）。
String formatTimeAgo(DateTime dt) {
  final now = DateTime.now();
  final diff = now.difference(dt);
  if (diff.inSeconds < 60) return '剛剛';
  if (diff.inMinutes < 60) return '${diff.inMinutes}分鐘前';
  if (diff.inHours < 24) return '${diff.inHours}小時前';
  if (diff.inDays < 7) return '${diff.inDays}天前';
  if (dt.year == now.year) return '${dt.month}/${dt.day}';
  return '${dt.year}/${dt.month}/${dt.day}';
}

/// 對話列表預覽的文字：墓碑與圖片用目錄的句子，文字訊息用內容；伺服器沒有帶預覽，或種類不認得時是空字串。
String previewText(ChatPreview? p, UiLocale locale) {
  if (p == null) return '';
  if (p.erased) return chatPreviewErased(locale);
  if (p.hidden) return chatPreviewHidden(locale);
  return switch (p.kind) {
    pb.MessageKind.MESSAGE_KIND_IMAGE => chatPreviewImage(locale),
    pb.MessageKind.MESSAGE_KIND_TEXT => p.text ?? '',
    _ => '',
  };
}

/// 墓碑訊息氣泡的文字；不是墓碑時是 null。
String? tombstoneText(ChatMessage m, UiLocale locale) {
  if (m.erased) return chatPreviewErased(locale);
  if (m.hidden) return chatPreviewHidden(locale);
  return null;
}
