import 'package:hawkr_buyer/core/session/session_model.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hawkr_buyer/features/wallet/data/wallet_remote_service.dart';
import 'package:hawkr_buyer/features/wallet/data/wallet_repository.dart';
import 'package:hawkr_buyer/features/wallet/presentation/wallet_controller.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

/// `GET /app/coupons` 一筆券的 wire 形狀。
///
/// 欄名以 gateway `templateInfoToMap`
/// （backend-payuni `gateway/internal/handler/coupon.go:1177-1222`）為準：
/// 全 camelCase 單一鍵，沒有 snake 別名。
const Map<String, dynamic> _liveTemplateCoupon = {
  'templateId': '7490877555104607313',
  'title': '夏日折價券',
  'type': '1',
  'targetType': '1',
  'applicableStoreId': '2685914682',
  'sellerName': '陳記',
  'storeCode': '2685914682',
  'logoKey': 'public/seller_logo/2685914682/20260805/3f47a6bf.png',
  'logoUrl':
      'https://test-media.example.com/i/original/'
      'public/seller_logo/2685914682/20260805/3f47a6bf.png',
  'thresholdAmount': '300',
  'discountAmount': '50',
  'discountPercent': '0',
  'maxDiscountAmount': '0',
  'startAt': '2026-08-01T00:00:00Z',
  'endAt': '2026-08-31T00:00:00Z',
  'usageLimitPerAccount': '1',
  'status': '1',
  'createdAt': '2026-08-01T00:00:00Z',
};

/// 只含「解析層曾經猜過、但後端從未回過」的 key。任何欄位讀得出東西，
/// 代表那個死候選又長回來了。
const Map<String, dynamic> _deadKeysOnly = {
  'id': 7,
  'couponId': 7,
  'coupon_id': 7,
  'name': '不該被讀到的名字',
  'discount': 999,
  'minAmount': 999,
  'min_amount': 999,
  'discount_amount': 999,
  'discount_percent': 999,
  'threshold_amount': 999,
  'usage_limit_per_account': 999,
  'expiredAt': '2026-12-31T00:00:00Z',
  'expired_at': '2026-12-31T00:00:00Z',
  'usedAt': '2026-07-01T00:00:00Z',
  'used_at': '2026-07-01T00:00:00Z',
  'used': '2026-07-01T00:00:00Z',
  'end_at': '2026-12-31T00:00:00Z',
  'expire_date': '2026-12-31T00:00:00Z',
  'created_at': '2026-01-01T00:00:00Z',
  'seller_name': '不該被讀到的商家',
  'logo_url': 'https://dead.example.test/1.png',
  'sellerLogoUrl': 'https://dead.example.test/2.png',
  'seller_logo_url': 'https://dead.example.test/3.png',
  'sellerAvatarUrl': 'https://dead.example.test/4.png',
  'sellerAvatar': 'https://dead.example.test/5.png',
  'issuerAvatarUrl': 'https://dead.example.test/6.png',
  'seller_avatar_url': 'https://dead.example.test/7.png',
  'ruleType': 'amount_off',
  'rule_type': 'amount_off',
  'conditionType': 'min_purchase',
  'condition_type': 'min_purchase',
  'discountRate': 0.85,
  'discount_rate': 0.85,
};

