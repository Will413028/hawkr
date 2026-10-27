import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_buyer/features/moderation/data/block_model.dart';

/// `GET /app/blocks` 對 dev 的實打回應（2026-08-06，買家 2623900520 封鎖
/// 2662192214 後查詢，逐字貼上；驗完已解封還原）。
///
/// gateway `ListBlocks` 把 proto `UserBlock` 直接放進 `records`，所以 wire 用的是
/// protoc-gen-go 的 snake_case tag，且**只有三個欄位**。
const Map<String, dynamic> _liveBlock = {
  'id': '7490877555104607313',
  'blocked_id': '2662192214',
  'create_time_ms': 1785964382915,
};

/// 只含「model 曾經猜過、但後端從未回過」的 key。解析器碰到這份資料必須回空值
/// ——讀得出東西代表那些死候選又長回來了。
const Map<String, dynamic> _deadKeysOnly = {
  'userId': '9999999999',
  'user_id': '9999999999',
  'blockedId': '9999999999',
  'blockedUserId': '9999999999',
  'blocked_user_id': '9999999999',
  'targetUserId': '9999999999',
  'target_user_id': '9999999999',
  'accountId': '9999999999',
  'account_id': '9999999999',
  'createTimeMs': 1721800000000,
  'createdAt': 1721800000000,
  'created_at': 1721800000000,
  'username': '不該被讀到的名字',
  'nickname': '不該被讀到的名字',
  'avatarUrl': 'https://example.com/should-not-read.png',
};

void main() {
  group('BlockedUser.fromJson 只認 UserBlock proto 的三個欄位', () {
    test('dev 實打 payload 解析出被封鎖者 id 與建立時間', () {
      final u = BlockedUser.fromJson(_liveBlock);
      expect(
        u.userId,
        '2662192214',
        reason: 'userId 必須綁 blocked_id（被封鎖者），不是 id（封鎖紀錄本身的 TSID）',
      );
      expect(u.createTimeMs, 1785964382915);
    });

    test('只有 id 欄位（沒有 blocked_id）時 userId 維持空字串，不誤綁封鎖紀錄自己的 TSID', () {
      final u = BlockedUser.fromJson(const {'id': '42'});
      expect(
        u.userId,
        '',
        reason: 'id 是封鎖紀錄的 TSID 不是使用者 id，誤綁會讓解除封鎖打錯 id 變成 no-op',
      );
    });

    test('十五個舊候選 key 一個都讀不到——收斂後沒有相容層', () {
      final u = BlockedUser.fromJson(_deadKeysOnly);
      expect(u.userId, '');
      expect(u.createTimeMs, isNull);
    });

    test('create_time_ms 是字串時仍解析得出來', () {
      final u = BlockedUser.fromJson(const {
        'blocked_id': '999',
        'create_time_ms': '1721800000000',
      });
      expect(u.userId, '999');
      expect(u.createTimeMs, 1721800000000);
    });

    test('完全空物件 userId 維持空字串、createTimeMs 為 null', () {
      final u = BlockedUser.fromJson(const {});
      expect(u.userId, '');
      expect(u.createTimeMs, isNull);
    });
  });
}
