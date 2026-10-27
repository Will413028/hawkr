import 'package:freezed_annotation/freezed_annotation.dart';

part 'product_model.freezed.dart';
part 'product_model.g.dart';

@freezed
abstract class Product with _$Product {
  const factory Product({
    @JsonKey(name: 'skuId') required int skuId,
    required String title,
    required int price,
    required String image,
    @JsonKey(name: 'categoryId') int? categoryId,
    @JsonKey(name: 'sellerId') int? sellerId,
    @JsonKey(name: 'merchantName') String? merchantName,
    int? stock,
    @JsonKey(name: 'isFavorited') @Default(false) bool isFavorited,
    @JsonKey(name: 'isRestockSubscribed')
    @Default(false)
    bool isRestockSubscribed,
    @JsonKey(name: 'selfOwned') @Default(false) bool selfOwned,
  }) = _Product;

  factory Product.fromJson(Map<String, dynamic> json) =>
      _$ProductFromJson(json);
}

@freezed
abstract class SearchSuggest with _$SearchSuggest {
  const factory SearchSuggest({@Default([]) List<String> suggestions}) =
      _SearchSuggest;

  factory SearchSuggest.fromJson(Map<String, dynamic> json) =>
      _$SearchSuggestFromJson(json);
}
