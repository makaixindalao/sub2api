-- 恢复 Opus 4.6-thinking 映射（对齐上游 DefaultAntigravityModelMapping）
-- 作者: mkx
-- 日期: 2026-02-10
--
-- 背景:
-- 此前 053_revert_opus46_to_opus45.sql 将 Opus 映射降级到 4.5-thinking，
-- 现在需要恢复为上游版本的 4.6-thinking 映射。
--
-- 变更:
-- - claude-opus-4-6-thinking 恢复为自映射（原: claude-opus-4-5-thinking）
-- - claude-opus-4-6 映射到 claude-opus-4-6-thinking（原: claude-opus-4-5-thinking）
-- - claude-opus-4-5-thinking 映射到 claude-opus-4-6-thinking（原: 自映射）
-- - claude-opus-4-5-20251101 映射到 claude-opus-4-6-thinking（原: claude-opus-4-5-thinking）

-- 1. claude-opus-4-6-thinking → claude-opus-4-6-thinking（恢复自映射）
UPDATE accounts
SET credentials = jsonb_set(
    credentials,
    '{model_mapping,claude-opus-4-6-thinking}',
    '"claude-opus-4-6-thinking"'::jsonb
)
WHERE platform = 'antigravity'
  AND deleted_at IS NULL
  AND credentials->'model_mapping' IS NOT NULL
  AND credentials->'model_mapping'->>'claude-opus-4-6-thinking' IS NOT NULL;

-- 2. claude-opus-4-6 → claude-opus-4-6-thinking
UPDATE accounts
SET credentials = jsonb_set(
    credentials,
    '{model_mapping,claude-opus-4-6}',
    '"claude-opus-4-6-thinking"'::jsonb
)
WHERE platform = 'antigravity'
  AND deleted_at IS NULL
  AND credentials->'model_mapping' IS NOT NULL
  AND credentials->'model_mapping'->>'claude-opus-4-6' IS NOT NULL;

-- 3. claude-opus-4-5-thinking → claude-opus-4-6-thinking（迁移旧模型）
UPDATE accounts
SET credentials = jsonb_set(
    credentials,
    '{model_mapping,claude-opus-4-5-thinking}',
    '"claude-opus-4-6-thinking"'::jsonb
)
WHERE platform = 'antigravity'
  AND deleted_at IS NULL
  AND credentials->'model_mapping' IS NOT NULL
  AND credentials->'model_mapping'->>'claude-opus-4-5-thinking' IS NOT NULL;

-- 4. claude-opus-4-5-20251101 → claude-opus-4-6-thinking
UPDATE accounts
SET credentials = jsonb_set(
    credentials,
    '{model_mapping,claude-opus-4-5-20251101}',
    '"claude-opus-4-6-thinking"'::jsonb
)
WHERE platform = 'antigravity'
  AND deleted_at IS NULL
  AND credentials->'model_mapping' IS NOT NULL
  AND credentials->'model_mapping'->>'claude-opus-4-5-20251101' IS NOT NULL;
