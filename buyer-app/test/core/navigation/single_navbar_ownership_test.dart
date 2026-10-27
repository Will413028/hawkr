import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test('only the global navigation frame owns the app Navbar', () {
    const framePath = 'lib/shared/widgets/buyer_navigation_frame.dart';
    const componentPath = 'lib/shared/widgets/shared_bottom_nav.dart';
    final dartFiles = Directory('lib')
        .listSync(recursive: true)
        .whereType<File>()
        .where((file) => file.path.endsWith('.dart'))
        .toList(growable: false);

    expect(
      'SharedBottomNav('.allMatches(File(framePath).readAsStringSync()).length,
      1,
    );
    for (final file in dartFiles.where(
      (file) => file.path != framePath && file.path != componentPath,
    )) {
      expect(
        file.readAsStringSync().contains('SharedBottomNav('),
        isFalse,
        reason: '${file.path} must provide content only',
      );
    }
  });

  test('imperative full-page routes stay confined to approved modal flows', () {
    const approved = {
      'lib/features/cart/presentation/store_pickup_form_screen.dart',
      'lib/features/chat/presentation/chat_thread_screen.dart',
    };
    final actual = Directory('lib')
        .listSync(recursive: true)
        .whereType<File>()
        .where((file) => file.path.endsWith('.dart'))
        .where((file) {
          final source = file.readAsStringSync();
          return source.contains('MaterialPageRoute') ||
              source.contains('PageRouteBuilder') ||
              source.contains('CupertinoPageRoute');
        })
        .map((file) => file.path)
        .toSet();

    expect(actual, approved);
  });

  test('persistent shell tab switches are instantaneous', () {
    final source = File(
      'lib/features/home/presentation/main_navigation_screen.dart',
    ).readAsStringSync();

    expect(source, contains('_pageController.jumpToPage(index);'));
    expect(source, isNot(contains('_pageController.animateToPage(')));
  });
}
