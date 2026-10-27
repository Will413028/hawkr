import 'package:hawkr_buyer/core/networking/membership_batch.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('splits unique IDs into batches of the RPC limit', () async {
    final batches = <List<int>>[];
    final matched = await checkMembershipInBatches(
      [...List.generate(250, (i) => i), 0, 1],
      (batch) async {
        batches.add(batch);
        return batch.where((id) => id.isEven);
      },
    );
    expect(batches.map((b) => b.length), [100, 100, 50]);
    expect(batches.expand((b) => b).toSet(), hasLength(250));
    expect(matched, hasLength(125));
    expect(matched.every((id) => id.isEven), isTrue);
  });

  test('an empty screen sends no request', () async {
    var calls = 0;
    final matched = await checkMembershipInBatches(<int>[], (batch) async {
      calls++;
      return batch;
    });
    expect(calls, 0);
    expect(matched, isEmpty);
  });
}
