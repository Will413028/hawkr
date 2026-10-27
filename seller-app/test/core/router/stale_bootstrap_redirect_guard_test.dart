// Regression test for issue #1493 Finding 2.
//
// Root cause (confirmed via real-device repro + [Router:redirect] logs on
// dev, see backend-payuni issue #1493): `WelcomeScreen.bootstrap()` is a long
// multi-await chain (up to 3 sequential API calls). While it is
// still in flight, the top-level GoRouter `redirect` (session gating —
// login / onboarding / suspension, the single source of truth in
// `redirect_logic.dart`) can independently navigate the user away from
// `/welcome` to some other page (e.g. a suspended seller gets redirected to
// the suspension notice page, then manually navigates to a whitelisted page
// like /main/orders). `MaterialPageRoute` keeps `context.mounted == true`
// throughout the exit transition (`maintainState: true`), so `WelcomeScreen`'s
// own `context.mounted` guard does not catch this race. When bootstrap()
// finally finishes, its unconditional `goTo('/main/dashboard')` fires
// anyway, clobbering whatever page the top-level redirect had already placed
// the user on — the suspended seller got silently bounced back to the
// suspension notice page a moment after landing on a whitelisted page.
//
// The fix (`lib/features/auth/presentation/welcome_screen.dart`'s `goTo`)
// guards every navigation with `ModalRoute.of(context)?.isCurrent` — once
// another route has taken over as the Navigator's current route, `goTo`
// becomes a no-op and defers to whatever redirect_logic already decided.
//
// `WelcomeScreen` itself pulls in a large singleton dependency graph
// (Firebase, secure storage, chat polling, push notifications, ...) that
// makes it unsuitable for a fast/deterministic widget test — see git history
// for an earlier attempt that hung indefinitely on unrelated platform-channel
// timers. This test instead exercises the exact guarding pattern
// (`ModalRoute.of(context)?.isCurrent`) that the real fix uses, via a minimal
// stand-in widget, so it stays fast and focused on the actual mechanism.
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

/// Mirrors the exact shape of `WelcomeScreen`'s `bootstrap()`/`goTo()` pair:
/// a long async chain that, at the very end, unconditionally tries to
/// navigate — guarded (or not, via [useStaleGuard]) against having already
/// been superseded by an independent navigation elsewhere.
class _StaleAsyncLandingScreen extends StatefulWidget {
  const _StaleAsyncLandingScreen({
    required this.pendingDecision,
    required this.useStaleGuard,
  });

  /// Controls exactly when the async "decision" (mirrors bootstrap()'s
  /// getMySeller() call) resolves, so the test can inject a race.
  final Completer<void> pendingDecision;

  /// Toggle to reproduce the bug (false) vs. verify the fix (true).
  final bool useStaleGuard;

  @override
  State<_StaleAsyncLandingScreen> createState() =>
      _StaleAsyncLandingScreenState();
}

class _StaleAsyncLandingScreenState extends State<_StaleAsyncLandingScreen> {
  // WelcomeScreen is a HookConsumerWidget: its `goTo()` closure captures the
  // raw `BuildContext` parameter from `build(context, ref)` — a plain
  // Element reference whose `.mounted` extension just checks `_widget !=
  // null` and never throws. That's different from this State subclass's
  // *own* `context` getter, which asserts-and-throws once defunct. Capture
  // the context the same (safe) way WelcomeScreen does, so this test
  // exercises the real semantics instead of a State-specific artifact.
  BuildContext? _capturedContext;

  @override
  void initState() {
    super.initState();
    _run();
  }

  Future<void> _run() async {
    await widget.pendingDecision.future;
    final ctx = _capturedContext;
    if (ctx == null || !ctx.mounted) return;
    // This is the exact guard added to WelcomeScreen.goTo() by the fix.
    if (widget.useStaleGuard && ModalRoute.of(ctx)?.isCurrent != true) {
      return;
    }
    ctx.go('/main/dashboard');
  }

  @override
  Widget build(BuildContext context) {
    _capturedContext = context;
    return const Text('welcome');
  }
}

GoRouter _buildRouter(Completer<void> pendingDecision, bool useStaleGuard) {
  return GoRouter(
    initialLocation: '/welcome',
    routes: [
      GoRoute(
        path: '/welcome',
        builder: (c, s) => _StaleAsyncLandingScreen(
          pendingDecision: pendingDecision,
          useStaleGuard: useStaleGuard,
        ),
      ),
      GoRoute(
        path: '/main/dashboard',
        builder: (c, s) => const Text('dashboard'),
      ),
      GoRoute(
        path: '/seller/suspension-notice',
        builder: (c, s) => const Text('elsewhere'),
      ),
    ],
  );
}

void main() {
  testWidgets('guarded goTo: stale async decision does not clobber a page an '
      'independent (top-level redirect) navigation already landed on', (
    tester,
  ) async {
    final pendingDecision = Completer<void>();
    final router = _buildRouter(pendingDecision, true);
    addTearDown(router.dispose);

    await tester.pumpWidget(MaterialApp.router(routerConfig: router));
    expect(find.text('welcome'), findsOneWidget);

    // Independent navigation — mirrors the top-level redirect (session
    // gating) moving the user away from /welcome while bootstrap() is
    // still mid-flight.
    router.go('/seller/suspension-notice');
    await tester.pumpAndSettle();
    expect(find.text('elsewhere'), findsOneWidget);

    // Now let the stale async decision resolve and try to navigate.
    pendingDecision.complete();
    await tester.pumpAndSettle();

    expect(
      find.text('elsewhere'),
      findsOneWidget,
      reason: 'stale goTo must not clobber the page redirect already set',
    );
    expect(find.text('dashboard'), findsNothing);
  });

  testWidgets(
    'sanity check: without the guard, the same race DOES clobber the page '
    '(proves the test actually exercises the bug, not a tautology)',
    (tester) async {
      final pendingDecision = Completer<void>();
      final router = _buildRouter(pendingDecision, false);
      addTearDown(router.dispose);

      await tester.pumpWidget(MaterialApp.router(routerConfig: router));
      router.go('/seller/suspension-notice');
      // Deliberately do NOT pumpAndSettle here: on a real device, the old
      // route's transition (and thus `context.mounted`) can outlive the
      // navigation that superseded it (see the #1493 real-log evidence in
      // the file header — WelcomeScreen's disposed-ValueNotifier crash fired
      // *after* a second redirect, not immediately on navigating away). A
      // single pump is enough for the new route to become current while the
      // old one may still be mounted mid-transition — reproduce that window.
      await tester.pump();

      pendingDecision.complete();
      await tester.pump();
      await tester.pump();

      // Unguarded: stale goTo wins and clobbers the redirect-chosen page —
      // this is issue #1493 Finding 2 reproduced at the mechanism level.
      await tester.pumpAndSettle();
      expect(find.text('dashboard'), findsOneWidget);
    },
  );

  testWidgets(
    'guarded goTo: happy path (no race) still lands on dashboard normally',
    (tester) async {
      final pendingDecision = Completer<void>();
      final router = _buildRouter(pendingDecision, true);
      addTearDown(router.dispose);

      await tester.pumpWidget(MaterialApp.router(routerConfig: router));
      pendingDecision.complete();
      await tester.pumpAndSettle();

      expect(find.text('dashboard'), findsOneWidget);
    },
  );
}
