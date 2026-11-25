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

import 'package:protobuf/protobuf.dart' as $pb;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

/// ProductContentLimits 是商品內容的上限：每個欄位只用來掛 hawkr.common.v1alpha1.limits，不出現在任何 RPC。
/// 欄位名決定生成的常數名：string 欄位的 max_runes 生成 Max<欄位>Runes，repeated 欄位的 max_items 生成 Max<欄位>。
/// 商品的請求訊息定義之後，上限改掛在實際的請求欄位上，這個訊息即可刪除。
class ProductContentLimits extends $pb.GeneratedMessage {
  factory ProductContentLimits({
    $core.String? title,
    $core.Iterable<$core.String>? axes,
    $core.String? axisName,
    $core.String? optionValue,
    $core.Iterable<$core.String>? valuesPerAxis,
    $core.Iterable<$core.String>? skus,
    $core.Iterable<$core.String>? galleryImages,
    $core.String? alt,
    $core.Iterable<$core.String>? descriptionBlocks,
    $core.Iterable<$core.String>? descriptionImages,
    $core.String? description,
    $core.Iterable<$core.String>? listItems,
    $core.String? listItem,
    $core.Iterable<$core.String>? tableRows,
    $core.Iterable<$core.String>? tableCols,
    $core.String? tableCell,
  }) {
    final result = ProductContentLimits._();
    if (title != null) result.title = title;
    if (axes != null) result.axes.addAll(axes);
    if (axisName != null) result.axisName = axisName;
    if (optionValue != null) result.optionValue = optionValue;
    if (valuesPerAxis != null) result.valuesPerAxis.addAll(valuesPerAxis);
    if (skus != null) result.skus.addAll(skus);
    if (galleryImages != null) result.galleryImages.addAll(galleryImages);
    if (alt != null) result.alt = alt;
    if (descriptionBlocks != null)
      result.descriptionBlocks.addAll(descriptionBlocks);
    if (descriptionImages != null)
      result.descriptionImages.addAll(descriptionImages);
    if (description != null) result.description = description;
    if (listItems != null) result.listItems.addAll(listItems);
    if (listItem != null) result.listItem = listItem;
    if (tableRows != null) result.tableRows.addAll(tableRows);
    if (tableCols != null) result.tableCols.addAll(tableCols);
    if (tableCell != null) result.tableCell = tableCell;
    return result;
  }

  ProductContentLimits._();

