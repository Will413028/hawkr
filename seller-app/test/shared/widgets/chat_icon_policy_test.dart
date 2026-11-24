import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test('all seller chat entry points use the canonical chat icon name', () {
    final bubble = File(
      'lib/shared/widgets/chat_bubble_icon.dart',
    ).readAsStringSync();

    expect(bubble, contains('HkIconName.chat'));
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
