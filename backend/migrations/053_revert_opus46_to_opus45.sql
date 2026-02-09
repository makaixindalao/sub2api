-- 将 Opus 4.6 映射目标回退到 Opus 4.5-thinking
-- 作者: mkx
-- 日期: 2026-02-10
--
-- 变更:
-- - claude-opus-4-6-thinking 映射到 claude-opus-4-5-thinking（原: 自身）
-- - claude-opus-4-6 映射到 claude-opus-4-5-thinking（原: claude-opus-4-6-thinking）
-- - claude-opus-4-5-thinking 保持映射到自身（原: claude-opus-4-6-thinking）
-- - claude-opus-4-5-20251101 映射到 claude-opus-4-5-thinking（原: claude-opus-4-6-thinking）

-- 1. claude-opus-4-6-thinking → claude-opus-4-5-thinking
UPDATE accounts
SET credentials = jsonb_set(
    credentials,
    '{model_mapping,claude-opus-4-6-thinking}',
    '"claude-opus-4-5-thinking"'::jsonb
)
WHERE platform = 'antigravity'
  AND deleted_at IS NULL
  AND credentials->'model_mapping' IS NOT NULL
  AND credentials->'model_mapping'->>'claude-opus-4-6-thinking' IS NOT NULL;

-- 2. claude-opus-4-6 → claude-opus-4-5-thinking
UPDATE accounts
SET credentials = jsonb_set(
    credentials,
    '{model_mapping,claude-opus-4-6}',
    '"claude-opus-4-5-thinking"'::jsonb
)
WHERE platform = 'antigravity'
  AND deleted_at IS NULL
  AND credentials->'model_mapping' IS NOT NULL
  AND credentials->'model_mapping'->>'claude-opus-4-6' IS NOT NULL;

-- 3. claude-opus-4-5-thinking → claude-opus-4-5-thinking（恢复自映射）
UPDATE accounts
SET credentials = jsonb_set(
    credentials,
    '{model_mapping,claude-opus-4-5-thinking}',
    '"claude-opus-4-5-thinking"'::jsonb
)
WHERE platform = 'antigravity'
  AND deleted_at IS NULL
  AND credentials->'model_mapping' IS NOT NULL
  AND credentials->'model_mapping'->>'claude-opus-4-5-thinking' IS NOT NULL;

-- 4. claude-opus-4-5-20251101 → claude-opus-4-5-thinking
UPDATE accounts
SET credentials = jsonb_set(
    credentials,
    '{model_mapping,claude-opus-4-5-20251101}',
    '"claude-opus-4-5-thinking"'::jsonb
)
WHERE platform = 'antigravity'
  AND deleted_at IS NULL
  AND credentials->'model_mapping' IS NOT NULL
  AND credentials->'model_mapping'->>'claude-opus-4-5-20251101' IS NOT NULL;
