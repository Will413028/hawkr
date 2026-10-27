import 'package:hawkr_buyer/config/app_constants.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('AppConstants.shareFallbackUrl', () {
    test('is disabled when no public site origin is configured', () {
      expect(AppConstants.publicSiteOrigin, '');
      expect(AppConstants.shareFallbackUrl('/products/1'), isNull);
      expect(AppConstants.shareFallbackUrl('/products/1', origin: ' '), isNull);
    });

    test('builds the link from the configured origin only', () {
      expect(
        AppConstants.shareFallbackUrl(
          '/sellers/7/posts#post-9',
          origin: 'https://web.example.test/',
        ),
        'https://web.example.test/sellers/7/posts#post-9',
      );
    });
  });
}
