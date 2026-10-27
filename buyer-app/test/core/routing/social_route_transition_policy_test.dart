import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test('social preview routes keep the existing page and Navbar mounted', () {
    final source = File('lib/core/routing/app_router.dart').readAsStringSync();
    final socialStart = source.indexOf("path: '/social/posts'");
    final groupedRoutesStart = source.indexOf(
      '// ── Grouped sub-pages',
      socialStart,
    );

    expect(socialStart, greaterThanOrEqualTo(0));
    expect(groupedRoutesStart, greaterThan(socialStart));

    final socialRoutes = source.substring(socialStart, groupedRoutesStart);
    expect(
      RegExp(
        r"path: '/social/posts',[\s\S]*?pageBuilder:[\s\S]*?NoTransitionPage<void>",
      ).hasMatch(socialRoutes),
      isTrue,
    );
    expect(
      RegExp(
        r"path: '/social/post/:postId',[\s\S]*?pageBuilder:[\s\S]*?floatingDetailPage<void>",
      ).hasMatch(socialRoutes),
      isTrue,
    );
  });
}
