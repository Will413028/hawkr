import 'dart:io';

import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('font-size expression parser covers every conditional branch', () {
    const source = '''
TextStyle(fontSize: compact ? 11.sp : 13.sp),
TextStyle(
  fontSize: isPhone
      ? 15.w
      : (isTablet ? 16 : 18),
),
''';

    expect(
      _fontSizeValues(source),
      orderedEquals(<double>[11, 13, 15, 16, 18]),
    );
  });

  test('font sizes follow the website type scale', () {
    final violations = <String>[];
    for (final file in _dartFiles()) {
      final source = file.readAsStringSync();
      for (final expression in _fontSizeExpressions(source)) {
        final invalidValues = _numericLiterals(expression.value).where((value) {
          // Web TemperatureTag is a scoped 11dp badge role, not body copy.
          final temperatureBadge =
              file.path == 'lib/shared/widgets/hk_temperature_tag.dart' &&
              value == 11;
          final cartSection =
              file.path ==
                  'lib/features/cart/presentation/cart_typography.dart' &&
              value == 22;
          return !_websiteTypeScale.contains(value) &&
              !temperatureBadge &&
              !cartSection;
        }).toList();
        if (invalidValues.isNotEmpty) {
          violations.add(
            '${file.path}:${expression.line} '
            '${invalidValues.join('/')} in `${expression.value.trim()}`',
          );
        }
      }
    }
    expect(
      violations,
      isEmpty,
      reason:
          'Use only the app 12/14/16/18/20/24 type ladder: '
          '$violations',
    );
  });

  test('font sizes use fixed logical pixels instead of ScreenUtil scaling', () {
    final violations = <String>[];
    final screenUtilUnit = RegExp(r'\.(?:sp|w|h|r)\b');

    for (final file in _dartFiles()) {
      final source = file.readAsStringSync();
      for (final expression in _fontSizeExpressions(source)) {
        if (screenUtilUnit.hasMatch(expression.value)) {
          violations.add(
            '${file.path}:${expression.line} `${expression.value.trim()}`',
          );
        }
      }
    }

    expect(
      violations,
      isEmpty,
      reason:
          'Flutter logical pixels already normalize device density. Keep text '
          'on the fixed website scale and let TextScaler handle accessibility; '
          'do not bind fontSize to screen width: $violations',
    );
  });

  test('legacy ScreenUtil units cannot globally shrink the interface', () {
    final source = File('lib/main.dart').readAsStringSync();
    expect(
      RegExp(r'enableScaleWH:\s*\(\)\s*=>\s*false').hasMatch(source),
      isTrue,
      reason:
          'Keep legacy .w/.h/.r values as logical pixels; responsive changes '
          'must happen through breakpoints and constraints.',
    );
    expect(
      RegExp(r'enableScaleText:\s*\(\)\s*=>\s*false').hasMatch(source),
      isTrue,
      reason:
          'Text uses the fixed website type scale; accessibility is handled '
          'by MediaQuery.textScaler.',
    );
  });

  test('text is never silently scaled down to fit', () {
    final violations = <String>[];
    final shrinkingText = RegExp(
      r'FittedBox\([\s\S]{0,600}?fit:\s*BoxFit\.scaleDown[\s\S]{0,600}?\bText(?:\.rich)?\s*\(',
    );
    for (final file in _dartFiles()) {
      final source = file.readAsStringSync();
      if (shrinkingText.hasMatch(source)) violations.add(file.path);
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
      var source = file.readAsStringSync();
      // CartTypography's scoped 600 emphasis is documented in component-library.md.
      // Only these cart consumers use it; other non-canonical weights still fail.
      const cartEmphasisPaths = {
        'lib/features/cart/presentation/cart_typography.dart',
        'lib/features/cart/presentation/widgets/coupon_selection_sheet.dart',
        'lib/features/cart/presentation/widgets/cart_product_tile.dart',
        'lib/features/cart/presentation/widgets/shipping_section.dart',
      };
      if (cartEmphasisPaths.contains(file.path)) {
        source = source.replaceAll('FontWeight.w600', 'FontWeight.w700');
      }
      if (nonCanonicalWeight.hasMatch(source)) {
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
          file.path.endsWith('shared/widgets/animated_orbs_background.dart')) {
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

  test('primary body text roles meet WCAG AA on website surfaces', () {
    for (final foreground in <Color>[
      AppColors.textEmphasis,
      AppColors.textPrimary,
    ]) {
      expect(
        _contrast(foreground, AppColors.bgCard),
        greaterThanOrEqualTo(4.5),
      );
      expect(
        _contrast(foreground, AppColors.bgPrimary),
        greaterThanOrEqualTo(4.5),
      );
    }
  });

  test('website muted role remains visible on app surfaces', () {
    for (final surface in <Color>[AppColors.bgCard, AppColors.bgPrimary]) {
      expect(
        _contrast(AppColors.textSecondary, surface),
        greaterThanOrEqualTo(3),
      );
      expect(_contrast(AppColors.textHint, surface), greaterThanOrEqualTo(3));
    }
  });
}

const List<double> _websiteTypeScale = <double>[12, 14, 16, 18, 20, 24];

final class _FontSizeExpression {
  const _FontSizeExpression(this.value, this.line);

  final String value;
  final int line;
}

/// Extracts a complete Dart `fontSize:` value, including multiline and nested
/// ternaries. A line-based regex used to miss the second branch of expressions
/// such as `compact ? 11.sp : 13.sp`, which is exactly where off-scale sizes
/// tended to survive.
Iterable<_FontSizeExpression> _fontSizeExpressions(String source) sync* {
  final property = RegExp(r'\bfontSize\s*:');
  for (final match in property.allMatches(source)) {
    final start = match.end;
    var roundDepth = 0;
    var squareDepth = 0;
    var curlyDepth = 0;
    String? quote;
    var end = source.length;

    for (var index = start; index < source.length; index++) {
      final char = source[index];
      if (quote != null) {
        if (char == r'\') {
          index++;
        } else if (char == quote) {
          quote = null;
        }
        continue;
      }
      if (char == '"' || char == "'") {
        quote = char;
        continue;
      }
      if (char == '(') {
        roundDepth++;
      } else if (char == '[') {
        squareDepth++;
      } else if (char == '{') {
        curlyDepth++;
      } else if (char == ')') {
        if (roundDepth == 0 && squareDepth == 0 && curlyDepth == 0) {
          end = index;
          break;
        }
        roundDepth--;
      } else if (char == ']') {
        squareDepth--;
      } else if (char == '}') {
        curlyDepth--;
      } else if ((char == ',' || char == ';') &&
          roundDepth == 0 &&
          squareDepth == 0 &&
          curlyDepth == 0) {
        end = index;
        break;
      }
    }

    yield _FontSizeExpression(
      source.substring(start, end),
      '\n'.allMatches(source.substring(0, match.start)).length + 1,
    );
  }
}

List<double> _fontSizeValues(String source) => _fontSizeExpressions(
  source,
).expand((expression) => _numericLiterals(expression.value)).toList();

Iterable<double> _numericLiterals(String expression) => RegExp(
  r'(?<![\w.])(\d+(?:\.\d+)?)(?!\d)',
).allMatches(expression).map((match) => double.parse(match.group(1)!));

Iterable<File> _dartFiles() => Directory('lib')
    .listSync(recursive: true)
    .whereType<File>()
    .where((file) => file.path.endsWith('.dart'));

double _contrast(Color a, Color b) {
  final high = a.computeLuminance() >= b.computeLuminance() ? a : b;
  final low = identical(high, a) ? b : a;
  return (high.computeLuminance() + 0.05) / (low.computeLuminance() + 0.05);
}
