CREATE TABLE IF NOT EXISTS `template_parameter` (
  `tparam_id` int NOT NULL AUTO_INCREMENT,
  `tparam_title` varchar(75) NOT NULL,
  `tparam_index_title` text NOT NULL,
  `tparam_field` varchar(200) NOT NULL,
  `tparam_index` int NOT NULL,
  `tparam_type` varchar(1) NOT NULL,
  `tparam_operation` text NOT NULL,
  `tparam_length` text NOT NULL,
  `tparam_except` text,
  PRIMARY KEY (`tparam_id`),
  UNIQUE KEY `tparam_id` (`tparam_id`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;
