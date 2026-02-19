CREATE TABLE IF NOT EXISTS `sync_terminal` (
  `sync_term_id` int NOT NULL AUTO_INCREMENT,
  `sync_term_creator_id` int NOT NULL,
  `sync_term_creator_name` text NOT NULL,
  `sync_term_created_time` datetime NOT NULL,
  `sync_term_status` varchar(1) NOT NULL DEFAULT '0',
  `sync_term_process` varchar(10) DEFAULT '0',
  `created_by` varchar(100) NOT NULL,
  `created_dt` datetime NOT NULL,
  PRIMARY KEY (`sync_term_creator_id`,`sync_term_created_time`),
  UNIQUE KEY `sync_term_id` (`sync_term_id`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;
