CREATE TABLE IF NOT EXISTS `terminal` (
  `term_id` int NOT NULL AUTO_INCREMENT,
  `term_device_id` text NOT NULL,
  `term_serial_num` text NOT NULL,
  `term_product_num` text NOT NULL,
  `term_model` text NOT NULL,
  `term_app_name` text NOT NULL,
  `term_app_version` text NOT NULL,
  `term_tms_create_operator` text NOT NULL,
  `term_tms_create_dt_operator` datetime NOT NULL,
  `term_tms_update_operator` text,
  `term_tms_update_dt_operator` datetime DEFAULT NULL,
  `created_by` varchar(100) NOT NULL,
  `created_dt` datetime NOT NULL,
  `updated_by` varchar(100) DEFAULT NULL,
  `updated_dt` datetime DEFAULT NULL,
  PRIMARY KEY (`term_id`),
  UNIQUE KEY `term_id` (`term_id`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;
