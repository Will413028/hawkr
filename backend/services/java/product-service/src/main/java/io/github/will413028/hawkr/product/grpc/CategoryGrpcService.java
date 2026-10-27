package io.github.will413028.hawkr.product.grpc;

import io.github.will413028.hawkr.common.grpc.GrpcAuthContext;
import io.github.will413028.hawkr.common.obs.grpc.GrpcErrorHandler;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.AccountType;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.proto.ProtoConversionUtils;
import io.github.will413028.hawkr.product.entity.CategoryEntity;
import io.github.will413028.hawkr.product.service.CategoryService;
import io.github.will413028.hawkr.product.service.ProductService;
import io.grpc.stub.StreamObserver;
import net.devh.boot.grpc.server.service.GrpcService;

import java.util.List;
import java.util.stream.Collectors;

/**
 * Category Service gRPC Handler（2026-04-25 政策大改造）
 *
 * <p>取消審核機制；新增 4 個 admin RPC（CreateForAdmin / Update / BatchSoftDelete / Restore）；
 * 移除 BatchUpdateCategoryStatus。
 */
@GrpcService
public class CategoryGrpcService extends CategoryServiceGrpc.CategoryServiceImplBase {

    private static final ObsLogger log = ObsLogger.forClass(CategoryGrpcService.class);

    private final CategoryService categoryService;
    private final ProductService productService;

    public CategoryGrpcService(CategoryService categoryService, ProductService productService) {
        this.categoryService = categoryService;
        this.productService = productService;
    }

    // ==================== 讀取 ====================

