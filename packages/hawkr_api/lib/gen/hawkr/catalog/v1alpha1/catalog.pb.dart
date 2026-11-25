// This is a generated file - do not edit.
//
// Generated from hawkr/catalog/v1alpha1/catalog.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:fixnum/fixnum.dart' as $fixnum;
import 'package:protobuf/protobuf.dart' as $pb;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

/// ProductInput 是商品的完整可寫內容。修改是整體取代：沒有帶的圖片、選項軸與 SKU 組合會被移除。
/// 狀態不在這裡，用切換狀態的操作改。
class ProductInput extends $pb.GeneratedMessage {
  factory ProductInput({
    $core.String? title,
    $core.String? categoryId,
    $core.Iterable<$core.String>? hashtagInputs,
    $core.Iterable<ImageInput>? gallery,
    $core.Iterable<DescriptionBlock>? description,
    $core.Iterable<OptionAxisInput>? axes,
    $core.Iterable<SkuInput>? skus,
  }) {
    final result = ProductInput._();
    if (title != null) result.title = title;
    if (categoryId != null) result.categoryId = categoryId;
    if (hashtagInputs != null) result.hashtagInputs.addAll(hashtagInputs);
    if (gallery != null) result.gallery.addAll(gallery);
    if (description != null) result.description.addAll(description);
    if (axes != null) result.axes.addAll(axes);
    if (skus != null) result.skus.addAll(skus);
    return result;
  }

  ProductInput._();

