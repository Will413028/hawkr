import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_seller/features/seller/data/measure_display.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';

void main() {
  test('CONTENT_TAKEDOWN renders the catalog noun in the meta line', () {
    final measure = Measure(kind: MeasureKind.MEASURE_KIND_CONTENT_TAKEDOWN);
    expect(measureMeta(measure, UiLocale.zhTW), '措施：內容下架');
    expect(measureMeta(measure, UiLocale.en), 'Measure: Content removal');
  });
}
