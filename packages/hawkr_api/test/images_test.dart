import 'package:hawkr_api/images.dart';
import 'package:test/test.dart';

void main() {
  test('只把 {preset} 換成 preset 名稱', () {
    final image = Image(
      uriTemplate:
          'https://media.example.test/i/{preset}/public/7109/b1/p1.jpg',
    );
    expect(
      image.uri(ImagePreset.banner),
      'https://media.example.test/i/banner/public/7109/b1/p1.jpg',
    );
  });

  test('外部圖片的樣板沒有變數，原樣回傳', () {
    final image = Image(uriTemplate: 'https://lh3.example.test/a/photo.jpg');
    expect(
      image.uri(ImagePreset.avatar),
      'https://lh3.example.test/a/photo.jpg',
    );
  });

  test('preset 清單由 image gateway 的 presets.ts 生成', () {
    expect(ImagePreset.values.map((p) => p.name), contains('motion'));
    expect(ImagePreset.values.last, ImagePreset.original);
  });
}
