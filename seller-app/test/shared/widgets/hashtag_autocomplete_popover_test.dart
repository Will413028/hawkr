import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_seller/shared/widgets/hashtag_autocomplete_popover.dart';

void main() {
  testWidgets('older failing request does not clear newer results', (
    tester,
  ) async {
    final older = Completer<List<HashtagAutocompleteItem>>();
    final newer = Completer<List<HashtagAutocompleteItem>>();
    Future<List<HashtagAutocompleteItem>> search(String p) =>
        p == 'a' ? older.future : newer.future;

    Widget build(String prefix) => ProviderScope(
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        builder: (_, _) => MaterialApp(
          home: Scaffold(
            body: HashtagAutocompletePopover(
              prefix: prefix,
              onSelect: (_) {},
              search: search,
            ),
          ),
        ),
      ),
    );

    await tester.pumpWidget(build('a'));
    await tester.pumpWidget(build('ab'));
    await tester.pump(const Duration(milliseconds: 400));

    newer.complete([const HashtagAutocompleteItem(name: 'newer')]);
    await tester.pump();
    expect(find.textContaining('newer'), findsWidgets);

    older.completeError(Exception('slow failure'));
    await tester.pump();
    expect(find.textContaining('newer'), findsWidgets);
  });
}
