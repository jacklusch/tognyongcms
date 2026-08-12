-- Dulizhan CMS — 示例数据翻译组迁移
-- 背景：阶段 3 前的 seed 把 hello-zh 与 hello-en 建为两条独立内容（content_id 不同），
-- 导致编辑 zh 内容时切到 en Tab 不显示英文（en 不属于同一翻译组）。
-- 修复后的 seed 会让 zh/en 共享 content_id（翻译组）；本脚本把既有独立数据归组。
--
-- 用法（先备份 dulizhan.db）：
--   sqlite3 dulizhan.db < scripts/migrate-seed-translation.sql

-- 把 hello-en 归入 hello-zh 的翻译组（content_id 改为 zh 的组键）。
-- 幂等：仅当两者当前不在同组时更新。
UPDATE content
SET content_id = (
  SELECT content_id FROM content WHERE lang = 'zh' AND slug = 'hello-zh' LIMIT 1
)
WHERE lang = 'en'
  AND slug = 'hello-en'
  AND content_id != (
    SELECT content_id FROM content WHERE lang = 'zh' AND slug = 'hello-zh' LIMIT 1
  );

-- 验证：zh/en 现在应共享 content_id
-- SELECT content_id, lang, slug FROM content ORDER BY lang;
