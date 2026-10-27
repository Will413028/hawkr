import 'package:hawkr_buyer/core/routing/market_route.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('canonicalMarketLocation', () {
    test('moves the legacy market route to tab 1 and maps its old labels', () {
      final location = canonicalMarketLocation(
        Uri.parse(
          '/marketplace?sort=${Uri.encodeQueryComponent('最新上架')}'
          '&keyword=${Uri.encodeQueryComponent(' 虱目魚 ')}&campaign=fall',
        ),
      );
      final uri = Uri.parse(location);

      expect(uri.path, '/');
      expect(uri.queryParameters, {
        'sort': 'LATEST',
        'q': '虱目魚',
        'campaign': 'fall',
        'tab': '1',
      });
    });

    test('drops invalid or default sort while preserving repeated filters', () {
      final location = canonicalMarketLocation(
        Uri.parse('/category?sort=unknown&hashtag=a&hashtag=b&tab=4'),
      );
      final uri = Uri.parse(location);

      expect(uri.queryParameters['sort'], isNull);
      expect(uri.queryParameters['tab'], '1');
      expect(uri.queryParametersAll['hashtag'], ['a', 'b']);
    });
  });

  group('MarketRouteIntent', () {
    test('reads canonical keyword and backend sort values', () {
      final intent = MarketRouteIntent.fromUri(
        Uri.parse('/?tab=1&q=%20高麗菜%20&sort=price-low'),
      );

      expect(intent.keyword, '高麗菜');
      expect(intent.sort, 'PRICE_ASC');
    });

    test('uses recommended defaults for unsupported values', () {
      final intent = MarketRouteIntent.fromUri(
        Uri.parse('/?tab=1&sort=%E7%B6%9C%E5%90%88%E6%8E%A8%E8%96%A6'),
      );

      expect(intent.keyword, isEmpty);
      expect(intent.sort, isEmpty);
    });
  });
}
