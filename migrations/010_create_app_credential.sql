CREATE TABLE IF NOT EXISTS `app_credential` (
  `app_cred_id` int NOT NULL AUTO_INCREMENT,
  `app_cred_user` varchar(256) NOT NULL,
  `app_cred_name` varchar(100) NOT NULL,
  `app_cred_enable` varchar(1) DEFAULT '1',
  `created_by` varchar(100) NOT NULL,
  `created_dt` datetime NOT NULL,
  PRIMARY KEY (`app_cred_id`),
  UNIQUE KEY `app_cred_id` (`app_cred_id`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;
