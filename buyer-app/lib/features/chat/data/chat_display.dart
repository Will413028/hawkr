import 'package:hawkr_api/gen/hawkr/chat/v1alpha1/chat.pb.dart' as pb;
import 'package:hawkr_ui_text/hawkr_ui_text.dart';

/// 這則訊息在對話裡顯示的文字（只用於文字氣泡與預覽）：已隱藏、已抹除與不認得的種類取共用文案目錄，
/// 圖片訊息沒有文字（畫面畫圖片）。回 null 表示該畫圖片。
String? chatMessageText(pb.Message m, UiLocale l) {
  if (m.hidden) return chatPreviewHidden(l);
  if (m.erased) return chatPreviewErased(l);
  switch (m.kind) {
    case pb.MessageKind.MESSAGE_KIND_TEXT:
      return m.hasText() ? m.text : '';
    case pb.MessageKind.MESSAGE_KIND_IMAGE:
      return null;
    default:
      return chatMessageUnsupportedKind(l);
  }
}

/// 對話列表上最後一則訊息的預覽文字。
String chatPreviewText(pb.Preview p, UiLocale l) {
  if (p.hidden) return chatPreviewHidden(l);
  if (p.erased) return chatPreviewErased(l);
  switch (p.kind) {
    case pb.MessageKind.MESSAGE_KIND_TEXT:
      return p.hasText() ? p.text : '';
    case pb.MessageKind.MESSAGE_KIND_IMAGE:
      return chatPreviewImage(l);
    default:
      return chatMessageUnsupportedKind(l);
  }
}

/// 橫幅等只有單則訊息的地方用的預覽：圖片取共用文案目錄的圖片預覽。
String chatMessagePreview(pb.Message m, UiLocale l) =>
    chatMessageText(m, l) ?? chatPreviewImage(l);
