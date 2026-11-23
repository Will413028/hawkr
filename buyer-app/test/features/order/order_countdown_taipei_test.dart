import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_buyer/features/order/data/order_model.dart';

String _taipeiWall(DateTime instant) {
  final t = instant.toUtc().add(const Duration(hours: 8));
  String two(int v) => v.toString().padLeft(2, '0');
  return '${t.year}-${two(t.month)}-${two(t.day)} '
      '${two(t.hour)}:${two(t.minute)}:${two(t.second)}';
}

void main() {
  test('marker-less deadlines are Taipei wall time at any device timezone', () {
    final deadline = DateTime.now().add(const Duration(hours: 2, minutes: 30));
    final raw = _taipeiWall(deadline);
    final complete = Order(autoCompleteAt: raw).autoCompleteRemaining!;
    final payment = Order(paymentDeadline: raw).paymentRemaining!;
    for (final r in [complete, payment]) {
      expect(r.inMinutes, inInclusiveRange(148, 150));
    }
    expect(Order(autoCompleteAt: raw).autoCompleteCountdownLabel, '2 小時後自動完成');
  });
}
