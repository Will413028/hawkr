const int marketTabIndex = 1;

const String marketKeywordQuery = 'q';
const String marketSortQuery = 'sort';

const Set<String> _marketSortValues = {
  'LATEST',
  'SALES',
  'PRICE_ASC',
  'PRICE_DESC',
  'RATING',
};

const Map<String, String> _marketSortAliases = {
  '最新上架': 'LATEST',
  '銷量最高': 'SALES',
  '價格低到高': 'PRICE_ASC',
  '價格高到低': 'PRICE_DESC',
  '評分最高': 'RATING',
  'LATEST': 'LATEST',
  'SALES': 'SALES',
  'PRICELOW': 'PRICE_ASC',
  'PRICEHIGH': 'PRICE_DESC',
  'PRICE_ASC': 'PRICE_ASC',
  'PRICE_DESC': 'PRICE_DESC',
  'RATING': 'RATING',
};

/// The small, shareable query contract owned by the canonical market tab.
class MarketRouteIntent {
  const MarketRouteIntent({this.keyword = '', this.sort = ''});

  factory MarketRouteIntent.fromUri(Uri uri) {
    final query = uri.queryParameters;
    return MarketRouteIntent(
      keyword: normalizeMarketKeyword(
        query[marketKeywordQuery] ?? query['keyword'],
      ),
      sort: normalizeMarketSort(query[marketSortQuery]),
    );
  }

  final String keyword;
  final String sort;
}

String normalizeMarketKeyword(String? value) => value?.trim() ?? '';

/// Accepts both the legacy Chinese labels and the backend sort values.
/// An empty result means the default recommended order.
String normalizeMarketSort(String? value) {
  final trimmed = value?.trim() ?? '';
  if (trimmed.isEmpty || trimmed == '推薦' || trimmed == '綜合推薦') {
    return '';
  }

  final upper = trimmed.toUpperCase();
  if (_marketSortValues.contains(upper)) return upper;
  return _marketSortAliases[trimmed] ??
      _marketSortAliases[upper.replaceAll(RegExp(r'[-_\s]'), '')] ??
      '';
}

/// Converts retired market/category URLs into the root tab without dropping
/// unrelated filter or campaign query parameters.
String canonicalMarketLocation(Uri source) {
  final query = <String, List<String>>{
    for (final entry in source.queryParametersAll.entries)
      entry.key: List<String>.of(entry.value),
  };

  query['tab'] = const ['$marketTabIndex'];

  final keyword = normalizeMarketKeyword(
    source.queryParameters[marketKeywordQuery] ??
        source.queryParameters['keyword'],
  );
  query.remove('keyword');
  if (keyword.isEmpty) {
    query.remove(marketKeywordQuery);
  } else {
    query[marketKeywordQuery] = [keyword];
  }

  final sort = normalizeMarketSort(source.queryParameters[marketSortQuery]);
  if (sort.isEmpty) {
    query.remove(marketSortQuery);
  } else {
    query[marketSortQuery] = [sort];
  }

  return Uri(path: '/', queryParameters: query).toString();
}
