CREATE TABLE IF NOT EXISTS `technician` (
  `tech_id` int NOT NULL AUTO_INCREMENT,
  `tech_name` varchar(150) NOT NULL,
  `tech_nip` varchar(50) NOT NULL,
  `tech_number` varchar(100) NOT NULL,
  `tech_address` text NOT NULL,
  `tech_company` varchar(100) NOT NULL,
  `tech_sercive_point` varchar(100) NOT NULL,
  `tech_phone` varchar(15) NOT NULL,
  `tech_gender` varchar(1) NOT NULL,
  `tech_status` varchar(1) NOT NULL DEFAULT '1',
  `created_by` varchar(100) NOT NULL,
  `created_dt` datetime NOT NULL,
  `updated_by` varchar(100) DEFAULT NULL,
  `updated_dt` datetime DEFAULT NULL,
  PRIMARY KEY (`tech_id`),
  UNIQUE KEY `tech_id` (`tech_id`),
  UNIQUE KEY `tech_number` (`tech_number`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;
