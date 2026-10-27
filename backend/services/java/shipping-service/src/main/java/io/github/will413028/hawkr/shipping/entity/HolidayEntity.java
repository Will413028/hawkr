package io.github.will413028.hawkr.shipping.entity;

import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import lombok.Data;

import java.time.LocalDate;
import java.time.LocalDateTime;

@Data
@TableName("holidays")
public class HolidayEntity {

    @TableId(type = IdType.AUTO)
    private Long id;

    private LocalDate holidayDate;
    private String name;
    private Short year;
    private LocalDateTime createdAt;
}
