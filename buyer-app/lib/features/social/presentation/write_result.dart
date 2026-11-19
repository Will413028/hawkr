/// 使用者主動的寫入結果。[discarded]：回來時觀看者已換（或畫面已不在），結果沒有套用，也不必提示。
enum WriteStatus { applied, discarded, failed }

class WriteResult {
  const WriteResult._(this.status, [this.error]);

  const WriteResult.applied() : this._(WriteStatus.applied);
  const WriteResult.discarded() : this._(WriteStatus.discarded);
  const WriteResult.failed(Object error) : this._(WriteStatus.failed, error);

  final WriteStatus status;
  final Object? error;

  bool get isApplied => status == WriteStatus.applied;
}
