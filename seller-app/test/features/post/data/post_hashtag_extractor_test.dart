import 'package:hawkr_seller/features/post/data/post_hashtag_extractor.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('PostHashtagExtractor', () {
    test('extracts Chinese, ASCII letters, and digits in order', () {
      final tags = PostHashtagExtractor.extractFromText(
        '今天很適合 #美食 #Apple123 #宅配',
      );

      expect(tags, ['美食', 'Apple123', '宅配']);
    });

    test('deduplicates case-insensitive ASCII tags', () {
      final tags = PostHashtagExtractor.extractFromText(
        '#Apple #apple #APPLE #美食',
      );

      expect(tags, ['Apple', '美食']);
    });

    test('limits results to ten tags', () {
      final tags = PostHashtagExtractor.extractFromText(
        List.generate(12, (index) => '#tag$index').join(' '),
      );

      expect(tags.length, 10);
      expect(tags.last, 'tag9');
    });

    test('stops hashtag body at unsupported characters', () {
      final tags = PostHashtagExtractor.extractFromText(
        '#abc_def #美食🍎 #good!',
      );

      expect(tags, ['abc', '美食', 'good']);
    });

    test('detects tagging mode only after a valid standalone hash', () {
      expect(PostHashtagExtractor.detectTaggingMode('今天 #美')?.prefix, '美');
      expect(PostHashtagExtractor.detectTaggingMode('今天#美'), isNull);
      expect(PostHashtagExtractor.detectTaggingMode('今天 #'), isNull);
    });
  });
}
