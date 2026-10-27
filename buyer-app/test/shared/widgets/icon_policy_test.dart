import 'dart:io';

import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('only the AppIcons registry references Material Icons directly', () {
    final violations = <String>[];
    for (final entity in Directory('lib').listSync(recursive: true)) {
      if (entity is! File || !entity.path.endsWith('.dart')) continue;
      if (entity.path.endsWith('shared/icons/app_icons.dart')) continue;
      final source = entity.readAsStringSync();
      if (RegExp(r'\bIcons\s*\.').hasMatch(source) ||
          source.contains('CupertinoIcons')) {
        violations.add(entity.path);
      }
    }
    expect(
      violations,
      isEmpty,
      reason: 'Use AppIcons or HkIcon instead of Icons: $violations',
    );
  });

  test('SvgPicture.asset is only used for brand assets', () {
    const allowedPrefixes = ['assets/brand/'];
    final svgLiteral = RegExp(r"""['"](assets/[^'"]+\.svg)['"]""");
    final violations = <String>[];
    for (final entity in Directory('lib').listSync(recursive: true)) {
      if (entity is! File || !entity.path.endsWith('.dart')) continue;
      final source = entity.readAsStringSync();
      for (final match in svgLiteral.allMatches(source)) {
        final path = match.group(1)!;
        if (!allowedPrefixes.any(path.startsWith)) {
          violations.add('${entity.path}: $path');
        }
      }
    }
    expect(
      violations,
      isEmpty,
      reason: 'UI icons are Material Icons; only brand art stays SVG',
    );
  });

  test('every registry entry is a Material icon or an alias of one', () {
    final registry = File('lib/shared/icons/app_icons.dart').readAsStringSync();
    final entries = RegExp(
      r'static const\s+(\w+)\s*=\s*([\w.]+);',
    ).allMatches(registry).toList();
    expect(entries, isNotEmpty);
    final names = entries.map((entry) => entry.group(1)!).toSet();
    for (final entry in entries) {
      final value = entry.group(2)!;
      expect(
        value.startsWith('Icons.') || names.contains(value),
        isTrue,
        reason: '${entry.group(1)} must map to Icons.* or another entry',
      );
    }
  });

  test('common communication and commerce tokens map to Material icons', () {
    final registry = File('lib/shared/icons/app_icons.dart').readAsStringSync();
    for (final token in [
      'chatBubbleOutline',
      'shoppingCartOutlined',
      'search',
      'notificationsOutlined',
      'personOutline',
      'arrowBack',
    ]) {
      expect(
        RegExp('static const $token = Icons\\.').hasMatch(registry),
        isTrue,
        reason: '$token must map directly to a Material icon',
      );
    }
  });

  test('icon constructors never bypass semantic color tokens', () {
    final violations = <String>[];
    final constructor = RegExp(
      r'\b(?:Icon|HkIcon|ChatBubbleIcon|HawkrLocationIcon)\s*\(',
    );
    final rawColor = RegExp(
      r'(?:const\s+)?Color\(0x[0-9A-Fa-f]{8}\)|'
      r'\bColors\.[A-Za-z0-9_]+(?:\.shade\d+|\[\d+\])?|'
      r'AppColors\.[A-Za-z0-9_]+|AppTypography\.color[A-Za-z0-9_]+|'
      r'ShippingColors\.[A-Za-z0-9_]+',
    );

    for (final entity in Directory('lib').listSync(recursive: true)) {
      if (entity is! File || !entity.path.endsWith('.dart')) continue;
      final source = entity.readAsStringSync();
      for (final match in constructor.allMatches(source)) {
        final open = source.indexOf('(', match.start);
        final close = _matchingParen(source, open);
        if (close == null) continue;
        final body = source.substring(match.start, close + 1);
        if (rawColor.hasMatch(body)) {
          final line =
              '\n'.allMatches(source.substring(0, match.start)).length + 1;
          violations.add('${entity.path}:$line');
        }
      }
    }
    expect(
      violations,
      isEmpty,
      reason: 'Use AppIconColors for static icon colors: $violations',
    );
  });

  test('meaningful icon roles meet WCAG non-text contrast', () {
    const surface = AppColors.bgCard;
    for (final entry in <String, Color>{
      'default': AppIconColors.defaultColor,
      'strong': AppIconColors.strong,
      'muted': AppIconColors.muted,
      'active': AppIconColors.active,
      'secondary': AppIconColors.secondary,
      'success': AppIconColors.success,
      'warning': AppIconColors.warning,
      'danger': AppIconColors.danger,
      'info': AppIconColors.info,
    }.entries) {
      expect(
        _contrast(entry.value, surface),
        greaterThanOrEqualTo(3),
        reason: '${entry.key} must remain visible on the website card surface',
      );
    }
    expect(
      _contrast(AppIconColors.onBrand, AppColors.brandPrimaryStrong),
      greaterThanOrEqualTo(3),
    );
    expect(
      _contrast(AppIconColors.onBrand, AppColors.primary),
      greaterThanOrEqualTo(3),
    );
  });
}

int? _matchingParen(String source, int open) {
  var depth = 0;
  String? quote;
  for (var i = open; i < source.length; i++) {
    final char = source[i];
    if (quote != null) {
      if (char == r'\') {
        i++;
      } else if (char == quote) {
        quote = null;
      }
      continue;
    }
    if (char == '"' || char == "'") {
      quote = char;
    } else if (char == '(') {
      depth++;
    } else if (char == ')' && --depth == 0) {
      return i;
    }
  }
  return null;
}

double _contrast(Color a, Color b) {
  final lighter = a.computeLuminance() > b.computeLuminance() ? a : b;
  final darker = identical(lighter, a) ? b : a;
  return (lighter.computeLuminance() + 0.05) /
      (darker.computeLuminance() + 0.05);
}
