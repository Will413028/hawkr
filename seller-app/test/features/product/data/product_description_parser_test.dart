import 'package:hawkr_seller/features/product/data/product_description_parser.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('parseProductDescription', () {
    test('returns plain text when description is a string', () {
      final parsed = parseProductDescription('  新鮮好吃  ');

      expect(parsed.text, '新鮮好吃');
      expect(parsed.blocks, isEmpty);
    });

    test('extracts text and blocks from structured description', () {
      final parsed = parseProductDescription({
        'version': '1.0',
        'blocks': [
          {'type': 'text', 'content': '第一段'},
          {'type': 'image', 'url': 'https://example.com/a.jpg'},
          {'type': 'text', 'text': '第二段'},
        ],
      });

      expect(parsed.text, '第一段\n第二段');
      expect(parsed.blocks, hasLength(3));
    });

    test('does not stringify empty maps into braces', () {
      final parsed = parseProductDescription({});

      expect(parsed.text, isEmpty);
      expect(parsed.blocks, isEmpty);
    });
  });
}
