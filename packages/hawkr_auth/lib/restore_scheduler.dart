import 'dart:async';

class RestoreScheduler {
  Timer? _timer;
  int _generation = 0;

  int cancel() {
    _timer?.cancel();
    return ++_generation;
  }

  bool isCurrent(int generation) => generation == _generation;

  void retry<T>({
    required Future<T> Function() request,
    required Future<void> Function(T value) onSuccess,
    required bool Function(Object error) isDenied,
    required Future<void> Function(Object error) onDenied,
    required void Function(Object error) onUnavailable,
  }) {
    final generation = cancel();
    void schedule(int attempt) {
      if (attempt == 3 || !isCurrent(generation)) return;
      _timer = Timer(Duration(milliseconds: 1500 * (1 << attempt)), () async {
        final T value;
        try {
          value = await request();
        } catch (error) {
          if (!isCurrent(generation)) return;
          if (isDenied(error)) {
            try {
              await onDenied(error);
            } catch (failure) {
              onUnavailable(failure);
            }
          } else {
            onUnavailable(error);
            schedule(attempt + 1);
          }
          return;
        }
        if (!isCurrent(generation)) return;
        try {
          await onSuccess(value);
        } catch (error) {
          onUnavailable(error);
        }
      });
    }

    schedule(0);
  }
}
