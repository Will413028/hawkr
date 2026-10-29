import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/store_card.pb.dart';
import 'package:hawkr_buyer/features/seller/data/store_card_display.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';

void main() {
  test('an open store with a code is openable and has no status label', () {
    final card = StoreCard(
      id: '1',
      storeCode: 'jinhu',
      name: '金湖水產',
      status: StoreCardStatus.STORE_CARD_STATUS_OPEN,
    );

    expect(card.isOpen, isTrue);
    expect(card.statusLabel(UiLocale.zhTW), isNull);
    expect(card.displayName(UiLocale.zhTW), '金湖水產');
  });

  test('paused and closed stores are not openable and carry their label', () {
    final paused = StoreCard(
      id: '1',
      storeCode: 'a',
      name: 'A',
      status: StoreCardStatus.STORE_CARD_STATUS_PAUSED,
    );
    final closed = StoreCard(
      id: '2',
      storeCode: 'b',
      name: 'B',
      status: StoreCardStatus.STORE_CARD_STATUS_CLOSED,
    );

    expect(paused.isOpen, isFalse);
    expect(paused.statusLabel(UiLocale.zhTW), '暫停營業');
    expect(closed.isOpen, isFalse);
    expect(closed.statusLabel(UiLocale.zhTW), '已關閉');
  });

  test('an unspecified status is neither openable nor labelled', () {
    final card = StoreCard(id: '1', storeCode: 'a', name: 'A');

    expect(card.isOpen, isFalse);
    expect(card.statusLabel(UiLocale.zhTW), isNull);
  });

  test('an open store without a store code is not openable', () {
    final card = StoreCard(
      id: '1',
      name: 'A',
      status: StoreCardStatus.STORE_CARD_STATUS_OPEN,
    );

    expect(card.isOpen, isFalse);
  });

  test('an unset or blank name shows the closed store name', () {
    expect(StoreCard(id: '1').displayName(UiLocale.zhTW), '已關閉的店家');
    expect(StoreCard(id: '1', name: '  ').displayName(UiLocale.zhTW), '已關閉的店家');
  });

  test('location is the county and town of a known town code', () {
    expect(StoreCard(townCode: '64000090').location, '高雄市前鎮區');
    expect(StoreCard(townCode: '').location, isEmpty);
    expect(StoreCard(townCode: '00000000').location, isEmpty);
  });

  test('rating stats need both the average and the review count', () {
    expect(StoreCard(ratingAverage: 4.5).hasRatingStats, isFalse);
    expect(StoreCard(reviewCount: 3).hasRatingStats, isFalse);
    expect(
      StoreCard(ratingAverage: 4.5, reviewCount: 3).hasRatingStats,
      isTrue,
    );
  });
}
