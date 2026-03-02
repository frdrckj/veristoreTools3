-- Add tms_username column to store the TMS login username per user.
ALTER TABLE `user` ADD COLUMN `tms_username` varchar(200) DEFAULT NULL;
