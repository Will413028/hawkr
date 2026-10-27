import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_svg/flutter_svg.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';

void main() {
  group('HkIcon', () {
    for (final name in HkIconName.values) {
      testWidgets('${name.name} 畫 Material Icon', (tester) async {
        await tester.pumpWidget(MaterialApp(home: Center(child: HkIcon(name))));

        final icon = tester.widget<Icon>(
          find.descendant(of: find.byType(HkIcon), matching: find.byType(Icon)),
        );
        expect(icon.icon, isNotNull);
        expect(icon.icon!.fontFamily, 'MaterialIcons');
        expect(
          find.descendant(
            of: find.byType(HkIcon),
            matching: find.byType(SvgPicture),
          ),
          findsNothing,
        );
      });
    }
  });

  group('圖示來源', () {
    test('pubspec dependencies 沒有 iconly、lottie', () {
      final pubspec = File('pubspec.yaml').readAsStringSync();
      expect(
        RegExp(r'^\s+iconly\s*:', multiLine: true).hasMatch(pubspec),
        isFalse,
      );
      expect(
        RegExp(r'^\s+lottie\s*:', multiLine: true).hasMatch(pubspec),
        isFalse,
      );
    });

    test('lib 沒有 import iconly、lottie 或引用 assets/lottie/', () {
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

    test('lib 沒有第三方商標圖形（Material Icons 裡的品牌常數）', () {
      final brand = RegExp(
        r'\bIcons\.(android|apple|discord|facebook|g_mobiledata|paypal|'
        r'reddit|snapchat|telegram|tiktok|wechat)(_sharp|_rounded|_outlined)?\b',
      );
      final offenders = <String>[];
      for (final entity in Directory('lib').listSync(recursive: true)) {
        if (entity is! File || !entity.path.endsWith('.dart')) continue;
        if (brand.hasMatch(entity.readAsStringSync())) {
          offenders.add(entity.path);
        }
      }
      expect(offenders, isEmpty);
    });
  });
}
