CREATE TABLE IF NOT EXISTS `export` (
  `exp_id` int NOT NULL AUTO_INCREMENT,
  `exp_filename` varchar(50) NOT NULL,
  `exp_data` longblob,
  `exp_current` varchar(10) DEFAULT '0',
  `exp_total` varchar(10) DEFAULT '0',
  PRIMARY KEY (`exp_id`),
  UNIQUE KEY `exp_id` (`exp_id`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;
