import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:test/test.dart';

void main() {
  test('整份描述的文字總量是 3000、圖片 20，單一儲存格另有 200', () {
    expect(catalogProductInputDescriptionMaxTotalRunes, 3000);
    expect(catalogProductInputDescriptionMaxImageItems, 20);
    expect(catalogTableRowCellsMaxRunes, 200);
  });

  test('標題與 SKU 數的上限來自商品寫入訊息欄位上的 limits option', () {
    expect(catalogProductInputTitleMaxRunes, 100);
    expect(catalogProductInputSkusMaxItems, 100);
  });
}
