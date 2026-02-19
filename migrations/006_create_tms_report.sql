CREATE TABLE IF NOT EXISTS `tms_report` (
  `tms_rpt_id` int NOT NULL AUTO_INCREMENT,
  `tms_rpt_name` varchar(255) NOT NULL,
  `tms_rpt_file` longblob,
  `tms_rpt_row` longtext,
  `tms_rpt_cur_page` varchar(10) DEFAULT '0',
  `tms_rpt_total_page` varchar(10) DEFAULT '0',
  PRIMARY KEY (`tms_rpt_name`),
  UNIQUE KEY `tms_rpt_id` (`tms_rpt_id`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;
