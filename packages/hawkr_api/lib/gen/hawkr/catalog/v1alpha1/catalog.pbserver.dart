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

import 'catalog.pb.dart' as $5;
import 'catalog.pbjson.dart';

export 'catalog.pb.dart';

abstract class CatalogPublicServiceBase extends $pb.GeneratedService {
  $async.Future<$5.ListProductsResponse> listProducts(
      $pb.ServerContext ctx, $5.ListProductsRequest request);
  $async.Future<$5.SearchProductsResponse> searchProducts(
      $pb.ServerContext ctx, $5.SearchProductsRequest request);
  $async.Future<$5.SuggestProductNamesResponse> suggestProductNames(
      $pb.ServerContext ctx, $5.SuggestProductNamesRequest request);
  $async.Future<$5.GetProductResponse> getProduct(
      $pb.ServerContext ctx, $5.GetProductRequest request);
  $async.Future<$5.ListCategoriesResponse> listCategories(
      $pb.ServerContext ctx, $5.ListCategoriesRequest request);
  $async.Future<$5.SuggestHashtagsResponse> suggestHashtags(
      $pb.ServerContext ctx, $5.SuggestHashtagsRequest request);
  $async.Future<$5.SuggestTrendingHashtagsResponse> suggestTrendingHashtags(
      $pb.ServerContext ctx, $5.SuggestTrendingHashtagsRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'ListProducts':
        return $5.ListProductsRequest();
      case 'SearchProducts':
        return $5.SearchProductsRequest();
      case 'SuggestProductNames':
        return $5.SuggestProductNamesRequest();
      case 'GetProduct':
        return $5.GetProductRequest();
      case 'ListCategories':
        return $5.ListCategoriesRequest();
      case 'SuggestHashtags':
        return $5.SuggestHashtagsRequest();
      case 'SuggestTrendingHashtags':
        return $5.SuggestTrendingHashtagsRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'ListProducts':
        return listProducts(ctx, request as $5.ListProductsRequest);
      case 'SearchProducts':
        return searchProducts(ctx, request as $5.SearchProductsRequest);
      case 'SuggestProductNames':
        return suggestProductNames(
            ctx, request as $5.SuggestProductNamesRequest);
      case 'GetProduct':
        return getProduct(ctx, request as $5.GetProductRequest);
      case 'ListCategories':
        return listCategories(ctx, request as $5.ListCategoriesRequest);
      case 'SuggestHashtags':
        return suggestHashtags(ctx, request as $5.SuggestHashtagsRequest);
      case 'SuggestTrendingHashtags':
        return suggestTrendingHashtags(
            ctx, request as $5.SuggestTrendingHashtagsRequest);
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
  $async.Future<$5.ListMyProductsResponse> listMyProducts(
      $pb.ServerContext ctx, $5.ListMyProductsRequest request);
  $async.Future<$5.GetMyProductResponse> getMyProduct(
      $pb.ServerContext ctx, $5.GetMyProductRequest request);
  $async.Future<$5.CountMyProductsResponse> countMyProducts(
      $pb.ServerContext ctx, $5.CountMyProductsRequest request);
  $async.Future<$5.CreateProductResponse> createProduct(
      $pb.ServerContext ctx, $5.CreateProductRequest request);
  $async.Future<$5.UpdateProductResponse> updateProduct(
      $pb.ServerContext ctx, $5.UpdateProductRequest request);
  $async.Future<$5.SetProductStatusResponse> setProductStatus(
      $pb.ServerContext ctx, $5.SetProductStatusRequest request);
  $async.Future<$5.DeleteProductResponse> deleteProduct(
      $pb.ServerContext ctx, $5.DeleteProductRequest request);
  $async.Future<$5.AdjustStockResponse> adjustStock(
      $pb.ServerContext ctx, $5.AdjustStockRequest request);
  $async.Future<$5.SetStockResponse> setStock(
      $pb.ServerContext ctx, $5.SetStockRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'ListMyProducts':
        return $5.ListMyProductsRequest();
      case 'GetMyProduct':
        return $5.GetMyProductRequest();
      case 'CountMyProducts':
        return $5.CountMyProductsRequest();
      case 'CreateProduct':
        return $5.CreateProductRequest();
      case 'UpdateProduct':
        return $5.UpdateProductRequest();
      case 'SetProductStatus':
        return $5.SetProductStatusRequest();
      case 'DeleteProduct':
        return $5.DeleteProductRequest();
      case 'AdjustStock':
        return $5.AdjustStockRequest();
      case 'SetStock':
        return $5.SetStockRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'ListMyProducts':
        return listMyProducts(ctx, request as $5.ListMyProductsRequest);
      case 'GetMyProduct':
        return getMyProduct(ctx, request as $5.GetMyProductRequest);
      case 'CountMyProducts':
        return countMyProducts(ctx, request as $5.CountMyProductsRequest);
      case 'CreateProduct':
        return createProduct(ctx, request as $5.CreateProductRequest);
      case 'UpdateProduct':
        return updateProduct(ctx, request as $5.UpdateProductRequest);
      case 'SetProductStatus':
        return setProductStatus(ctx, request as $5.SetProductStatusRequest);
      case 'DeleteProduct':
        return deleteProduct(ctx, request as $5.DeleteProductRequest);
      case 'AdjustStock':
        return adjustStock(ctx, request as $5.AdjustStockRequest);
      case 'SetStock':
        return setStock(ctx, request as $5.SetStockRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json =>
      SellerCatalogAppServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => SellerCatalogAppServiceBase$messageJson;
}

abstract class CatalogDelegatedServiceBase extends $pb.GeneratedService {
  $async.Future<$5.ListStoreProductsResponse> listStoreProducts(
      $pb.ServerContext ctx, $5.ListStoreProductsRequest request);
  $async.Future<$5.GetStoreProductResponse> getStoreProduct(
      $pb.ServerContext ctx, $5.GetStoreProductRequest request);
  $async.Future<$5.CreateStoreProductResponse> createStoreProduct(
      $pb.ServerContext ctx, $5.CreateStoreProductRequest request);
  $async.Future<$5.UpdateStoreProductResponse> updateStoreProduct(
      $pb.ServerContext ctx, $5.UpdateStoreProductRequest request);
  $async.Future<$5.PublishStoreProductResponse> publishStoreProduct(
      $pb.ServerContext ctx, $5.PublishStoreProductRequest request);
  $async.Future<$5.UnpublishStoreProductResponse> unpublishStoreProduct(
      $pb.ServerContext ctx, $5.UnpublishStoreProductRequest request);
  $async.Future<$5.BatchUnpublishStoreProductsResponse>
      batchUnpublishStoreProducts(
          $pb.ServerContext ctx, $5.BatchUnpublishStoreProductsRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'ListStoreProducts':
        return $5.ListStoreProductsRequest();
      case 'GetStoreProduct':
        return $5.GetStoreProductRequest();
      case 'CreateStoreProduct':
        return $5.CreateStoreProductRequest();
      case 'UpdateStoreProduct':
        return $5.UpdateStoreProductRequest();
      case 'PublishStoreProduct':
        return $5.PublishStoreProductRequest();
      case 'UnpublishStoreProduct':
        return $5.UnpublishStoreProductRequest();
      case 'BatchUnpublishStoreProducts':
        return $5.BatchUnpublishStoreProductsRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'ListStoreProducts':
        return listStoreProducts(ctx, request as $5.ListStoreProductsRequest);
      case 'GetStoreProduct':
        return getStoreProduct(ctx, request as $5.GetStoreProductRequest);
      case 'CreateStoreProduct':
        return createStoreProduct(ctx, request as $5.CreateStoreProductRequest);
      case 'UpdateStoreProduct':
        return updateStoreProduct(ctx, request as $5.UpdateStoreProductRequest);
      case 'PublishStoreProduct':
        return publishStoreProduct(
            ctx, request as $5.PublishStoreProductRequest);
      case 'UnpublishStoreProduct':
        return unpublishStoreProduct(
            ctx, request as $5.UnpublishStoreProductRequest);
      case 'BatchUnpublishStoreProducts':
        return batchUnpublishStoreProducts(
            ctx, request as $5.BatchUnpublishStoreProductsRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json =>
      CatalogDelegatedServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => CatalogDelegatedServiceBase$messageJson;
}
