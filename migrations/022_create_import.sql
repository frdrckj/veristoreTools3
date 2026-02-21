CREATE TABLE IF NOT EXISTS `import` (
  `imp_id` int NOT NULL AUTO_INCREMENT,
  `imp_code_id` varchar(10) NOT NULL DEFAULT 'CSI',
  `imp_filename` varchar(100) NOT NULL,
  `imp_current` varchar(10) DEFAULT '0',
  `imp_total` varchar(10) DEFAULT '0',
  PRIMARY KEY (`imp_id`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;
