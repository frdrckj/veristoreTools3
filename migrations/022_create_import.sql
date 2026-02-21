CREATE TABLE IF NOT EXISTS `import` (
  `imp_id` int NOT NULL AUTO_INCREMENT,
  `imp_code_id` varchar(5) NOT NULL,
  `imp_filename` varchar(50) NOT NULL,
  `imp_data` longblob DEFAULT NULL,
  `imp_cur_row` varchar(10) DEFAULT '0',
  `imp_total_row` varchar(10) DEFAULT '0',
  PRIMARY KEY (`imp_id`),
  UNIQUE KEY `imp_id` (`imp_id`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;