  factory ProductContentLimits.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ProductContentLimits()..mergeFromBuffer(data, registry);
  factory ProductContentLimits.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ProductContentLimits()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ProductContentLimits',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.catalog.v1alpha1'),
      createEmptyInstance: ProductContentLimits.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'title')
    ..pPS(2, _omitFieldNames ? '' : 'axes')
    ..aOS(3, _omitFieldNames ? '' : 'axisName')
    ..aOS(4, _omitFieldNames ? '' : 'optionValue')
    ..pPS(5, _omitFieldNames ? '' : 'valuesPerAxis')
    ..pPS(6, _omitFieldNames ? '' : 'skus')
    ..pPS(7, _omitFieldNames ? '' : 'galleryImages')
    ..aOS(8, _omitFieldNames ? '' : 'alt')
    ..pPS(9, _omitFieldNames ? '' : 'descriptionBlocks')
    ..pPS(10, _omitFieldNames ? '' : 'descriptionImages')
    ..aOS(11, _omitFieldNames ? '' : 'description')
    ..pPS(12, _omitFieldNames ? '' : 'listItems')
    ..aOS(13, _omitFieldNames ? '' : 'listItem')
    ..pPS(14, _omitFieldNames ? '' : 'tableRows')
    ..pPS(15, _omitFieldNames ? '' : 'tableCols')
    ..aOS(16, _omitFieldNames ? '' : 'tableCell')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ProductContentLimits clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ProductContentLimits copyWith(void Function(ProductContentLimits) updates) =>
      super.copyWith((message) => updates(message as ProductContentLimits))
          as ProductContentLimits;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ProductContentLimits() / ProductContentLimits.new instead')
  static ProductContentLimits create() => ProductContentLimits._();
  static $pb.GeneratedMessage $_createMessage() => ProductContentLimits._();
  @$core.override
  ProductContentLimits createEmptyInstance() => ProductContentLimits._();
  @$core.pragma('dart2js:noInline')
  static ProductContentLimits getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ProductContentLimits>(
          ProductContentLimits.$_createMessage);
  static ProductContentLimits? _defaultInstance;

  /// 商品標題。
  @$pb.TagNumber(1)
  $core.String get title => $_getSZ(0);
  @$pb.TagNumber(1)
  set title($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasTitle() => $_has(0);
  @$pb.TagNumber(1)
  void clearTitle() => $_clearField(1);

  /// 選項軸個數。
  @$pb.TagNumber(2)
  $pb.PbList<$core.String> get axes => $_getList(1);

  /// 選項軸名稱。
  @$pb.TagNumber(3)
  $core.String get axisName => $_getSZ(2);
  @$pb.TagNumber(3)
  set axisName($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasAxisName() => $_has(2);
  @$pb.TagNumber(3)
  void clearAxisName() => $_clearField(3);

  /// 選項值。
  @$pb.TagNumber(4)
  $core.String get optionValue => $_getSZ(3);
  @$pb.TagNumber(4)
  set optionValue($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasOptionValue() => $_has(3);
  @$pb.TagNumber(4)
  void clearOptionValue() => $_clearField(4);

  /// 每個選項軸的值個數。
  @$pb.TagNumber(5)
  $pb.PbList<$core.String> get valuesPerAxis => $_getList(4);

  /// SKU 個數。
  @$pb.TagNumber(6)
  $pb.PbList<$core.String> get skus => $_getList(5);

  /// 主圖張數。
  @$pb.TagNumber(7)
  $pb.PbList<$core.String> get galleryImages => $_getList(6);

  /// 圖片的替代文字。
  @$pb.TagNumber(8)
  $core.String get alt => $_getSZ(7);
  @$pb.TagNumber(8)
  set alt($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasAlt() => $_has(7);
  @$pb.TagNumber(8)
  void clearAlt() => $_clearField(8);

  /// 描述的區塊個數。
  @$pb.TagNumber(9)
  $pb.PbList<$core.String> get descriptionBlocks => $_getList(8);

  /// 描述內的圖片張數。
  @$pb.TagNumber(10)
  $pb.PbList<$core.String> get descriptionImages => $_getList(9);

  /// 整份描述的文字總量：段落、清單項目與表格儲存格的文字合計（不含圖片的替代文字）。
  @$pb.TagNumber(11)
  $core.String get description => $_getSZ(10);
  @$pb.TagNumber(11)
  set description($core.String value) => $_setString(10, value);
  @$pb.TagNumber(11)
  $core.bool hasDescription() => $_has(10);
  @$pb.TagNumber(11)
  void clearDescription() => $_clearField(11);

  /// 清單的項目個數。
  @$pb.TagNumber(12)
  $pb.PbList<$core.String> get listItems => $_getList(11);

  /// 清單的單一項目文字。
  @$pb.TagNumber(13)
  $core.String get listItem => $_getSZ(12);
  @$pb.TagNumber(13)
  set listItem($core.String value) => $_setString(12, value);
  @$pb.TagNumber(13)
  $core.bool hasListItem() => $_has(12);
  @$pb.TagNumber(13)
  void clearListItem() => $_clearField(13);

  /// 表格的列數。
  @$pb.TagNumber(14)
  $pb.PbList<$core.String> get tableRows => $_getList(13);

  /// 表格的欄數。
  @$pb.TagNumber(15)
  $pb.PbList<$core.String> get tableCols => $_getList(14);

  /// 表格的單一儲存格文字。
  @$pb.TagNumber(16)
  $core.String get tableCell => $_getSZ(15);
  @$pb.TagNumber(16)
  set tableCell($core.String value) => $_setString(15, value);
  @$pb.TagNumber(16)
  $core.bool hasTableCell() => $_has(15);
  @$pb.TagNumber(16)
  void clearTableCell() => $_clearField(16);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
