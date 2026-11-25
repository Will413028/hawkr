import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:test/test.dart';

void main() {
  test('整份描述的總量是 3000，單一儲存格另有 200', () {
    expect(catalogProductContentLimitsDescriptionMaxRunes, 3000);
    expect(catalogProductContentLimitsTableCellMaxRunes, 200);
  });

  test('標題與 SKU 數的上限來自 proto 的 limits option', () {
    expect(catalogProductContentLimitsTitleMaxRunes, 100);
    expect(catalogProductContentLimitsSkusMaxItems, 100);
  });
}
