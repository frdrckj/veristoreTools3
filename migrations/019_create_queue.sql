CREATE TABLE IF NOT EXISTS `queue` (
  `id` int NOT NULL AUTO_INCREMENT,
  `channel` varchar(255) CHARACTER SET utf8mb3 COLLATE utf8mb3_bin NOT NULL,
  `job` longblob NOT NULL,
  `pushed_at` int NOT NULL,
  `ttr` int NOT NULL,
  `delay` int NOT NULL DEFAULT '0',
  `priority` int unsigned NOT NULL DEFAULT '1024',
  `reserved_at` int DEFAULT NULL,
  `attempt` int DEFAULT NULL,
  `done_at` int DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `channel` (`channel`),
  KEY `reserved_at` (`reserved_at`),
  KEY `priority` (`priority`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_bin;
