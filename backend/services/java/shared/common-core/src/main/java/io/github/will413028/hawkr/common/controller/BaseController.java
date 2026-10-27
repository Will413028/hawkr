package io.github.will413028.hawkr.common.controller;

import com.baomidou.mybatisplus.core.conditions.query.QueryWrapper;
import com.baomidou.mybatisplus.core.metadata.IPage;
import com.baomidou.mybatisplus.extension.plugins.pagination.Page;
import com.baomidou.mybatisplus.extension.service.IService;
import io.github.will413028.hawkr.common.utils.PageUtils;
import io.github.will413028.hawkr.common.utils.R;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.web.bind.annotation.*;

import java.util.Arrays;
import java.util.Map;

/**
 * Base Controller with common CRUD operations
 * Reduces code duplication across 40+ controllers
 *
 * @param <S> Service type
 * @param <T> Entity type
 */
public abstract class BaseController<S extends IService<T>, T> {

    @Autowired
    protected S baseService;

    /**
     * Paginated list query
     */
    @GetMapping("/list")
    public R list(@RequestParam Map<String, Object> params) {
        IPage<T> page = getPage(params);
        QueryWrapper<T> wrapper = getQueryWrapper(params);
        IPage<T> result = baseService.page(page, wrapper);
        return R.ok().put("page", new PageUtils(result));
    }

    /**
     * Get entity by ID
     */
    @GetMapping("/info/{id}")
    public R info(@PathVariable("id") Long id) {
        T entity = baseService.getById(id);
        return R.ok().put("data", entity);
    }

    /**
     * Save new entity
     */
    @PostMapping("/save")
    public R save(@RequestBody T entity) {
        baseService.save(entity);
        return R.ok();
    }

    /**
     * Update entity
     */
    @PutMapping("/update")
    public R update(@RequestBody T entity) {
        baseService.updateById(entity);
        return R.ok();
    }

    /**
     * Delete entities by IDs
     */
    @DeleteMapping("/delete")
    public R delete(@RequestBody Long[] ids) {
        baseService.removeByIds(Arrays.asList(ids));
        return R.ok();
    }

    /**
     * Build pagination from request params
     */
    protected IPage<T> getPage(Map<String, Object> params) {
        long currPage = 1;
        long limit = 10;

        if (params.get("page") != null) {
            currPage = Long.parseLong(params.get("page").toString());
        }
        if (params.get("limit") != null) {
            limit = Long.parseLong(params.get("limit").toString());
        }

        return new Page<>(currPage, limit);
    }

    /**
     * Build query wrapper - override in subclass for custom filtering
     */
    protected QueryWrapper<T> getQueryWrapper(Map<String, Object> params) {
        return new QueryWrapper<>();
    }
}
