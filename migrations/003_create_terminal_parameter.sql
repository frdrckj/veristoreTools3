CREATE TABLE IF NOT EXISTS `terminal_parameter` (
  `param_id` int NOT NULL AUTO_INCREMENT,
  `param_term_id` int NOT NULL,
  `param_host_name` text NOT NULL,
  `param_merchant_name` text NOT NULL,
  `param_tid` text NOT NULL,
  `param_mid` text NOT NULL,
  `param_address_1` text,
  `param_address_2` text,
  `param_address_3` text,
  `param_address_4` text,
  `param_address_5` text,
  `param_address_6` text,
  PRIMARY KEY (`param_id`),
  UNIQUE KEY `param_id` (`param_id`),
  KEY `fk_param_term_id_idx` (`param_term_id`),
  CONSTRAINT `fk_param_term_id` FOREIGN KEY (`param_term_id`) REFERENCES `terminal` (`term_id`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;
