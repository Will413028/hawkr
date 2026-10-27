import 'dart:collection';

final class AiEditHistoryEntry {
  const AiEditHistoryEntry({
    required this.action,
    required this.timestamp,
    required this.count,
  });

  final String action;
  final DateTime timestamp;
  final int count;

  factory AiEditHistoryEntry.fromJson(Map<String, dynamic> json) {
    return AiEditHistoryEntry(
      action: (json['action'] ?? '').toString(),
      timestamp:
          DateTime.tryParse((json['timestamp'] ?? '').toString()) ??
          DateTime.fromMillisecondsSinceEpoch(0),
      count:
          (json['count'] as num?)?.toInt() ??
          int.tryParse(json['count']?.toString() ?? '') ??
          0,
    );
  }

  Map<String, dynamic> toJson() => {
    'action': action,
    'timestamp': timestamp.toIso8601String(),
    'count': count,
  };

  Map<String, dynamic> toViewMap() {
    return UnmodifiableMapView<String, dynamic>({
      'action': action,
      'timestamp': timestamp,
      'count': count,
    });
  }
}
