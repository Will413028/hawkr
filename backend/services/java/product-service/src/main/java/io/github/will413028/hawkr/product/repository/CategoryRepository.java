package io.github.will413028.hawkr.product.repository;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.common.utils.code.CommonStatus;
import io.github.will413028.hawkr.product.entity.CategoryEntity;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;
import org.apache.ibatis.annotations.Update;

import java.time.LocalDateTime;
import java.util.List;
import java.util.Optional;

/**
 * 分類 Repository
 * 繼承 MyBatis-Plus BaseMapper
 */
@Mapper
public interface CategoryRepository extends BaseMapper<CategoryEntity> {

    /**
     * 根據 ID 查詢分類
     */
    @Select("SELECT * FROM category WHERE id = #{id}")
    Optional<CategoryEntity> findById(@Param("id") Long id);

    /**
     * 查詢所有分類（含 INACTIVE 軟刪除）
     */
    @Select("SELECT * FROM category ORDER BY id")
    List<CategoryEntity> findAll();

    /**
     * 根據狀態查詢分類
     */
    @Select("SELECT * FROM category WHERE status = #{status} ORDER BY id")
    List<CategoryEntity> findByStatus(@Param("status") String status);

    /**
     * 根據父分類 ID 查詢子分類（不限狀態）
     */
    @Select("SELECT * FROM category WHERE parent_id = #{parentId} ORDER BY id")
    List<CategoryEntity> findByParentId(@Param("parentId") Long parentId);

    /**
     * 查詢頂層分類（parent_id IS NULL，不限狀態）
     */
    @Select("SELECT * FROM category WHERE parent_id IS NULL ORDER BY id")
    List<CategoryEntity> findTopCategories();

    /**
     * 同父同名查詢（不限 status，用於自動套用判斷與軟刪除衝突檢查）
     */
    @Select("""
        SELECT * FROM category
        WHERE name = #{name}
          AND COALESCE(parent_id, 0) = COALESCE(CAST(#{parentId} AS BIGINT), 0)
        LIMIT 1
        """)
    Optional<CategoryEntity> findByNameAndParentId(
            @Param("name") String name,
            @Param("parentId") Long parentId);

    /**
     * 根據名稱查詢分類（限定 ACTIVE，避免撞到軟刪除分類）
     */
    @Select("SELECT * FROM category WHERE name = #{name} AND status = '9004' LIMIT 1")
    Optional<CategoryEntity> findActiveByName(@Param("name") String name);

    /**
     * 是否有 ACTIVE 子分類（軟刪除前置驗證）
     */
    @Select("""
        SELECT COUNT(*) > 0 FROM category
        WHERE parent_id = #{parentId} AND status = '9004'
        """)
    boolean hasActiveChildren(@Param("parentId") Long parentId);

    /**
     * 軟刪除（status=9003，寫 deleted_at；MetaObjectHandler 會自動更新 updated_at）
     */
    default int softDelete(Long id, LocalDateTime deletedAt, String updatedBy) {
        return softDeleteByStatus(id, deletedAt, updatedBy,
                CommonStatus.INACTIVE.value(), CommonStatus.ACTIVE.value());
    }

    @Update("""
        UPDATE category
        SET status = #{inactive},
            deleted_at = #{deletedAt},
            updated_by = #{updatedBy},
            updated_at = #{deletedAt}
        WHERE id = #{id} AND status = #{active}
        """)
    int softDeleteByStatus(
            @Param("id") Long id,
            @Param("deletedAt") LocalDateTime deletedAt,
            @Param("updatedBy") String updatedBy,
            @Param("inactive") String inactive,
            @Param("active") String active);

    /**
     * 還原軟刪除分類（status: 9003 → 9004，清空 deleted_at）
     */
    default int restore(Long id, LocalDateTime updatedAt, String updatedBy) {
        return restoreByStatus(id, updatedAt, updatedBy,
                CommonStatus.ACTIVE.value(), CommonStatus.INACTIVE.value());
    }

    @Update("""
        UPDATE category
        SET status = #{active},
            deleted_at = NULL,
            updated_by = #{updatedBy},
            updated_at = #{updatedAt}
        WHERE id = #{id} AND status = #{inactive}
        """)
    int restoreByStatus(
            @Param("id") Long id,
            @Param("updatedAt") LocalDateTime updatedAt,
            @Param("updatedBy") String updatedBy,
            @Param("active") String active,
            @Param("inactive") String inactive);
}
