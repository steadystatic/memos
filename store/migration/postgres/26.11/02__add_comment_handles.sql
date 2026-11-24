-- Short, human-readable comment handles: "<space handle_key>-<parent space_seq>-<comment_seq>",
-- for example haymakr-12-3. memo.next_comment_seq is a monotonic counter per parent memo, so a
-- deleted comment's number is never reused.
ALTER TABLE memo ADD COLUMN comment_seq INTEGER, ADD COLUMN next_comment_seq INTEGER NOT NULL DEFAULT 1;

-- Number each parent memo's existing comments by creation order.
UPDATE memo SET comment_seq = ranked.n
FROM (
  SELECT r.memo_id AS id,
         ROW_NUMBER() OVER (PARTITION BY r.parent_id ORDER BY m.created_ts, m.id) AS n
  FROM (
    SELECT memo_id, MIN(related_memo_id) AS parent_id
    FROM memo_relation
    WHERE type = 'COMMENT'
    GROUP BY memo_id
  ) AS r
  JOIN memo m ON m.id = r.memo_id
) AS ranked
WHERE memo.id = ranked.id;

UPDATE memo SET next_comment_seq = counts.max_seq + 1
FROM (
  SELECT r.parent_id, MAX(c.comment_seq) AS max_seq
  FROM (
    SELECT memo_id, MIN(related_memo_id) AS parent_id
    FROM memo_relation
    WHERE type = 'COMMENT'
    GROUP BY memo_id
  ) AS r
  JOIN memo c ON c.id = r.memo_id
  WHERE c.comment_seq IS NOT NULL
  GROUP BY r.parent_id
) AS counts
WHERE memo.id = counts.parent_id;
