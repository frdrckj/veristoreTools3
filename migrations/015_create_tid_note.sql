CREATE TABLE IF NOT EXISTS `tid_note` (
  `tid_note_id` int NOT NULL AUTO_INCREMENT,
  `tid_note_serial_num` text NOT NULL,
  `tid_note_data` text,
  `created_by` varchar(100) NOT NULL,
  `created_dt` datetime NOT NULL,
  PRIMARY KEY (`tid_note_id`),
  UNIQUE KEY `tid_note_id` (`tid_note_id`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;
