CREATE TABLE IF NOT EXISTS `csi_request` (
  `req_id` int NOT NULL AUTO_INCREMENT,
  `req_device_id` varchar(20) NOT NULL,
  `req_vendor` varchar(50) NOT NULL,
  `req_model` varchar(50) NOT NULL,
  `req_merchant_id` varchar(50) NOT NULL,
  `req_group_ids` text,
  `req_sn` varchar(50) DEFAULT '',
  `req_app` varchar(50) DEFAULT '',
  `req_app_name` varchar(100) DEFAULT '',
  `req_move_conf` int DEFAULT 0,
  `req_status` varchar(20) NOT NULL DEFAULT 'PENDING',
  `created_by` varchar(100) NOT NULL,
  `created_dt` datetime NOT NULL,
  `approved_by` varchar(100) DEFAULT NULL,
  `approved_dt` datetime DEFAULT NULL,
  PRIMARY KEY (`req_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
