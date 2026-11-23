-- Short, human-readable memo handles: "<space handle_key>-<space_seq>", for example haymakr-12.
-- space.next_seq is a monotonic counter, so a deleted memo's number is never reused.
ALTER TABLE `space` ADD COLUMN `handle_key` VARCHAR(32) NULL, ADD COLUMN `next_seq` INT NOT NULL DEFAULT 1;
ALTER TABLE `memo` ADD COLUMN `space_seq` INT NULL;

-- A Space's uid is already a short identifier in the common case; otherwise fall back to s<id>.
-- Users rename the key afterwards.
UPDATE `space` SET `handle_key` = CASE WHEN CHAR_LENGTH(`uid`) <= 16 THEN LOWER(`uid`) ELSE CONCAT('s', `id`) END;

-- Number each Space's top-level memos by creation order. Comments are not numbered.
UPDATE `memo` JOIN (
  SELECT m.`id` AS `id`, ROW_NUMBER() OVER (PARTITION BY m.`space_id` ORDER BY m.`created_ts`, m.`id`) AS `n`
  FROM `memo` m
  WHERE m.`space_id` IS NOT NULL
    AND m.`id` NOT IN (SELECT `memo_id` FROM `memo_relation` WHERE `type` = 'COMMENT')
) AS r ON `memo`.`id` = r.`id`
SET `memo`.`space_seq` = r.`n`;

UPDATE `space` SET `next_seq` = COALESCE((SELECT MAX(`space_seq`) FROM `memo` WHERE `memo`.`space_id` = `space`.`id`), 0) + 1;

CREATE UNIQUE INDEX `idx_space_handle_key` ON `space`(`handle_key`);
CREATE UNIQUE INDEX `idx_memo_space_seq` ON `memo`(`space_id`, `space_seq`);
