import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/core/extensions/url_extensions.dart';

/// media 域名基底跟隨當前 flavor（normalizeAvatarUrl 對 R2 裸 key 走 buildImageUrl，
/// 於 runtime 讀 F.appFlavor.mediaBaseHost）。測試以 dev flavor 初始化 → 本機單體（沒有 MEDIA_BASE_HOST define）。
const String _mediaBase = 'http://localhost:8080';

void main() {
  setUpAll(() {
    if (!F.isInitialized) F.init(Flavor.dev);
  });

  group('normalizeAvatarUrl 通用', () {
    test('空字串回空字串', () {
      expect(''.normalizeAvatarUrl(), '');
      expect('   '.normalizeAvatarUrl(), '');
    });

    test('http/https passthrough（含外部 Google 頭像）', () {
      const g = 'https://lh3.googleusercontent.com/a/abc';
      expect(g.normalizeAvatarUrl(), g);
      const media = 'https://media.example.com/avatars/x.png';
      expect(media.normalizeAvatarUrl(), media);
    });

    test('未知格式（含已移除的 OSS legacy 前綴）回原值 fallback', () {
      // OSS legacy 分支已移除（#1675 dead code）；這些輸入現在一律回原值。
      const legacy = 'avatars/merchant/abc.png';
      expect(legacy.normalizeAvatarUrl(), legacy);
      const member = 'member-avatars/x.png';
      expect(member.normalizeAvatarUrl(), member);
      const unknown = 'some/random/key.png';
      expect(unknown.normalizeAvatarUrl(), unknown);
    });
  });

  group('normalizeAvatarUrl R2 裸 objectKey（F2 修復）', () {
    const key = 'public/buyer_avatar/2623900520/20260719/76a47987.png';

    test('R2 裸 key → 組 /i/avatar/{key}（base 跟隨 flavor）', () {
      final full = key.normalizeAvatarUrl();
      expect(full, '$_mediaBase/i/avatar/$key');
      expect(full.startsWith('http'), isTrue);
    });
  });
}
