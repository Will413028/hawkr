import 'dart:io';

import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('font sizes follow the website type scale', () {
    final violations = <String>[];
    final fontSize = RegExp(r'fontSize:\s*(\d+)(?:\.sp)?\b');
    const allowed = <int>{12, 14, 16, 18, 20, 24, 30, 36, 48};
    for (final file in _dartFiles()) {
      final values = fontSize
          .allMatches(file.readAsStringSync())
          .map((match) => int.parse(match.group(1)!));
      if (values.any((value) => !allowed.contains(value))) {
        violations.add(file.path);
      }
    }
    expect(
      violations,
      isEmpty,
      reason: 'Use the website 12–48px type ladder: $violations',
    );
  });

  test('text is never silently scaled down to fit', () {
    final violations = <String>[];
    final shrinkingText = RegExp(
      r'FittedBox\([\s\S]{0,600}?fit:\s*BoxFit\.scaleDown[\s\S]{0,600}?\bText(?:\.rich)?\s*\(',
    );
    for (final file in _dartFiles()) {
      if (shrinkingText.hasMatch(file.readAsStringSync())) {
        violations.add(file.path);
      }
    }
    expect(
      violations,
      isEmpty,
      reason: 'Use responsive layout plus ellipsis/wrapping: $violations',
    );
  });

  test('font weights follow the website scale', () {
    final violations = <String>[];
    final nonCanonicalWeight = RegExp(r'FontWeight\.w(?:[12368]00)\b');
    for (final file in _dartFiles()) {
      if (nonCanonicalWeight.hasMatch(file.readAsStringSync())) {
        violations.add(file.path);
      }
    }
    expect(
      violations,
      isEmpty,
      reason: 'Use only the website 400 / 500 / 700 / 900 weights: $violations',
    );
  });

  test('surface radii follow the website scale', () {
    final violations = <String>[];
    final hardcodedRadius = RegExp(
      r'(?:BorderRadius|Radius)\.circular\((\d+)(?:\.(?:r|w|h|sp))?\)',
    );
    const allowed = <int>{0, 1, 2, 3, 6, 8, 12, 16, 24, 32, 999};
    for (final file in _dartFiles()) {
      final values = hardcodedRadius
          .allMatches(file.readAsStringSync())
          .map((match) => int.parse(match.group(1)!));
      if (values.any((value) => !allowed.contains(value))) {
        violations.add(file.path);
      }
    }
    expect(
      violations,
      isEmpty,
      reason: 'Use the website radius ladder from HkRadius: $violations',
    );
  });

  test('padding follows the website 4px spacing scale', () {
    final violations = <String>[];
    final edgeInsets = RegExp(
      r'EdgeInsets\.(?:all|symmetric|only|fromLTRB)\(([\s\S]{0,360}?)\)',
    );
    final literal = RegExp(
      r'(?<![/\w.])(\d+(?:\.\d+)?)(?:\.[whr])?(?=\s*(?:,|\)))',
    );
    const allowed = <double>[0, 4, 8, 12, 16, 20, 24, 32, 40, 48, 64, 80, 96];
    for (final file in _dartFiles()) {
      final source = file.readAsStringSync();
      final values = edgeInsets
          .allMatches(source)
          .expand(
            (insets) => literal
                .allMatches(insets.group(1)!)
                .map((match) => double.parse(match.group(1)!)),
          );
      if (values.any(
        (value) => !allowed.contains(value) && !(value > 96 && value % 4 == 0),
      )) {
        violations.add(file.path);
      }
    }
    expect(
      violations,
      isEmpty,
      reason: 'Use the website HkSpace 4px ladder: $violations',
    );
  });

  test('opaque UI colors stay in AppColors', () {
    final violations = <String>[];
    final opaqueLiteral = RegExp(r'Color\(0xFF[0-9A-Fa-f]{6}\)');
    for (final file in _dartFiles()) {
      if (file.path.endsWith('core/theme/app_colors.dart')) continue;
      if (file.path.endsWith('shared/widgets/fcm_debug_overlay.dart') ||
          file.path.endsWith('shared/widgets/fruit_refresh_indicator.dart') ||
          file.path.endsWith('shared/widgets/fruit_spinner.dart') ||
          file.path.endsWith('shared/widgets/animated_orbs_background.dart') ||
          file.path.contains('/features/photo_editor/') ||
          file.path.contains('/shared/widgets/rich_text_editor')) {
        continue;
      }
      if (opaqueLiteral.hasMatch(file.readAsStringSync())) {
        violations.add(file.path);
      }
    }
    expect(
      violations,
      isEmpty,
      reason:
          'Use semantic AppColors; keep content palettes isolated: $violations',
    );
  });

  test('body text roles meet WCAG AA on website surfaces', () {
    for (final foreground in <Color>[
      AppColors.textEmphasis,
      AppColors.textPrimary,
      AppColors.textSecondary,
    ]) {
      expect(
        _contrast(foreground, AppColors.surfaceCard),
        greaterThanOrEqualTo(4.5),
      );
      expect(_contrast(foreground, AppColors.bgApp), greaterThanOrEqualTo(4.5));
    }
  });
}

Iterable<File> _dartFiles() => Directory('lib')
    .listSync(recursive: true)
    .whereType<File>()
    .where((file) => file.path.endsWith('.dart'));

double _contrast(Color a, Color b) {
  final high = a.computeLuminance() >= b.computeLuminance() ? a : b;
  final low = identical(high, a) ? b : a;
  return (high.computeLuminance() + 0.05) / (low.computeLuminance() + 0.05);
}
