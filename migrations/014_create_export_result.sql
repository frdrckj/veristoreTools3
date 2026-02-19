CREATE TABLE IF NOT EXISTS `export_result` (
  `exp_res_id` int NOT NULL AUTO_INCREMENT,
  `exp_res_data` text NOT NULL,
  PRIMARY KEY (`exp_res_id`),
  UNIQUE KEY `exp_res_filename` (`exp_res_id`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;