    @Override
    public void getCategoryTree(GetCategoryTreeRequest request, StreamObserver<GetCategoryTreeResponse> responseObserver) {
        try {
            // status="active" 或 includeDeleted=false → 僅 ACTIVE
            // includeDeleted=true → 全部含 INACTIVE（admin 用）
            boolean activeOnly = request.getIncludeDeleted()
                    ? false
                    : (request.getStatus().isEmpty() || "active".equalsIgnoreCase(request.getStatus()) || "9004".equals(request.getStatus()));

            List<CategoryEntity> tree = categoryService.getCategoryTree(activeOnly);

            GetCategoryTreeResponse.Builder responseBuilder = GetCategoryTreeResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage("查詢成功");
            for (CategoryEntity entity : tree) {
                responseBuilder.addCategories(convertToNode(entity));
            }
            responseObserver.onNext(responseBuilder.build());
            log.info("getCategoryTree.success",
                    "取得分類樹成功，root_count=" + tree.size() + "，activeOnly=" + activeOnly);
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetCategoryTreeResponse.newBuilder(), e, "CategoryGrpcService", "getCategoryTree");
            return;
        }
        responseObserver.onCompleted();
    }

    @Override
    public void getChildCategories(GetChildCategoriesRequest request, StreamObserver<GetChildCategoriesResponse> responseObserver) {
        try {
            long parentId = ProtoConversionUtils.parseLongOrDefault(request.getParentId(), 0L);
            if (parentId < 0) {
                responseObserver.onNext(GetChildCategoriesResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("parent_id 不可為負數")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            List<CategoryEntity> children = categoryService.getChildCategories(parentId);
            GetChildCategoriesResponse.Builder responseBuilder = GetChildCategoriesResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage("查詢成功");
            for (CategoryEntity entity : children) {
                responseBuilder.addCategories(convertToItem(entity));
            }
            responseObserver.onNext(responseBuilder.build());
            log.info("getChildCategories.success", "parent_id=" + parentId + "，count=" + children.size());
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetChildCategoriesResponse.newBuilder(), e, "CategoryGrpcService", "getChildCategories");
            return;
        }
        responseObserver.onCompleted();
    }

    // ==================== 寫入：賣家新增 ====================

    @Override
    public void createCategory(CreateCategoryRequest request, StreamObserver<CreateCategoryResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.SELLER, AccountType.ADMIN);

            Long parentId = parseParentIdOrNull(request.getParentId());
            CategoryEntity created = categoryService.createCategory(parentId, request.getName(), request.getCreatedBy());

            responseObserver.onNext(CreateCategoryResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage("success")
                    .setCategory(convertToItem(created))
                    .build());
            log.info("createCategory.success", "id=" + created.getId() + "，name=" + created.getName());
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, CreateCategoryResponse.newBuilder(), e, "CategoryGrpcService", "createCategory");
            return;
        }
        responseObserver.onCompleted();
    }

    // ==================== 寫入：Admin 新增（與賣家共用 service.createCategory） ====================

    @Override
    public void createCategoryForAdmin(CreateCategoryForAdminRequest request, StreamObserver<CreateCategoryForAdminResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);

            Long parentId = parseParentIdOrNull(request.getParentId());
            CategoryEntity created = categoryService.createCategory(parentId, request.getName(), request.getCreatedBy());

            responseObserver.onNext(CreateCategoryForAdminResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage("success")
                    .setCategory(convertToItem(created))
                    .build());
            log.info("createCategoryForAdmin.success", "id=" + created.getId() + "，name=" + created.getName());
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, CreateCategoryForAdminResponse.newBuilder(), e, "CategoryGrpcService", "createCategoryForAdmin");
            return;
        }
        responseObserver.onCompleted();
    }

    // ==================== 寫入：Admin 改名 ====================

    @Override
    public void updateCategoryForAdmin(UpdateCategoryForAdminRequest request, StreamObserver<UpdateCategoryForAdminResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);

            long categoryId = ProtoConversionUtils.parseLongOrDefault(request.getCategoryId(), 0L);
            if (categoryId <= 0) {
                responseObserver.onNext(UpdateCategoryForAdminResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("category_id 必須大於 0")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            CategoryEntity updated = categoryService.updateCategoryName(
                    categoryId, request.getName(), request.getUpdatedBy());

            responseObserver.onNext(UpdateCategoryForAdminResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage("success")
                    .setCategory(convertToItem(updated))
                    .build());
            log.info("updateCategoryForAdmin.success", "id=" + updated.getId() + "，new_name=" + updated.getName());
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, UpdateCategoryForAdminResponse.newBuilder(), e, "CategoryGrpcService", "updateCategoryForAdmin");
            return;
        }
        responseObserver.onCompleted();
    }

    // ==================== 寫入：Admin 批次軟刪除 ====================

    @Override
    public void batchSoftDeleteCategoryForAdmin(BatchSoftDeleteCategoryForAdminRequest request, StreamObserver<BatchSoftDeleteCategoryForAdminResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);

            List<Long> categoryIds = request.getCategoryIdsList().stream()
                    .map(Long::parseLong)
                    .collect(Collectors.toList());

            List<Long> deletedIds = categoryService.softDelete(
                    categoryIds,
                    request.getUpdatedBy(),
                    productService::hasProductsInCategory);

            responseObserver.onNext(BatchSoftDeleteCategoryForAdminResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage("success")
                    .addAllDeletedIds(deletedIds.stream().map(String::valueOf).collect(Collectors.toList()))
                    .build());
            log.info("batchSoftDeleteCategoryForAdmin.success", "deleted_count=" + deletedIds.size());
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, BatchSoftDeleteCategoryForAdminResponse.newBuilder(), e, "CategoryGrpcService", "batchSoftDeleteCategoryForAdmin");
            return;
        }
        responseObserver.onCompleted();
    }

    // ==================== 寫入：Admin 還原 ====================

    @Override
    public void restoreCategoryForAdmin(RestoreCategoryForAdminRequest request, StreamObserver<RestoreCategoryForAdminResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);

            long categoryId = ProtoConversionUtils.parseLongOrDefault(request.getCategoryId(), 0L);
            if (categoryId <= 0) {
                responseObserver.onNext(RestoreCategoryForAdminResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("category_id 必須大於 0")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            CategoryEntity restored = categoryService.restore(categoryId, request.getUpdatedBy());

            responseObserver.onNext(RestoreCategoryForAdminResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage("success")
                    .setCategory(convertToItem(restored))
                    .build());
            log.info("restoreCategoryForAdmin.success", "id=" + restored.getId());
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, RestoreCategoryForAdminResponse.newBuilder(), e, "CategoryGrpcService", "restoreCategoryForAdmin");
            return;
        }
        responseObserver.onCompleted();
    }

    // ==================== 私有轉換方法 ====================

    private CategoryNode convertToNode(CategoryEntity entity) {
        CategoryNode.Builder builder = CategoryNode.newBuilder()
                .setId(String.valueOf(entity.getId()))
                .setName(entity.getName() != null ? entity.getName() : "")
                .setParentId(entity.getParentId() != null ? String.valueOf(entity.getParentId()) : "0")
                .setParentName(entity.getParentName() != null ? entity.getParentName() : "")
                .setStatus(entity.getStatus() != null ? entity.getStatus() : "");
        if (entity.getDeletedAt() != null) {
            builder.setDeletedAt(DateTimeUtils.formatIso8601(entity.getDeletedAt()));
        }

        if (entity.getChildren() != null && !entity.getChildren().isEmpty()) {
            for (CategoryEntity child : entity.getChildren()) {
                builder.addChildren(convertToNode(child));
            }
        }
        return builder.build();
    }

    private CategoryItem convertToItem(CategoryEntity entity) {
        CategoryItem.Builder builder = CategoryItem.newBuilder()
                .setId(String.valueOf(entity.getId()))
                .setName(entity.getName() != null ? entity.getName() : "")
                .setParentId(entity.getParentId() != null ? String.valueOf(entity.getParentId()) : "0")
                .setParentName(entity.getParentName() != null ? entity.getParentName() : "")
                .setStatus(entity.getStatus() != null ? entity.getStatus() : "");
        if (entity.getDeletedAt() != null) {
            builder.setDeletedAt(DateTimeUtils.formatIso8601(entity.getDeletedAt()));
        }
        return builder.build();
    }

    /** 解析 parent_id：空字串或 "0" → null（表示新增 L1）；其他值 → Long。 */
    private Long parseParentIdOrNull(String parentIdStr) {
        if (parentIdStr == null || parentIdStr.isEmpty() || "0".equals(parentIdStr)) {
            return null;
        }
        return Long.parseLong(parentIdStr);
    }
}
