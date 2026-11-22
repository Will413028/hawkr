import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/chat/v1alpha1/chat.pb.dart' as pb;
import 'package:hawkr_seller/features/chat/data/chat_model.dart';
import 'package:hawkr_seller/features/chat/presentation/chat_preview.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';

import 'support/chat_fakes.dart';

ChatPreview _p({
  pb.MessageKind kind = pb.MessageKind.MESSAGE_KIND_TEXT,
  String? text,
  bool hidden = false,
  bool erased = false,
}) => ChatPreview(
  kind: kind,
  text: text,
  hidden: hidden,
  erased: erased,
  fromStore: false,
);

void main() {
  group('previewText', () {
    test('text preview is the text', () {
      expect(previewText(_p(text: '你好'), UiLocale.zhTW), '你好');
    });

    test('image preview uses the catalog sentence', () {
      expect(
        previewText(_p(kind: pb.MessageKind.MESSAGE_KIND_IMAGE), UiLocale.zhTW),
        chatPreviewImage(UiLocale.zhTW),
      );
    });

    test('hidden and erased use the catalog sentences, even with text', () {
      expect(
        previewText(_p(text: 'secret', hidden: true), UiLocale.zhTW),
        chatPreviewHidden(UiLocale.zhTW),
      );
      expect(
        previewText(_p(text: 'secret', erased: true), UiLocale.zhTW),
        chatPreviewErased(UiLocale.zhTW),
      );
    });

    test('no preview or an unknown kind is empty, never raw payload', () {
      expect(previewText(null, UiLocale.zhTW), '');
      expect(
        previewText(
          _p(kind: pb.MessageKind.MESSAGE_KIND_UNSPECIFIED, text: '{"a":1}'),
          UiLocale.zhTW,
        ),
        '',
      );
    });
  });

  group('tombstoneText', () {
    test('tombstones show the catalog sentence, normal messages null', () {
      expect(
        tombstoneText(
          ChatMessage.fromProto(protoMessage(1, hidden: true)),
          UiLocale.zhTW,
        ),
        chatPreviewHidden(UiLocale.zhTW),
      );
      expect(tombstoneText(clientMessage(1), UiLocale.zhTW), isNull);
    });
  });
}
