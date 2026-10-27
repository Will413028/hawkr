import 'package:flutter_riverpod/flutter_riverpod.dart';

enum AuthLogLevel { warning, error }

typedef AuthLogger =
    void Function(
      AuthLogLevel level,
      String message,
      Object error,
      StackTrace stack,
    );

void discardAuthLog(
  AuthLogLevel level,
  String message,
  Object error,
  StackTrace stack,
) {}

final authLoggerProvider = Provider<AuthLogger>((ref) => discardAuthLog);
