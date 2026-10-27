import 'dart:io';

import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('dismiss glyphs map to their own Material close icons', () {
    expect(AppIcons.close, Icons.close);
    expect(AppIcons.closeRounded, Icons.close_rounded);
    expect(AppIcons.clear, Icons.clear);
    expect(AppIcons.cancel, Icons.cancel);
    expect(AppIcons.cancelOutlined, Icons.cancel_outlined);
  });

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
