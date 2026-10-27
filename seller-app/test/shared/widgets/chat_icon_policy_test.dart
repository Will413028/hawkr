import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test('all seller chat entry points use the canonical chat icon name', () {
    final bubble = File(
      'lib/shared/widgets/chat_bubble_icon.dart',
    ).readAsStringSync();

    expect(bubble, contains('HkIconName.chat'));
  });

  test('seller AI and buyer conversations share one chat chrome system', () {
    final assistant = File(
      'lib/features/ai_assistant/presentation/ai_assistant_screen.dart',
    ).readAsStringSync();
    final thread = File(
      'lib/features/chat/presentation/chat_thread_screen.dart',
    ).readAsStringSync();
    final assistantAvatar = File(
      'lib/features/ai_assistant/presentation/widgets/'
      'ai_assistant_avatar.dart',
    ).readAsStringSync();

    for (final source in [assistant, thread]) {
      expect(source, contains('ChatConversationTopBar'));
      expect(source, contains('ChatConversationDotPattern'));
      expect(source, contains('ChatInputBar'));
      expect(source, isNot(contains('_ChatBgPainter')));
    }
    expect(assistant, contains('AiAssistantAvatar'));
    expect(assistantAvatar, contains('AppIcons.autoAwesome'));
  });

  test('settings status contrast belongs to the visible cover surface', () {
    final router = File('lib/core/router/app_router.dart').readAsStringSync();
    final store = File(
      'lib/features/post/presentation/page_screen.dart',
    ).readAsStringSync();
    final settings = File(
      'lib/features/seller/presentation/profile_screen.dart',
    ).readAsStringSync();
    final header = File(
      'lib/features/post/presentation/widgets/page_header.dart',
    ).readAsStringSync();

    expect(router, isNot(contains('currentIndex == 1 || currentIndex == 4')));
    expect(router, contains('SystemUiOverlayStyle.dark'));
    expect(store, contains('SystemUiOverlayStyle.light.copyWith'));
    expect(settings, contains('SystemUiOverlayStyle.dark.copyWith'));
    expect(header, contains('SystemUiOverlayStyle.light.copyWith'));
  });
}
