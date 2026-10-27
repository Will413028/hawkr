import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  const customControlSurfaces = <String>[
    'lib/features/notification/presentation/notifications_screen.dart',
    'lib/shared/widgets/comments_bottom_sheet.dart',
    'lib/features/home/presentation/home_widget/home_member_card_section.dart',
    'lib/features/home/presentation/home_widget/home_banner_section.dart',
    'lib/shared/widgets/main_top_bar.dart',
  ];

  for (final path in customControlSurfaces) {
    test('$path keeps custom interactions on the shared a11y primitive', () {
      final source = File(path).readAsStringSync();
      final rawControls = RegExp(
        r'\b(?:GestureDetector|InkWell)\s*\(',
      ).allMatches(source);

      expect(
        rawControls,
        isEmpty,
        reason:
            'Use HkTapTarget so compact visuals retain a labeled 48dp target. '
            'Standard Flutter controls such as TextField/ElevatedButton remain '
            'valid.',
      );
    });
  }

  test('notification and comment controls retain their semantic inventory', () {
    final notificationSource = File(
      customControlSurfaces[0],
    ).readAsStringSync();
    final commentSource = File(customControlSurfaces[1]).readAsStringSync();

    for (final key in const <String>[
      'notifications-mark-all-read-action',
      'notifications-delete-mode-action',
      'notification-row-',
      'notification-selection-indicator-',
    ]) {
      expect(notificationSource, contains(key));
    }

    for (final key in const <String>[
      'comments-panel-close',
      'cancel-comment-reply',
      'send-comment-action',
      'comment-reply-',
      'comment-like-',
      'comment-more-replies-',
    ]) {
      expect(commentSource, contains(key));
    }
  });
}
