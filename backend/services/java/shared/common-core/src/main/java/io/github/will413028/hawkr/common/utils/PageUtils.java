package io.github.will413028.hawkr.common.utils;

import com.baomidou.mybatisplus.core.metadata.IPage;
import lombok.Data;

import java.io.Serializable;
import java.util.List;

/**
 * Unified Pagination Response
 */
@Data
public class PageUtils implements Serializable {
    private static final long serialVersionUID = 1L;

    /**
     * Total records
     */
    private long totalCount;

    /**
     * Records per page
     */
    private long pageSize;

    /**
     * Total pages
     */
    private long totalPage;

    /**
     * Current page number
     */
    private long currPage;

    /**
     * Data list
     */
    private List<?> list;

    public PageUtils(IPage<?> page) {
        this.list = page.getRecords();
        this.totalCount = page.getTotal();
        this.pageSize = page.getSize();
        this.currPage = page.getCurrent();
        this.totalPage = page.getPages();
    }

    public PageUtils(List<?> list, long totalCount, long pageSize, long currPage) {
        this.list = list;
        this.totalCount = totalCount;
        this.pageSize = pageSize;
        this.currPage = currPage;
        this.totalPage = (totalCount + pageSize - 1) / pageSize;
    }
}
