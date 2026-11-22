import 'dart:async';

import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_seller/features/coupon/presentation/tag_audience_toggle.dart';

void main() {
  late Map<String, Completer<List<String>>> pending;
  late Set<String> selected;
  late List<String> customers;
  late bool loading;
  late List<Object> errors;
  late TagAudienceToggle<String> toggle;

  setUp(() {
    pending = {};
    selected = {};
    customers = [];
    loading = false;
    errors = [];
    toggle = TagAudienceToggle<String>(
      fetch: (id) => (pending[id] = Completer<List<String>>()).future,
      setSelected: (v) => selected = v,
      setCustomers: (v) => customers = v,
      setLoading: (v) => loading = v,
      onError: errors.add,
    );
  });

  test(
    'a late result for a tag that is no longer selected is dropped',
    () async {
      final a = toggle.toggle('a', selected);
      final b = toggle.toggle('b', selected);
      pending['b']!.complete(['from-b']);
      await b;
      pending['a']!.complete(['from-a']);
      await a;
      expect(selected, {'b'});
      expect(customers, ['from-b']);
      expect(loading, isFalse);
    },
  );

  test('deselecting while loading drops the result', () async {
    final a = toggle.toggle('a', selected);
    await toggle.toggle('a', selected);
    pending['a']!.complete(['x']);
    await a;
    expect(selected, isEmpty);
    expect(customers, isEmpty);
    expect(loading, isFalse);
  });

  test('a late error of a superseded tag is not reported', () async {
    final a = toggle.toggle('a', selected);
    final b = toggle.toggle('b', selected);
    pending['a']!.completeError(StateError('boom'));
    await a;
    expect(errors, isEmpty);
    expect(loading, isTrue);
    pending['b']!.complete(['b1']);
    await b;
    expect(customers, ['b1']);
  });

  test('an error of the current tag clears the list and reports', () async {
    final a = toggle.toggle('a', selected);
    pending['a']!.completeError(StateError('boom'));
    await a;
    expect(errors, hasLength(1));
    expect(customers, isEmpty);
    expect(loading, isFalse);
  });
}
