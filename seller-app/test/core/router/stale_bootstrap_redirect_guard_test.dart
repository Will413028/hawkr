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

    router.go('/seller/suspension-notice');
    await tester.pumpAndSettle();
    expect(find.text('elsewhere'), findsOneWidget);

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
      await tester.pump();

      pendingDecision.complete();
      await tester.pump();
      await tester.pump();

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
