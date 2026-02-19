CREATE TABLE IF NOT EXISTS `faq` (
  `faq_id` int NOT NULL AUTO_INCREMENT,
  `faq_parent` int DEFAULT NULL,
  `faq_seq` int NOT NULL,
  `faq_privileges` varchar(60) NOT NULL,
  `faq_name` text NOT NULL,
  `faq_link` text,
  PRIMARY KEY (`faq_id`),
  UNIQUE KEY `faq_id` (`faq_id`),
  KEY `fk_faq_parent_id_idx` (`faq_parent`),
  CONSTRAINT `fk_faq_parent_id` FOREIGN KEY (`faq_parent`) REFERENCES `faq` (`faq_id`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;
