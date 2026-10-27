import 'package:hawkr_seller/features/photo_editor/data/ai_style_model.dart';
import 'package:flutter_test/flutter_test.dart';

/// `GET /api/v1/app/ai_styles` 對 dev 的實打回應（2026-08-06，賣家 2685914682，
/// 取第一筆逐字貼上，prompt 內文以省略號代替）。
///
/// 縮圖是 49 §6.1 成對的 `thumbnailKey`（裸 objectKey，SSOT）+ `thumbnailUrl`
/// （Gateway 以 MEDIA_BASE_URL 組的 `/i/original/` URL），欄名以 gateway
/// `AiStyleItem`（backend-payuni `gateway/internal/handler/ai.go:22-32`）為準。
const Map<String, dynamic> _liveStyle = {
  'id': '1',
  'name': 'natural',
  'label': '新鮮自然',
  'prompt': '為上傳的農產品照片進行自然清新修圖…',
  'thumbnailKey':
      'public/ai_style/2613330734/20260410/19236cc7-66ef-43d9-884d-544066a84920.png',
  'thumbnailUrl':
      'https://test-media.example.com/i/original/'
      'public/ai_style/2613330734/20260410/19236cc7-66ef-43d9-884d-544066a84920.png',
  'sortOrder': '1',
};

void main() {
  group('AiStyle 縮圖只認 thumbnailKey', () {
    test('dev 實打回應解析出裸 objectKey', () {
      final style = AiStyle.fromJson(_liveStyle);

      expect(style.id, '1');
      expect(style.label, '新鮮自然');
      expect(
        style.thumbnailKey,
        'public/ai_style/2613330734/20260410/'
        '19236cc7-66ef-43d9-884d-544066a84920.png',
      );
      expect(style.sortOrder, '1');
    });

    // 讀 URL 等於每張 100×100 的縮圖都去拉 /i/original/ 原圖；顯示端帶 scene
    // 走 buildImageUrl 才會套縮圖 preset，所以這裡刻意不吃 thumbnailUrl。
    test('只給 thumbnailUrl（以及其他舊猜測欄名）時 thumbnailKey 維持空字串', () {
      final style = AiStyle.fromJson(const {
        'id': '2',
        'name': 'x',
        'label': 'x',
        'prompt': 'x',
        'thumbnailUrl': 'https://dead.example.test/original.png',
        'thumbnail': 'dead/1.png',
        'thumbnail_key': 'dead/2.png',
        'imageUrl': 'https://dead.example.test/2.png',
        'previewUrl': 'https://dead.example.test/3.png',
      });

      expect(style.thumbnailKey, '');
    });

    test('缺 thumbnailKey / sortOrder 時走預設值', () {
      final style = AiStyle.fromJson(const {
        'id': '3',
        'name': 'x',
        'label': 'x',
        'prompt': 'x',
      });

      expect(style.thumbnailKey, '');
      expect(style.sortOrder, '0');
    });
  });
}