  factory ProductInput.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ProductInput()..mergeFromBuffer(data, registry);
  factory ProductInput.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ProductInput()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ProductInput',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: ProductInput.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'title')
    ..aOS(2, _omitFieldNames ? '' : 'categoryId')
    ..pPS(3, _omitFieldNames ? '' : 'hashtagInputs')
    ..pPM<ImageInput>(4, _omitFieldNames ? '' : 'gallery',
        subBuilder: ImageInput.$_createMessage)
    ..pPM<DescriptionBlock>(5, _omitFieldNames ? '' : 'description',
        subBuilder: DescriptionBlock.$_createMessage)
    ..pPM<OptionAxisInput>(6, _omitFieldNames ? '' : 'axes',
        subBuilder: OptionAxisInput.$_createMessage)
    ..pPM<SkuInput>(7, _omitFieldNames ? '' : 'skus',
        subBuilder: SkuInput.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ProductInput clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ProductInput copyWith(void Function(ProductInput) updates) =>
      super.copyWith((message) => updates(message as ProductInput))
          as ProductInput;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ProductInput() / ProductInput.new instead')
  static ProductInput create() => ProductInput._();
  static $pb.GeneratedMessage $_createMessage() => ProductInput._();
  @$core.override
  ProductInput createEmptyInstance() => ProductInput._();
  @$core.pragma('dart2js:noInline')
  static ProductInput getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ProductInput>(
          ProductInput.$_createMessage);
  static ProductInput? _defaultInstance;

  /// 商品標題，去頭尾空白後不得為空。
  @$pb.TagNumber(1)
  $core.String get title => $_getSZ(0);
  @$pb.TagNumber(1)
  set title($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasTitle() => $_has(0);
  @$pb.TagNumber(1)
  void clearTitle() => $_clearField(1);

  /// 分類 ID，必須是葉節點。
  @$pb.TagNumber(2)
  $core.String get categoryId => $_getSZ(1);
  @$pb.TagNumber(2)
  set categoryId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasCategoryId() => $_has(1);
  @$pb.TagNumber(2)
  void clearCategoryId() => $_clearField(2);

  /// 標籤的輸入（使用者輸入的樣子，可帶開頭的 #）；整批取代，空清單是沒有標籤。個數與格式的規則同 social 的標籤（platform/hashtag）。
  @$pb.TagNumber(3)
  $pb.PbList<$core.String> get hashtagInputs => $_getList(2);

  /// 主圖，依顯示順序；狀態為 ACTIVE 的商品至少要有一張。
  @$pb.TagNumber(4)
  $pb.PbList<ImageInput> get gallery => $_getList(3);

  /// 描述的區塊，依顯示順序。上限跨整份描述計算：區塊個數，帶圖片的區塊個數，以及段落、清單項目與表格儲存格的文字總量。
  @$pb.TagNumber(5)
  $pb.PbList<DescriptionBlock> get description => $_getList(4);

  /// 選項軸，依順序；沒有軸的商品只有一個預設 SKU。
  @$pb.TagNumber(6)
  $pb.PbList<OptionAxisInput> get axes => $_getList(5);

  /// SKU，以選項值的組合辨識；至少一個。
  @$pb.TagNumber(7)
  $pb.PbList<SkuInput> get skus => $_getList(6);
}

enum ImageInput_Source { stagedToken, assetId, notSet }

/// ImageInput 是寫入時的一張圖：新上傳的圖帶 staged token，商品現有的圖帶它的 asset_id，兩者剛好給一個。
class ImageInput extends $pb.GeneratedMessage {
  factory ImageInput({
    $core.String? stagedToken,
    $core.String? assetId,
    $core.String? alt,
  }) {
    final result = ImageInput._();
    if (stagedToken != null) result.stagedToken = stagedToken;
    if (assetId != null) result.assetId = assetId;
    if (alt != null) result.alt = alt;
    return result;
  }

  ImageInput._();

  factory ImageInput.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ImageInput()..mergeFromBuffer(data, registry);
  factory ImageInput.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ImageInput()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, ImageInput_Source> _ImageInput_SourceByTag =
      {
    1: ImageInput_Source.stagedToken,
    2: ImageInput_Source.assetId,
    0: ImageInput_Source.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ImageInput',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: ImageInput.$_createMessage)
    ..oo(0, [1, 2])
    ..aOS(1, _omitFieldNames ? '' : 'stagedToken')
    ..aOS(2, _omitFieldNames ? '' : 'assetId')
    ..aOS(3, _omitFieldNames ? '' : 'alt')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ImageInput clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ImageInput copyWith(void Function(ImageInput) updates) =>
      super.copyWith((message) => updates(message as ImageInput)) as ImageInput;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ImageInput() / ImageInput.new instead')
  static ImageInput create() => ImageInput._();
  static $pb.GeneratedMessage $_createMessage() => ImageInput._();
  @$core.override
  ImageInput createEmptyInstance() => ImageInput._();
  @$core.pragma('dart2js:noInline')
  static ImageInput getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ImageInput>(ImageInput.$_createMessage);
  static ImageInput? _defaultInstance;

  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  ImageInput_Source whichSource() => _ImageInput_SourceByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  void clearSource() => $_clearField($_whichOneof(0));

  /// media 上傳完成後拿到的 staged token；伺服器在寫入的 transaction 內確認它。
  @$pb.TagNumber(1)
  $core.String get stagedToken => $_getSZ(0);
  @$pb.TagNumber(1)
  set stagedToken($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStagedToken() => $_has(0);
  @$pb.TagNumber(1)
  void clearStagedToken() => $_clearField(1);

  /// 商品現有的圖（讀取商品時回傳的資產 ID），沿用不重新上傳。
  @$pb.TagNumber(2)
  $core.String get assetId => $_getSZ(1);
  @$pb.TagNumber(2)
  set assetId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAssetId() => $_has(1);
  @$pb.TagNumber(2)
  void clearAssetId() => $_clearField(2);

  /// 圖片的替代文字，可空。
  @$pb.TagNumber(3)
  $core.String get alt => $_getSZ(2);
  @$pb.TagNumber(3)
  set alt($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasAlt() => $_has(2);
  @$pb.TagNumber(3)
  void clearAlt() => $_clearField(3);
}

enum DescriptionBlock_Block { paragraph, image, bulletList, table, notSet }

/// DescriptionBlock 是描述的一個區塊，剛好設一種。
class DescriptionBlock extends $pb.GeneratedMessage {
  factory DescriptionBlock({
    $core.String? paragraph,
    ImageInput? image,
    BulletList? bulletList,
    Table? table,
  }) {
    final result = DescriptionBlock._();
    if (paragraph != null) result.paragraph = paragraph;
    if (image != null) result.image = image;
    if (bulletList != null) result.bulletList = bulletList;
    if (table != null) result.table = table;
    return result;
  }

  DescriptionBlock._();

  factory DescriptionBlock.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DescriptionBlock()..mergeFromBuffer(data, registry);
  factory DescriptionBlock.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DescriptionBlock()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, DescriptionBlock_Block>
      _DescriptionBlock_BlockByTag = {
    1: DescriptionBlock_Block.paragraph,
    2: DescriptionBlock_Block.image,
    3: DescriptionBlock_Block.bulletList,
    4: DescriptionBlock_Block.table,
    0: DescriptionBlock_Block.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DescriptionBlock',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: DescriptionBlock.$_createMessage)
    ..oo(0, [1, 2, 3, 4])
    ..aOS(1, _omitFieldNames ? '' : 'paragraph')
    ..aOM<ImageInput>(2, _omitFieldNames ? '' : 'image',
        subBuilder: ImageInput.$_createMessage)
    ..aOM<BulletList>(3, _omitFieldNames ? '' : 'bulletList',
        subBuilder: BulletList.$_createMessage)
    ..aOM<Table>(4, _omitFieldNames ? '' : 'table',
        subBuilder: Table.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DescriptionBlock clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DescriptionBlock copyWith(void Function(DescriptionBlock) updates) =>
      super.copyWith((message) => updates(message as DescriptionBlock))
          as DescriptionBlock;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use DescriptionBlock() / DescriptionBlock.new instead')
  static DescriptionBlock create() => DescriptionBlock._();
  static $pb.GeneratedMessage $_createMessage() => DescriptionBlock._();
  @$core.override
  DescriptionBlock createEmptyInstance() => DescriptionBlock._();
  @$core.pragma('dart2js:noInline')
  static DescriptionBlock getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<DescriptionBlock>(
          DescriptionBlock.$_createMessage);
  static DescriptionBlock? _defaultInstance;

  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  @$pb.TagNumber(3)
  @$pb.TagNumber(4)
  DescriptionBlock_Block whichBlock() =>
      _DescriptionBlock_BlockByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  @$pb.TagNumber(3)
  @$pb.TagNumber(4)
  void clearBlock() => $_clearField($_whichOneof(0));

  /// 段落，純文字。
  @$pb.TagNumber(1)
  $core.String get paragraph => $_getSZ(0);
  @$pb.TagNumber(1)
  set paragraph($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasParagraph() => $_has(0);
  @$pb.TagNumber(1)
  void clearParagraph() => $_clearField(1);

  /// 圖片。
  @$pb.TagNumber(2)
  ImageInput get image => $_getN(1);
  @$pb.TagNumber(2)
  set image(ImageInput value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasImage() => $_has(1);
  @$pb.TagNumber(2)
  void clearImage() => $_clearField(2);
  @$pb.TagNumber(2)
  ImageInput ensureImage() => $_ensure(1);

  /// 項目清單。
  @$pb.TagNumber(3)
  BulletList get bulletList => $_getN(2);
  @$pb.TagNumber(3)
  set bulletList(BulletList value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasBulletList() => $_has(2);
  @$pb.TagNumber(3)
  void clearBulletList() => $_clearField(3);
  @$pb.TagNumber(3)
  BulletList ensureBulletList() => $_ensure(2);

  /// 表格。
  @$pb.TagNumber(4)
  Table get table => $_getN(3);
  @$pb.TagNumber(4)
  set table(Table value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasTable() => $_has(3);
  @$pb.TagNumber(4)
  void clearTable() => $_clearField(4);
  @$pb.TagNumber(4)
  Table ensureTable() => $_ensure(3);
}

/// BulletList 是項目清單。
class BulletList extends $pb.GeneratedMessage {
  factory BulletList({
    $core.Iterable<$core.String>? items,
  }) {
    final result = BulletList._();
    if (items != null) result.items.addAll(items);
    return result;
  }

  BulletList._();

  factory BulletList.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BulletList()..mergeFromBuffer(data, registry);
  factory BulletList.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BulletList()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BulletList',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: BulletList.$_createMessage)
    ..pPS(1, _omitFieldNames ? '' : 'items')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BulletList clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BulletList copyWith(void Function(BulletList) updates) =>
      super.copyWith((message) => updates(message as BulletList)) as BulletList;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use BulletList() / BulletList.new instead')
  static BulletList create() => BulletList._();
  static $pb.GeneratedMessage $_createMessage() => BulletList._();
  @$core.override
  BulletList createEmptyInstance() => BulletList._();
  @$core.pragma('dart2js:noInline')
  static BulletList getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BulletList>(BulletList.$_createMessage);
  static BulletList? _defaultInstance;

  /// 清單項目，至少一項；每項的文字上限是 max_runes。
  @$pb.TagNumber(1)
  $pb.PbList<$core.String> get items => $_getList(0);
}

/// Table 是表格，每列的儲存格數相同。
class Table extends $pb.GeneratedMessage {
  factory Table({
    $core.Iterable<TableRow>? rows,
  }) {
    final result = Table._();
    if (rows != null) result.rows.addAll(rows);
    return result;
  }

  Table._();

  factory Table.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Table()..mergeFromBuffer(data, registry);
  factory Table.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Table()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Table',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: Table.$_createMessage)
    ..pPM<TableRow>(1, _omitFieldNames ? '' : 'rows',
        subBuilder: TableRow.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Table clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Table copyWith(void Function(Table) updates) =>
      super.copyWith((message) => updates(message as Table)) as Table;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Table() / Table.new instead')
  static Table create() => Table._();
  static $pb.GeneratedMessage $_createMessage() => Table._();
  @$core.override
  Table createEmptyInstance() => Table._();
  @$core.pragma('dart2js:noInline')
  static Table getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Table>(Table.$_createMessage);
  static Table? _defaultInstance;

  /// 表格的列，至少一列。
  @$pb.TagNumber(1)
  $pb.PbList<TableRow> get rows => $_getList(0);
}

/// TableRow 是表格的一列。
class TableRow extends $pb.GeneratedMessage {
  factory TableRow({
    $core.Iterable<$core.String>? cells,
  }) {
    final result = TableRow._();
    if (cells != null) result.cells.addAll(cells);
    return result;
  }

  TableRow._();

  factory TableRow.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TableRow()..mergeFromBuffer(data, registry);
  factory TableRow.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TableRow()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TableRow',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: TableRow.$_createMessage)
    ..pPS(1, _omitFieldNames ? '' : 'cells')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TableRow clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TableRow copyWith(void Function(TableRow) updates) =>
      super.copyWith((message) => updates(message as TableRow)) as TableRow;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use TableRow() / TableRow.new instead')
  static TableRow create() => TableRow._();
  static $pb.GeneratedMessage $_createMessage() => TableRow._();
  @$core.override
  TableRow createEmptyInstance() => TableRow._();
  @$core.pragma('dart2js:noInline')
  static TableRow getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<TableRow>(TableRow.$_createMessage);
  static TableRow? _defaultInstance;

  /// 這一列的儲存格，至少一格；每格的文字上限是 max_runes。
  @$pb.TagNumber(1)
  $pb.PbList<$core.String> get cells => $_getList(0);
}

/// OptionAxisInput 是一個選項軸：名稱與有序的選項值。名稱與值都是賣家的字，不翻譯。
class OptionAxisInput extends $pb.GeneratedMessage {
  factory OptionAxisInput({
    $core.String? name,
    $core.Iterable<$core.String>? values,
  }) {
    final result = OptionAxisInput._();
    if (name != null) result.name = name;
    if (values != null) result.values.addAll(values);
    return result;
  }

  OptionAxisInput._();

  factory OptionAxisInput.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      OptionAxisInput()..mergeFromBuffer(data, registry);
  factory OptionAxisInput.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      OptionAxisInput()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'OptionAxisInput',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: OptionAxisInput.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'name')
    ..pPS(2, _omitFieldNames ? '' : 'values')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  OptionAxisInput clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  OptionAxisInput copyWith(void Function(OptionAxisInput) updates) =>
      super.copyWith((message) => updates(message as OptionAxisInput))
          as OptionAxisInput;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use OptionAxisInput() / OptionAxisInput.new instead')
  static OptionAxisInput create() => OptionAxisInput._();
  static $pb.GeneratedMessage $_createMessage() => OptionAxisInput._();
  @$core.override
  OptionAxisInput createEmptyInstance() => OptionAxisInput._();
  @$core.pragma('dart2js:noInline')
  static OptionAxisInput getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<OptionAxisInput>(
          OptionAxisInput.$_createMessage);
  static OptionAxisInput? _defaultInstance;

  /// 軸名稱，例如「顏色」。
  @$pb.TagNumber(1)
  $core.String get name => $_getSZ(0);
  @$pb.TagNumber(1)
  set name($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasName() => $_has(0);
  @$pb.TagNumber(1)
  void clearName() => $_clearField(1);

  /// 選項值，依顯示順序，至少一個；每個值的長度上限是 max_runes。
  @$pb.TagNumber(2)
  $pb.PbList<$core.String> get values => $_getList(1);
}

/// SkuInput 是一個 SKU，以選項值的組合辨識：沒有軸的商品兩個選項都空；有一個軸只給 option1。
/// 組合已存在時更新價格；庫存只在新組合時作為初始值，既有 SKU 的庫存只由庫存調整改。
class SkuInput extends $pb.GeneratedMessage {
  factory SkuInput({
    $core.String? option1,
    $core.String? option2,
    $fixnum.Int64? price,
    $fixnum.Int64? compareAtPrice,
    $core.int? initialStock,
  }) {
    final result = SkuInput._();
    if (option1 != null) result.option1 = option1;
    if (option2 != null) result.option2 = option2;
    if (price != null) result.price = price;
    if (compareAtPrice != null) result.compareAtPrice = compareAtPrice;
    if (initialStock != null) result.initialStock = initialStock;
    return result;
  }

  SkuInput._();

  factory SkuInput.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SkuInput()..mergeFromBuffer(data, registry);
  factory SkuInput.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SkuInput()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SkuInput',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: SkuInput.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'option1')
    ..aOS(2, _omitFieldNames ? '' : 'option2')
    ..aInt64(3, _omitFieldNames ? '' : 'price')
    ..aInt64(4, _omitFieldNames ? '' : 'compareAtPrice')
    ..aI(5, _omitFieldNames ? '' : 'initialStock')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SkuInput clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SkuInput copyWith(void Function(SkuInput) updates) =>
      super.copyWith((message) => updates(message as SkuInput)) as SkuInput;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use SkuInput() / SkuInput.new instead')
  static SkuInput create() => SkuInput._();
  static $pb.GeneratedMessage $_createMessage() => SkuInput._();
  @$core.override
  SkuInput createEmptyInstance() => SkuInput._();
  @$core.pragma('dart2js:noInline')
  static SkuInput getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SkuInput>(SkuInput.$_createMessage);
  static SkuInput? _defaultInstance;

  /// 第一個軸的選項值，必須是 axes[0].values 之一。
  @$pb.TagNumber(1)
  $core.String get option1 => $_getSZ(0);
  @$pb.TagNumber(1)
  set option1($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasOption1() => $_has(0);
  @$pb.TagNumber(1)
  void clearOption1() => $_clearField(1);

  /// 第二個軸的選項值，必須是 axes[1].values 之一。
  @$pb.TagNumber(2)
  $core.String get option2 => $_getSZ(1);
  @$pb.TagNumber(2)
  set option2($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasOption2() => $_has(1);
  @$pb.TagNumber(2)
  void clearOption2() => $_clearField(2);

  /// 售價，新台幣元的整數，大於 0。
  @$pb.TagNumber(3)
  $fixnum.Int64 get price => $_getI64(2);
  @$pb.TagNumber(3)
  set price($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPrice() => $_has(2);
  @$pb.TagNumber(3)
  void clearPrice() => $_clearField(3);

  /// 原價（劃線價），新台幣元的整數；沒有設是沒有原價。
  @$pb.TagNumber(4)
  $fixnum.Int64 get compareAtPrice => $_getI64(3);
  @$pb.TagNumber(4)
  set compareAtPrice($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasCompareAtPrice() => $_has(3);
  @$pb.TagNumber(4)
  void clearCompareAtPrice() => $_clearField(4);

  /// 新組合的初始庫存，不得為負。
  @$pb.TagNumber(5)
  $core.int get initialStock => $_getIZ(4);
  @$pb.TagNumber(5)
  set initialStock($core.int value) => $_setSignedInt32(4, value);
  @$pb.TagNumber(5)
  $core.bool hasInitialStock() => $_has(4);
  @$pb.TagNumber(5)
  void clearInitialStock() => $_clearField(5);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
