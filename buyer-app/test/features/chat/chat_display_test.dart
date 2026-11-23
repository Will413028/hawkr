import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/chat/v1alpha1/chat.pb.dart' as pb;
import 'package:hawkr_buyer/features/chat/data/chat_display.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';

import 'chat_fakes.dart';

void main() {
  const l = UiLocale.zhTW;

  test('訊息文字：文字、已隱藏、已抹除、不認得的種類取共用文案目錄，圖片回 null', () {
    expect(chatMessageText(chatMessage('c', 1, text: '你好'), l), '你好');
    expect(
      chatMessageText(chatMessage('c', 1, hidden: true), l),
      chatPreviewHidden(l),
    );
    expect(
      chatMessageText(chatMessage('c', 1, erased: true), l),
      chatPreviewErased(l),
    );
    expect(
      chatMessageText(
        chatMessage('c', 1, kind: pb.MessageKind.MESSAGE_KIND_UNSPECIFIED),
        l,
      ),
      chatMessageUnsupportedKind(l),
    );
    expect(
      chatMessageText(
        chatMessage('c', 1, kind: pb.MessageKind.MESSAGE_KIND_IMAGE),
        l,
      ),
      isNull,
    );
  });

  test('預覽：文字、圖片、隱藏、抹除、不認得的種類', () {
    expect(
      chatPreviewText(
        pb.Preview(kind: pb.MessageKind.MESSAGE_KIND_TEXT, text: 'hi'),
        l,
      ),
      'hi',
    );
    expect(
      chatPreviewText(pb.Preview(kind: pb.MessageKind.MESSAGE_KIND_IMAGE), l),
      chatPreviewImage(l),
    );
    expect(chatPreviewText(pb.Preview(hidden: true), l), chatPreviewHidden(l));
    expect(chatPreviewText(pb.Preview(erased: true), l), chatPreviewErased(l));
    expect(chatPreviewText(pb.Preview(), l), chatMessageUnsupportedKind(l));
  });
}
