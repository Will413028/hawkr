import 'package:sentry_flutter/sentry_flutter.dart';

import '../../shared/utils/secret_redaction.dart';

const _sensitiveHeaderKeys = {
  'authorization',
  'cookie',
  'set-cookie',
  'x-csrf-token',
  'x-xsrf-token',
};

/// beforeSend PII scrub — 同 website/admin 規則：
/// 拔敏感 header/cookie、遞迴清 body/message/breadcrumb 敏感值、拔 user email/ip。
/// sendDefaultPii 已預設 false（見 main.dart SentryFlutter.init），這裡是防禦性第二層。
SentryEvent scrubSentryEvent(SentryEvent event, Hint hint) {
  final request = event.request;
  if (request != null) {
    // Mutate request in place via copyWith with removeCookies flag.
    // This is safe because removeCookies is a dedicated flag that correctly nulls
    // cookies while preserving every other field (fragment/apiTarget/env/etc.) via
    // copyWith's ?? this.field fallback.
    // ignore: deprecated_member_use
    event.request = request.copyWith(
      url: _redactUrl(request.url),
      queryString: _redactQuery(request.queryString),
      headers: request.headers.isEmpty
          ? null
          : Map.fromEntries(
              request.headers.entries.where(
                (e) =>
                    !_sensitiveHeaderKeys.contains(e.key.toLowerCase()) &&
                    !isSensitiveKey(e.key),
              ),
            ),
      removeCookies: true,
      data: redactSensitiveData(request.data),
    );
  }

  final message = event.message;
  if (message != null) {
    message.formatted = redactSensitiveText(message.formatted);
    if (message.template != null) {
      message.template = redactSensitiveText(message.template!);
    }
    message.params = message.params
        ?.map(redactSensitiveData)
        .toList(growable: false);
  }

  for (final exception in event.exceptions ?? const <SentryException>[]) {
    if (exception.value != null) {
      exception.value = redactSensitiveText(exception.value!);
    }
  }

  for (final breadcrumb in event.breadcrumbs ?? const <Breadcrumb>[]) {
    if (breadcrumb.message != null) {
      breadcrumb.message = redactSensitiveText(breadcrumb.message!);
    }
    final data = breadcrumb.data;
    if (data != null) {
      breadcrumb.data = Map<String, dynamic>.from(
        redactSensitiveData(data) as Map,
      );
    }
  }

  // ignore: deprecated_member_use
  if (event.extra != null) {
    // ignore: deprecated_member_use
    event.extra = Map<String, dynamic>.from(
      // ignore: deprecated_member_use
      redactSensitiveData(event.extra!) as Map,
    );
  }

  final user = event.user;
  if (user != null) {
    // Mutate user fields in place
    user.email = null;
    user.ipAddress = null;
  }

  return event;
}

String? _redactUrl(String? value) {
  if (value == null) return null;
  final uri = Uri.tryParse(value);
  return uri == null
      ? redactSensitiveText(value)
      : redactSensitiveUri(uri).toString();
}

String? _redactQuery(String? value) {
  if (value == null || value.isEmpty) return value;
  return redactSensitiveUri(Uri(query: value)).query;
}
