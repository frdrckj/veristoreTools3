CREATE TABLE IF NOT EXISTS `queue_log` (
  `create_time` varchar(50) NOT NULL,
  `exec_time` varchar(20) NOT NULL,
  `process_name` varchar(5) NOT NULL,
  `service_name` varchar(255) DEFAULT NULL,
  PRIMARY KEY (`create_time`,`process_name`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;
