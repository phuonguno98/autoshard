-- MIT License
--
-- Copyright (c) 2026 phuonguno
--
-- Permission is hereby granted, free of charge, to any person obtaining a copy...

-- ============================================================================
-- Autoshard Registry Schema for MySQL/MariaDB
-- Project: Autoshard (github.com/phuonguno98/autoshard)
-- ============================================================================

-- The registry table is used for member discovery and consensus barrier.
-- It MUST be created before starting the application if using the MySQL adapter.

CREATE TABLE IF NOT EXISTS `autoshard_registry` (
    `member_id` VARCHAR(255) NOT NULL COMMENT 'Unique identifier for the cluster member',
    `perceived_version` INT NOT NULL DEFAULT 0 COMMENT 'The number of active members seen by this member during last sync',
    `last_heartbeat` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Timestamp of the last heartbeat, updated via DB NOW()',
    PRIMARY KEY (`member_id`),
    INDEX `idx_last_heartbeat` (`last_heartbeat`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
