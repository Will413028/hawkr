import 'package:hawkr_seller/features/report/data/block_model.dart';
import 'package:flutter_test/flutter_test.dart';

/// `GET /api/v1/app/blocks` 對 dev 的實打回應（2026-08-06，封鎖 → 查詢 → 解封，
/// 逐字貼上；驗完已還原）。
///
/// gateway `ListBlocks` 把 proto `UserBlock` 直接放進 `records`，所以 wire 用的是
/// protoc-gen-go 的 snake_case tag，且**只有三個欄位**。
const Map<String, dynamic> _liveBlock = {
  'id': '7490877555104607313',
  'blocked_id': '2662192214',
  'create_time_ms': 1785964382915,
};

/// 只含「解析層曾經猜過、但後端從未回過」的 key。
const Map<String, dynamic> _deadKeysOnly = {
  'user_id': '9999999999',
  'userId': '9999999999',
  'blocked_user_id': '9999999999',
  'blockedUserId': '9999999999',
  'nickname': '不該被讀到的名字',
  'nick_name': '不該被讀到的名字',
  'user_name': '不該被讀到的名字',
  'userName': '不該被讀到的名字',
  'name': '不該被讀到的名字',
  'avatarUrl': 'https://dead.example.test/a.png',
  'avatar': 'https://dead.example.test/b.png',
  'createTimeMs': 1721800000000,
  'create_time': 1721800000000,
  'createTime': 1721800000000,
};

void main() {
  group('BlockedUser.fromRaw 只認 UserBlock proto 的欄位', () {
    test('dev 實打 payload 綁到 blocked_id，不是紀錄自己的 TSID', () {
      final u = BlockedUser.fromRaw(_liveBlock);

      expect(
        u.userId,
        '2662192214',
        reason: '綁到 id 會讓「解除封鎖」打錯 id、後端 delete-by-pair 找不到列而變 no-op',
      );
      expect(u.createTimeMs, 1785964382915);
      expect(u.displayLabel, '使用者 192214');
    });

    test('只有 id 欄位時 userId 維持空字串', () {
      final u = BlockedUser.fromRaw(const {'id': '7490877555104607313'});

      expect(u.userId, '');
    });

    test('十四個舊候選 key 一個都讀不到——收斂後沒有相容層', () {
      final u = BlockedUser.fromRaw(_deadKeysOnly);

      expect(u.userId, '');
      expect(u.createTimeMs, 0);
    });

    test('create_time_ms 是字串時仍解析得出來', () {
      final u = BlockedUser.fromRaw(const {
        'blocked_id': '999',
        'create_time_ms': '1721800000000',
      });

      expect(u.userId, '999');
      expect(u.createTimeMs, 1721800000000);
      expect(u.displayLabel, '使用者 999');
    });
  });
}
