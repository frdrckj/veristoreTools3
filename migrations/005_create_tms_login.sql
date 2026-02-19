CREATE TABLE IF NOT EXISTS `tms_login` (
  `tms_login_id` int NOT NULL AUTO_INCREMENT,
  `tms_login_user` varchar(200) DEFAULT NULL,
  `tms_login_session` varchar(5120) DEFAULT NULL,
  `tms_login_scheduled` text,
  `tms_login_enable` varchar(1) DEFAULT '1',
  `created_by` varchar(100) NOT NULL,
  `created_dt` datetime NOT NULL,
  PRIMARY KEY (`tms_login_id`),
  UNIQUE KEY `tms_login_id` (`tms_login_id`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;
