import 'package:hawkr_seller/core/router/app_router.dart';
import 'package:hawkr_seller/features/notification/data/delegation_navigation.dart';
import 'package:flutter_test/flutter_test.dart';

/// 代行營運通知（56 號設計書）點下去要落在哪一頁。
///
/// 後端契約在 `services/notification-service/db/migrations/`
/// `20260816093000_delegation_template_ref_fields.sql:63-73`；這支釘的是本 App
/// 對那份契約的解讀，特別是兩個「照直覺寫就會錯」的地方：
///   ① 訂單的 `resourceId` 恆為空，單號只在 `resourceName`
///   ② `/post/feed` 吃 `postId`，缺值時停在第一篇
void main() {
  Map<String, dynamic> data(String type, {String? id, String? name}) => {
    'refType': 'DELEGATION',
    'refId': '2700000000000001',
    'resourceType': type,
    if (id != null) 'resourceId': id,
    if (name != null) 'resourceName': name,
  };

  group('order', () {
    test('用 resourceName 當單號開訂單詳情（resourceId 恆為空，不能拿來用）', () {
      final target = resolveDelegationTarget(
        data('order', id: '', name: 'O123456789'),
      );
      expect(target, isNotNull);
      expect(target!.shellPath, MainRoutes.orders);
      expect(target.pushPath, OrderRoutes.matchedDetail);
      // `orderSummaryFromRoutePayload` 讀的就是這個鍵（route_payload.dart:79）。
      expect(target.pushExtra?['orderSn'], 'O123456789');
    });

    test('沒有單號就不導航 —— 訂單詳情沒有單號會渲染成「訂單資料無效」', () {
      expect(resolveDelegationTarget(data('order', id: '')), isNull);
      expect(resolveDelegationTarget(data('order', name: '   ')), isNull);
    });
  });

  group('storefront / product', () {
    test('都落在店面頁，不再往下 push', () {
      for (final type in ['storefront', 'product']) {
        final target = resolveDelegationTarget(
          data(type, id: '2690000000000001', name: '有機蘿蔔'),
        );
        expect(target, isNotNull, reason: type);
        expect(target!.shellPath, MainRoutes.category, reason: type);
        expect(target.pushPath, isNull, reason: type);
      }
    });
  });

  group('post', () {
    test('開貼文動態，帶 resourceId 當 postId', () {
      final target = resolveDelegationTarget(
        data('post', id: '2690000000000002'),
      );
      expect(target, isNotNull);
      expect(target!.shellPath, MainRoutes.category);
      expect(target.pushPath, PostRoutes.feed);
      expect(target.pushExtra, {'postId': 2690000000000002});
    });

    test('resourceId 缺值或不是數字時不帶參數，停在第一篇', () {
      for (final id in [null, '', 'abc']) {
        final target = resolveDelegationTarget(data('post', id: id));
        expect(target!.pushPath, PostRoutes.feed, reason: '$id');
        expect(target.pushExtra, isNull, reason: '$id');
      }
    });
  });

  group('解不出目標', () {
    test('resourceType 缺席 / 空白 / 不在封閉四值內 → null', () {
      expect(resolveDelegationTarget(null), isNull);
      expect(resolveDelegationTarget(const {}), isNull);
      expect(resolveDelegationTarget({'resourceType': ''}), isNull);
      expect(resolveDelegationTarget({'resourceType': '   '}), isNull);
      expect(resolveDelegationTarget({'resourceType': 'coupon'}), isNull);
    });

    test('後端若哪天改送大寫也認得（值域宣告是小寫，但不值得為此壞掉）', () {
      final target = resolveDelegationTarget({'resourceType': 'STOREFRONT'});
      expect(target?.shellPath, MainRoutes.category);
    });
  });

  test('所有落地路徑都是 app_router 已註冊的常數，不是手打字串', () {
    const registered = {
      MainRoutes.orders,
      MainRoutes.category,
      OrderRoutes.matchedDetail,
      PostRoutes.feed,
    };
    final targets = [
      resolveDelegationTarget(data('order', name: 'O123456789')),
      resolveDelegationTarget(data('storefront')),
      resolveDelegationTarget(data('product')),
      resolveDelegationTarget(data('post')),
    ];
    for (final target in targets) {
      expect(target, isNotNull);
      expect(registered, contains(target!.shellPath));
      if (target.pushPath != null) {
        expect(registered, contains(target.pushPath));
      }
    }
  });
}
