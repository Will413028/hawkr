import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/core/analytics/analytics_providers.dart';

void main() {
  test('analytics enabled in prod and dev, disabled in stage', () {
    expect(analyticsEnabledForFlavor(Flavor.prod), isTrue);
    expect(analyticsEnabledForFlavor(Flavor.dev), isTrue);
    expect(analyticsEnabledForFlavor(Flavor.stage), isFalse);
  });
}
