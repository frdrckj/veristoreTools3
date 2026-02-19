CREATE TABLE IF NOT EXISTS `app_activation` (
  `app_act_id` int NOT NULL AUTO_INCREMENT,
  `app_act_csi` text NOT NULL,
  `app_act_tid` text NOT NULL,
  `app_act_mid` text NOT NULL,
  `app_act_model` text NOT NULL,
  `app_act_version` text NOT NULL,
  `app_act_engineer` text NOT NULL,
  `created_by` varchar(100) NOT NULL,
  `created_dt` datetime NOT NULL,
  PRIMARY KEY (`app_act_id`),
  UNIQUE KEY `app_act_id` (`app_act_id`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;
