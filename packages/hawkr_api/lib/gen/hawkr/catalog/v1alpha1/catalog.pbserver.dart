// This is a generated file - do not edit.
//
// Generated from hawkr/catalog/v1alpha1/catalog.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

import 'catalog.pb.dart' as $3;
import 'catalog.pbjson.dart';

export 'catalog.pb.dart';

abstract class CatalogPublicServiceBase extends $pb.GeneratedService {
  $async.Future<$3.ListProductsResponse> listProducts(
      $pb.ServerContext ctx, $3.ListProductsRequest request);
  $async.Future<$3.SearchProductsResponse> searchProducts(
      $pb.ServerContext ctx, $3.SearchProductsRequest request);
  $async.Future<$3.SuggestProductNamesResponse> suggestProductNames(
      $pb.ServerContext ctx, $3.SuggestProductNamesRequest request);
  $async.Future<$3.GetProductResponse> getProduct(
      $pb.ServerContext ctx, $3.GetProductRequest request);
  $async.Future<$3.ListCategoriesResponse> listCategories(
      $pb.ServerContext ctx, $3.ListCategoriesRequest request);
  $async.Future<$3.SuggestHashtagsResponse> suggestHashtags(
      $pb.ServerContext ctx, $3.SuggestHashtagsRequest request);
  $async.Future<$3.SuggestTrendingHashtagsResponse> suggestTrendingHashtags(
      $pb.ServerContext ctx, $3.SuggestTrendingHashtagsRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'ListProducts':
        return $3.ListProductsRequest();
      case 'SearchProducts':
        return $3.SearchProductsRequest();
      case 'SuggestProductNames':
        return $3.SuggestProductNamesRequest();
      case 'GetProduct':
        return $3.GetProductRequest();
      case 'ListCategories':
        return $3.ListCategoriesRequest();
      case 'SuggestHashtags':
        return $3.SuggestHashtagsRequest();
      case 'SuggestTrendingHashtags':
        return $3.SuggestTrendingHashtagsRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'ListProducts':
        return listProducts(ctx, request as $3.ListProductsRequest);
      case 'SearchProducts':
        return searchProducts(ctx, request as $3.SearchProductsRequest);
      case 'SuggestProductNames':
        return suggestProductNames(
            ctx, request as $3.SuggestProductNamesRequest);
      case 'GetProduct':
        return getProduct(ctx, request as $3.GetProductRequest);
      case 'ListCategories':
        return listCategories(ctx, request as $3.ListCategoriesRequest);
      case 'SuggestHashtags':
        return suggestHashtags(ctx, request as $3.SuggestHashtagsRequest);
      case 'SuggestTrendingHashtags':
        return suggestTrendingHashtags(
            ctx, request as $3.SuggestTrendingHashtagsRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json =>
      CatalogPublicServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => CatalogPublicServiceBase$messageJson;
}

abstract class SellerCatalogAppServiceBase extends $pb.GeneratedService {
  $async.Future<$3.ListMyProductsResponse> listMyProducts(
      $pb.ServerContext ctx, $3.ListMyProductsRequest request);
  $async.Future<$3.GetMyProductResponse> getMyProduct(
      $pb.ServerContext ctx, $3.GetMyProductRequest request);
  $async.Future<$3.CountMyProductsResponse> countMyProducts(
      $pb.ServerContext ctx, $3.CountMyProductsRequest request);
  $async.Future<$3.CreateProductResponse> createProduct(
      $pb.ServerContext ctx, $3.CreateProductRequest request);
  $async.Future<$3.UpdateProductResponse> updateProduct(
      $pb.ServerContext ctx, $3.UpdateProductRequest request);
  $async.Future<$3.SetProductStatusResponse> setProductStatus(
      $pb.ServerContext ctx, $3.SetProductStatusRequest request);
  $async.Future<$3.DeleteProductResponse> deleteProduct(
      $pb.ServerContext ctx, $3.DeleteProductRequest request);
  $async.Future<$3.AdjustStockResponse> adjustStock(
      $pb.ServerContext ctx, $3.AdjustStockRequest request);
  $async.Future<$3.SetStockResponse> setStock(
      $pb.ServerContext ctx, $3.SetStockRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'ListMyProducts':
        return $3.ListMyProductsRequest();
      case 'GetMyProduct':
        return $3.GetMyProductRequest();
      case 'CountMyProducts':
        return $3.CountMyProductsRequest();
      case 'CreateProduct':
        return $3.CreateProductRequest();
      case 'UpdateProduct':
        return $3.UpdateProductRequest();
      case 'SetProductStatus':
        return $3.SetProductStatusRequest();
      case 'DeleteProduct':
        return $3.DeleteProductRequest();
      case 'AdjustStock':
        return $3.AdjustStockRequest();
      case 'SetStock':
        return $3.SetStockRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'ListMyProducts':
        return listMyProducts(ctx, request as $3.ListMyProductsRequest);
      case 'GetMyProduct':
        return getMyProduct(ctx, request as $3.GetMyProductRequest);
      case 'CountMyProducts':
        return countMyProducts(ctx, request as $3.CountMyProductsRequest);
      case 'CreateProduct':
        return createProduct(ctx, request as $3.CreateProductRequest);
      case 'UpdateProduct':
        return updateProduct(ctx, request as $3.UpdateProductRequest);
      case 'SetProductStatus':
        return setProductStatus(ctx, request as $3.SetProductStatusRequest);
      case 'DeleteProduct':
        return deleteProduct(ctx, request as $3.DeleteProductRequest);
      case 'AdjustStock':
        return adjustStock(ctx, request as $3.AdjustStockRequest);
      case 'SetStock':
        return setStock(ctx, request as $3.SetStockRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json =>
      SellerCatalogAppServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => SellerCatalogAppServiceBase$messageJson;
}
