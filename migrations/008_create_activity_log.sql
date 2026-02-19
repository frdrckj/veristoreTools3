CREATE TABLE IF NOT EXISTS `activity_log` (
  `act_log_id` int NOT NULL AUTO_INCREMENT,
  `act_log_action` varchar(100) NOT NULL,
  `act_log_detail` text,
  `created_by` varchar(100) NOT NULL,
  `created_dt` datetime NOT NULL,
  PRIMARY KEY (`act_log_id`),
  UNIQUE KEY `act_log_id` (`act_log_id`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;
