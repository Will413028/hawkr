import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_svg/flutter_svg.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';

void main() {
  for (final name in HkIconName.values) {
    testWidgets('HkIcon.${name.name} 畫 Material Icon 而不是 SVG', (tester) async {
      await tester.pumpWidget(MaterialApp(home: Scaffold(body: HkIcon(name))));

      final icon = tester.widget<Icon>(
        find.descendant(of: find.byType(HkIcon), matching: find.byType(Icon)),
      );
      expect(icon.icon?.fontFamily, 'MaterialIcons');
      expect(
        find.descendant(
          of: find.byType(HkIcon),
          matching: find.byType(SvgPicture),
        ),
        findsNothing,
      );
    });
  }

  test('pubspec 不依賴 iconly 與 lottie，lib 不引用兩者與 Lottie 素材', () {
    final pubspec = File('pubspec.yaml').readAsStringSync();
    final dependencies = RegExp(
      r'^dependencies:\n((?:[ \t].*\n|\n)*)',
      multiLine: true,
    ).firstMatch(pubspec)!.group(1)!;
    expect(
      RegExp(r'^\s+iconly:', multiLine: true).hasMatch(dependencies),
      isFalse,
    );
    expect(
      RegExp(r'^\s+lottie:', multiLine: true).hasMatch(dependencies),
      isFalse,
    );

    final offenders = <String>[];
    for (final entity in Directory('lib').listSync(recursive: true)) {
      if (entity is! File || !entity.path.endsWith('.dart')) continue;
      final source = entity.readAsStringSync();
      if (source.contains('package:iconly') ||
          source.contains('package:lottie') ||
          source.contains('assets/lottie/')) {
        offenders.add(entity.path);
      }
    }
    expect(offenders, isEmpty);
  });
}
