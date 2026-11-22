/// 發券對象依標籤篩選：切換標籤並讀該標籤的客戶名單。
///
/// 一次只選一個標籤（CRM 的 `tag_id` 是單一標籤）。讀取可能亂序完成：只有目前仍選著的那個標籤的結果才會套用，
/// 較早發出、較晚回來的結果與錯誤都丟掉。
class TagAudienceToggle<C> {
  TagAudienceToggle({
    required this.fetch,
    required this.setSelected,
    required this.setCustomers,
    required this.setLoading,
    required this.onError,
  });

  final Future<List<C>> Function(String tagId) fetch;
  final void Function(Set<String> selected) setSelected;
  final void Function(List<C> customers) setCustomers;
  final void Function(bool loading) setLoading;
  final void Function(Object error) onError;

  int _generation = 0;

  Future<void> toggle(String tagId, Set<String> current) async {
    final gen = ++_generation;
    if (current.contains(tagId)) {
      setSelected(<String>{});
      setCustomers(<C>[]);
      setLoading(false);
      return;
    }
    setSelected({tagId});
    setLoading(true);
    List<C> result;
    try {
      result = await fetch(tagId);
    } on Object catch (e) {
      if (gen != _generation) return;
      onError(e);
      setCustomers(<C>[]);
      setLoading(false);
      return;
    }
    if (gen != _generation) return;
    setCustomers(result);
    setLoading(false);
  }
}
