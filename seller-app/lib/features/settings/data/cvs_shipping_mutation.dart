import '../../../core/exceptions/app_exception.dart';

typedef CvsMutationSender =
    Future<Map<String, dynamic>> Function(
      String path,
      Map<String, dynamic> body,
    );

class CvsGoodsTypeMutation {
  const CvsGoodsTypeMutation({
    required this.initialEnabled,
    required this.targetEnabled,
    required this.onboardingId,
    required this.createBody,
  });

  final bool initialEnabled;
  final bool targetEnabled;
  final String? onboardingId;
  final Map<String, dynamic> createBody;

  bool get requiresRequest => initialEnabled != targetEnabled;
  bool get hasOnboardingId => (onboardingId?.trim() ?? '').isNotEmpty;

  String get label => '7-11';

  String get requestPath {
    final id = onboardingId?.trim() ?? '';
    if (id.isNotEmpty) {
      return '/api/v1/app/shipping_onboardings/$id/toggle';
    }
    return '/api/v1/app/shipping_onboardings/cvs';
  }

  Map<String, dynamic> get requestBody {
    final id = onboardingId?.trim() ?? '';
    if (id.isNotEmpty) return {'enabled': targetEnabled};
    return createBody;
  }
}

class CvsAppliedMutation {
  const CvsAppliedMutation({
    required this.mutation,
    required this.onboardingId,
  });

  final CvsGoodsTypeMutation mutation;
  final String? onboardingId;
}

class CvsShippingMutationOutcome {
  const CvsShippingMutationOutcome._({
    required this.completed,
    required this.pending,
    this.error,
    this.alreadyRunning = false,
  });

  factory CvsShippingMutationOutcome.success(
    List<CvsAppliedMutation> completed,
  ) {
    return CvsShippingMutationOutcome._(
      completed: List.unmodifiable(completed),
      pending: const [],
    );
  }

  factory CvsShippingMutationOutcome.failure({
    required List<CvsAppliedMutation> completed,
    required List<CvsGoodsTypeMutation> pending,
    required AppException error,
    bool alreadyRunning = false,
  }) {
    return CvsShippingMutationOutcome._(
      completed: List.unmodifiable(completed),
      pending: List.unmodifiable(pending),
      error: error,
      alreadyRunning: alreadyRunning,
    );
  }

  final List<CvsAppliedMutation> completed;
  final List<CvsGoodsTypeMutation> pending;
  final AppException? error;
  final bool alreadyRunning;

  bool get isSuccess => error == null;
  CvsGoodsTypeMutation? get failedMutation =>
      isSuccess || pending.isEmpty ? null : pending.first;
}

/// Applies 7-11 goods-type changes sequentially and retains a checkpoint for
/// every successful step.
class CvsShippingMutationCoordinator {
  CvsShippingMutationCoordinator(this._send);

  final CvsMutationSender _send;
  bool _isRunning = false;

  bool get isRunning => _isRunning;

  Future<CvsShippingMutationOutcome> apply(
    List<CvsGoodsTypeMutation> mutations,
  ) async {
    final pending = mutations.where((item) => item.requiresRequest).toList();
    if (_isRunning) {
      return CvsShippingMutationOutcome.failure(
        completed: const [],
        pending: pending,
        error: const ApiException(code: -2, message: '物流設定正在更新，請稍候'),
        alreadyRunning: true,
      );
    }

    _isRunning = true;
    final completed = <CvsAppliedMutation>[];
    try {
      for (var index = 0; index < pending.length; index++) {
        final mutation = pending[index];
        if (!mutation.targetEnabled && !mutation.hasOnboardingId) {
          return CvsShippingMutationOutcome.failure(
            completed: completed,
            pending: pending.sublist(index),
            error: ApiException(
              code: -3,
              message: '${mutation.label}物流缺少設定識別碼，請重新載入後再試',
            ),
          );
        }
        try {
          final response = await _send(
            mutation.requestPath,
            mutation.requestBody,
          );
          completed.add(
            CvsAppliedMutation(
              mutation: mutation,
              onboardingId:
                  _extractOnboardingId(response) ?? mutation.onboardingId,
            ),
          );
        } on AppException catch (error) {
          return CvsShippingMutationOutcome.failure(
            completed: completed,
            pending: pending.sublist(index),
            error: error,
          );
        } catch (error) {
          return CvsShippingMutationOutcome.failure(
            completed: completed,
            pending: pending.sublist(index),
            error: UnknownException(
              message: '物流設定更新失敗，請稍後再試',
              originalError: error,
            ),
          );
        }
      }
      return CvsShippingMutationOutcome.success(completed);
    } finally {
      _isRunning = false;
    }
  }

  String? _extractOnboardingId(Map<String, dynamic> response) {
    final data = response['data'];
    final source = data is Map ? data : response;
    final value =
        source['onboardingId'] ?? source['onboarding_id'] ?? source['id'];
    final id = value?.toString().trim() ?? '';
    return id.isEmpty ? null : id;
  }
}
