CREATE TABLE IF NOT EXISTS `hash` (
  `id` int NOT NULL,
  `hash` binary(1) NOT NULL,
  `timestamp` timestamp NOT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
