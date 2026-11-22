-- platform 的工作日行事曆（人事行政總處辦公日曆表）的 desired state：一年一次整年匯入，每一天一列。
-- days 的每一列是一個台北日曆日：holiday 是辦公日曆表的「是否放假」（2 = 放假，0 = 上班；補班的週六是 holiday = false），note 是備註。
-- 一年的列要不就整年都在、要不就一列都沒有（匯入以一個 transaction 寫滿全年，套件的讀取端依此判斷涵蓋）；沒有任何列的年份就是沒有涵蓋，
-- 不能從缺列推斷成上班日或假日。不建跨 schema 的 FK。這裡只放表、約束與索引；改完以 atlas migrate diff 產生 migration（見 atlas.hcl）。
--
-- public 不屬於本 schema：寫在這裡只是讓 Atlas 知道它存在。

CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA bizcalendar;

CREATE TABLE bizcalendar.days (
    day     DATE     PRIMARY KEY,
    holiday BOOLEAN  NOT NULL,
    note    TEXT     NOT NULL DEFAULT '',
    year    SMALLINT NOT NULL,
    CONSTRAINT days_year_check CHECK (year BETWEEN 1 AND 9999),
    CONSTRAINT days_year_day_check CHECK (year = EXTRACT(YEAR FROM day)),
    CONSTRAINT days_note_check CHECK (note !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$')
);
CREATE INDEX days_by_year ON bizcalendar.days (year);
