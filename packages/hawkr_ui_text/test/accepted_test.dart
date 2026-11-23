import 'package:hawkr_api/gen/hawkr/common/v1alpha1/policy_category.pbenum.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:test/test.dart';

void main() {
  const community = [
    PolicyCategory.POLICY_CATEGORY_SPAM,
    PolicyCategory.POLICY_CATEGORY_HARASSMENT,
    PolicyCategory.POLICY_CATEGORY_HATE,
    PolicyCategory.POLICY_CATEGORY_SEXUAL_CONTENT,
  ];

  test('清單沒有 UNSPECIFIED 與重複', () {
    for (final list in [
      sellerCreateReportRequestCategory,
      moderationSubmitReportRequestCategory,
    ]) {
      expect(list, isNot(contains(PolicyCategory.POLICY_CATEGORY_UNSPECIFIED)));
      expect(list.toSet().length, list.length);
    }
  });

  test('店家檢舉 12 個且不含社群的四個與 PLATFORM_ABUSE', () {
    expect(sellerCreateReportRequestCategory, hasLength(12));
    for (final c in [
      ...community,
      PolicyCategory.POLICY_CATEGORY_PLATFORM_ABUSE,
    ]) {
      expect(sellerCreateReportRequestCategory, isNot(contains(c)));
    }
  });

  test('社群檢舉 8 個，含社群的四個', () {
    expect(moderationSubmitReportRequestCategory, hasLength(8));
    expect(moderationSubmitReportRequestCategory, containsAll(community));
  });
}
