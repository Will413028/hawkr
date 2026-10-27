const String redactedSecret = '<redacted>';

final RegExp _bearerPattern = RegExp(
  r'\bBearer\s+[A-Za-z0-9._~+/=-]+',
  caseSensitive: false,
);
final RegExp _jwtPattern = RegExp(
  r'\beyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\b',
);
final RegExp _secretAssignmentPattern = RegExp(
  r'''(["']?(?:authorization|token|access[_-]?token|refresh[_-]?token|jwt[_-]?token|fcm[_-]?token|device[_-]?token|password|secret|cookie)["']?\s*[:=]\s*["']?)([^,\s}\]"']+)''',
  caseSensitive: false,
);

/// Returns a copy suitable for logs and telemetry.
///
/// Secret-like map keys are redacted recursively while non-secret structure is
/// preserved, so diagnostics can still show response shape and status fields.
dynamic redactSensitiveData(dynamic value) {
  if (value is Map) {
    return value.map(
      (key, nestedValue) => MapEntry(
        key,
        isSensitiveKey(key.toString())
            ? redactedSecret
            : redactSensitiveData(nestedValue),
      ),
    );
  }
  if (value is Iterable) {
    return value.map(redactSensitiveData).toList(growable: false);
  }
  if (value is String) return redactSensitiveText(value);
  return value;
}

/// Redacts common secret representations that may already have been rendered
/// into a log message (Bearer values, JWTs, or key/value assignments).
String redactSensitiveText(String value) {
  return value
      .replaceAll(_bearerPattern, 'Bearer $redactedSecret')
      .replaceAll(_jwtPattern, redactedSecret)
      .replaceAllMapped(
        _secretAssignmentPattern,
        (match) => '${match.group(1)}$redactedSecret',
      );
}

bool isSensitiveKey(String key) {
  final normalized = key.toLowerCase().replaceAll(RegExp('[^a-z0-9]'), '');
  return normalized == 'authorization' ||
      normalized == 'cookie' ||
      normalized == 'setcookie' ||
      normalized == 'password' ||
      normalized == 'secret' ||
      normalized == 'apikey' ||
      normalized == 'xcsrftoken' ||
      normalized == 'xxsrftoken' ||
      normalized.endsWith('token');
}

/// Keeps the route and non-secret query context while removing query secrets.
Uri redactSensitiveUri(Uri uri) {
  if (uri.queryParametersAll.isEmpty) return uri;
  final query = <String, dynamic>{};
  for (final entry in uri.queryParametersAll.entries) {
    query[entry.key] = isSensitiveKey(entry.key)
        ? redactedSecret
        : entry.value.length == 1
        ? redactSensitiveText(entry.value.single)
        : entry.value.map(redactSensitiveText).toList(growable: false);
  }
  return uri.replace(queryParameters: query);
}
