import 'dart:io';

import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('dismiss aliases use the plain unframed X', () {
    expect(AppIcons.close, Icons.close);
    expect(AppIcons.closeRounded, Icons.close_rounded);
    expect(AppIcons.clear, Icons.clear);
  });

  test(
    'feature code only uses cancel glyphs for the cancelled order state',
    () {
      final offenders = Directory('lib')
          .listSync(recursive: true)
          .whereType<File>()
          .where((file) => file.path.endsWith('.dart'))
          .where(
            (file) => RegExp(
              r'AppIcons\.cancel(Outlined)?\b',
            ).hasMatch(file.readAsStringSync()),
          )
          .map((file) => file.path)
          .toList();

      expect(offenders, ['lib/shared/widgets/hk_icon.dart']);
    },
  );

  test('feature code does not use framed close-square glyphs', () {
    final offenders = Directory('lib')
        .listSync(recursive: true)
        .whereType<File>()
        .where((file) => file.path.endsWith('.dart'))
        .where((file) => file.readAsStringSync().contains('.close_square'))
        .map((file) => file.path)
        .toList();

    expect(offenders, isEmpty);
  });
}