void main() {
  test('repository parses the live /app/coupons template shape', () async {
    final repository = WalletRepository(
      _FakeWalletRemoteService({
        'code': 0,
        'data': {
          'records': [_liveTemplateCoupon],
        },
      }),
    );

    final coupons = await repository.getMyCoupons();

    expect(coupons.single.id, '7490877555104607313');
    expect(coupons.single.name, '夏日折價券');
    expect(coupons.single.discountAmount, 50);
    expect(coupons.single.thresholdAmount, 300);
    expect(coupons.single.usageLimitPerAccount, 1);
    expect(coupons.single.endAt, '2026-08-31T00:00:00Z');
    expect(coupons.single.sellerName, '陳記');
    expect(coupons.single.storeId, '2685914682');
    // 49 §6.1：裸 key 優先，顯示端帶 scene 過 buildImageUrl。
    expect(
      coupons.single.sellerAvatarUrl,
      'public/seller_logo/2685914682/20260805/3f47a6bf.png',
    );
  });

  test('已使用券用 instanceId 當 id（列表 ValueKey 不會互撞）', () async {
    final repository = WalletRepository(
      _FakeWalletRemoteService({
        'code': 0,
        'data': {
          'records': [
            {..._liveTemplateCoupon, 'instanceId': '7490877555104607999'},
          ],
        },
      }),
    );

    final coupons = await repository.getMyCoupons();

    expect(coupons.single.id, '7490877555104607999');
  });

  test('三十幾個舊候選 key 一個都讀不到——收斂後沒有相容層', () async {
    final repository = WalletRepository(
      _FakeWalletRemoteService({
        'code': 0,
        'data': {
          'records': [_deadKeysOnly],
        },
      }),
    );

    final coupon = (await repository.getMyCoupons()).single;

    expect(coupon.id, '', reason: 'id / couponId / coupon_id 都不是契約欄位');
    expect(coupon.name, '優惠券', reason: '標題只認 title，不認 name');
    expect(coupon.discountAmount, isNull);
    expect(coupon.discountPercent, isNull);
    expect(coupon.thresholdAmount, isNull);
    expect(coupon.usageLimitPerAccount, isNull);
    expect(coupon.endAt, '');
    expect(coupon.createdAt, '');
    expect(coupon.sellerName, '');
    expect(coupon.sellerAvatarUrl, '');
  });

  test(
    'coupon controller returns immutable display-ready card state',
    () async {
      final service = _FakeWalletRemoteService([
        {
          'templateId': '7490877555104607313',
          'title': '會員券',
          'discountPercent': '0.85',
          'thresholdAmount': '200',
        },
      ]);
      final container = ProviderContainer(
        overrides: [
          walletRemoteServiceProvider.overrideWithValue(service),
          sessionProvider.overrideWith(_ActiveSession.new),
        ],
      );
      addTearDown(container.dispose);
      await container.read(sessionProvider.future);

      final cards = await container.read(
        couponControllerProvider(status: 1).future,
      );

      expect(service.requestedStatuses, [1]);
      expect(cards.single.id, '7490877555104607313');
      expect(cards.single.title, '會員券');
      expect(cards.single.content, contains('8.5折'));
      expect(cards.single.ruleDescription, contains('使用門檻：滿 NT\$200'));
      expect(() => cards.add(cards.single), throwsUnsupportedError);
    },
  );

  test('無有效 session（訪客／已登出）不打 /app/coupons，直接回空清單', () async {
    final service = _FakeWalletRemoteService([]);
    final container = ProviderContainer(
      overrides: [
        walletRemoteServiceProvider.overrideWithValue(service),
        sessionProvider.overrideWith(_GuestSession.new),
      ],
    );
    addTearDown(container.dispose);
    await container.read(sessionProvider.future);

    final cards = await container.read(
      couponControllerProvider(status: 0).future,
    );

    expect(cards, isEmpty);
    expect(
      service.requestedStatuses,
      isEmpty,
      reason: '訪客／已登出不該打 API，這是 401 重試風暴的成因',
    );
  });
}

class _ActiveSession extends Session {
  @override
  Future<SessionModel?> build() async =>
      const SessionModel(userId: 'buyer-1', nickname: 'Buyer');
}

class _GuestSession extends Session {
  @override
  Future<SessionModel?> build() async => null;
}

final class _FakeWalletRemoteService implements WalletRemoteService {
  _FakeWalletRemoteService(this.couponBody);

  final dynamic couponBody;
  final List<int> requestedStatuses = [];

  @override
  Future<dynamic> fetchCoupons({required int status}) async {
    requestedStatuses.add(status);
    return couponBody;
  }

  @override
  Future<Map<String, dynamic>> fetchCouponTemplates({
    required int page,
    required int pageSize,
  }) async => const {};

  @override
  Future<Map<String, dynamic>> receiveCoupon(String templateId) async =>
      const {};
}
