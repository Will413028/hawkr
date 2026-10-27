import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test('Material button text uses the canonical single-line label', () {
    final violations = <String>[];
    for (final file
        in Directory('lib')
            .listSync(recursive: true)
            .whereType<File>()
            .where((file) => file.path.endsWith('.dart'))) {
      final source = file.readAsStringSync();
      for (final constructor in _buttonConstructors(source)) {
        for (final argument in _rootArguments(constructor.body)) {
          if (argument.name != 'child' && argument.name != 'label') continue;
          if (!RegExp(r'\bText(?:\.rich)?\s*\(').hasMatch(argument.value)) {
            continue;
          }
          final line =
              '\n'.allMatches(source.substring(0, constructor.offset)).length +
              1;
          violations.add('${file.path}:$line ${argument.name}');
        }
      }
    }

    expect(
      violations,
      isEmpty,
      reason:
          'Text-bearing Material buttons must use HkButtonLabel so labels '
          'stay on one line, ellipsize safely, and retain their full semantics: '
          '$violations',
    );
  });
}

final class _ConstructorSlice {
  const _ConstructorSlice({required this.offset, required this.body});

  final int offset;
  final String body;
}

final class _NamedArgument {
  const _NamedArgument(this.name, this.value);

  final String name;
  final String value;
}

Iterable<_ConstructorSlice> _buttonConstructors(String source) sync* {
  final button = RegExp(
    r'\b(?:FilledButton|ElevatedButton|OutlinedButton|TextButton)'
    r'(?:\.icon|\.tonal|\.tonalIcon)?\s*\(',
  );
  for (final match in button.allMatches(source)) {
    final open = match.end - 1;
    final close = _matchingParenthesis(source, open);
    if (close == null) continue;
    yield _ConstructorSlice(
      offset: match.start,
      body: source.substring(open + 1, close),
    );
  }
}

Iterable<_NamedArgument> _rootArguments(String body) sync* {
  var start = 0;
  var round = 0;
  var square = 0;
  var curly = 0;
  String? quote;

  for (var index = 0; index <= body.length; index++) {
    final atEnd = index == body.length;
    final char = atEnd ? ',' : body[index];
    if (!atEnd && quote != null) {
      if (char == r'\') {
        index++;
      } else if (char == quote) {
        quote = null;
      }
      continue;
    }
    if (!atEnd && (char == "'" || char == '"')) {
      quote = char;
      continue;
    }
    if (!atEnd && char == '/' && index + 1 < body.length) {
      if (body[index + 1] == '/') {
        final newline = body.indexOf('\n', index + 2);
        index = newline < 0 ? body.length : newline;
        continue;
      }
      if (body[index + 1] == '*') {
        final close = body.indexOf('*/', index + 2);
        index = close < 0 ? body.length : close + 1;
        continue;
      }
    }
    switch (char) {
      case '(':
        round++;
      case ')':
        round--;
      case '[':
        square++;
      case ']':
        square--;
      case '{':
        curly++;
      case '}':
        curly--;
    }
    if (char != ',' || round != 0 || square != 0 || curly != 0) continue;

    final argument = body.substring(start, index).trim();
    start = index + 1;
    final separator = argument.indexOf(':');
    if (separator <= 0) continue;
    final name = argument.substring(0, separator).trim();
    if (!RegExp(r'^[A-Za-z_]\w*$').hasMatch(name)) continue;
    yield _NamedArgument(name, argument.substring(separator + 1).trim());
  }
}

int? _matchingParenthesis(String source, int open) {
  var depth = 0;
  String? quote;
  for (var index = open; index < source.length; index++) {
    final char = source[index];
    if (quote != null) {
      if (char == r'\') {
        index++;
      } else if (char == quote) {
        quote = null;
      }
      continue;
    }
    if (char == "'" || char == '"') {
      quote = char;
      continue;
    }
    if (char == '/' && index + 1 < source.length) {
      if (source[index + 1] == '/') {
        final newline = source.indexOf('\n', index + 2);
        index = newline < 0 ? source.length : newline;
        continue;
      }
      if (source[index + 1] == '*') {
        final close = source.indexOf('*/', index + 2);
        index = close < 0 ? source.length : close + 1;
        continue;
      }
    }
    if (char == '(') depth++;
    if (char == ')' && --depth == 0) return index;
  }
  return null;
}
