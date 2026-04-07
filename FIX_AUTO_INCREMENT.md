# Fix Broken Auto-Increment Columns

After running the database sync tool, some auto-increment columns may have been changed to nullable. Run these commands on the server to fix them.

## On Dev Server (Docker MySQL)

```bash
cd /opt/veristoretools3

# 1. Check which tables are broken
docker compose exec mysql mysql -u root -pveristoretools3 veristoretools3 -e "
SELECT TABLE_NAME, COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE
FROM INFORMATION_SCHEMA.COLUMNS
WHERE TABLE_SCHEMA = 'veristoretools3'
AND COLUMN_KEY IN ('PRI','UNI')
AND IS_NULLABLE = 'YES'
AND EXTRA = ''
AND COLUMN_TYPE = 'int'
ORDER BY TABLE_NAME;
"

# 2. Fix sync_terminal
docker compose exec mysql mysql -u root -pveristoretools3 veristoretools3 -e "
SET @max_id = (SELECT COALESCE(MAX(sync_term_id), 0) FROM sync_terminal WHERE sync_term_id IS NOT NULL);
UPDATE sync_terminal SET sync_term_id = (@max_id := @max_id + 1) WHERE sync_term_id IS NULL;
ALTER TABLE sync_terminal MODIFY sync_term_id int NOT NULL AUTO_INCREMENT;
"

# 3. Fix tms_report
docker compose exec mysql mysql -u root -pveristoretools3 veristoretools3 -e "
SET @max_id = (SELECT COALESCE(MAX(tms_rpt_id), 0) FROM tms_report WHERE tms_rpt_id IS NOT NULL);
UPDATE tms_report SET tms_rpt_id = (@max_id := @max_id + 1) WHERE tms_rpt_id IS NULL;
ALTER TABLE tms_report DROP PRIMARY KEY;
ALTER TABLE tms_report MODIFY tms_rpt_id int NOT NULL AUTO_INCREMENT, ADD PRIMARY KEY (tms_rpt_id);
"

# 4. Fix user (if broken)
docker compose exec mysql mysql -u root -pveristoretools3 veristoretools3 -e "
SET @max_id = (SELECT COALESCE(MAX(user_id), 0) FROM user WHERE user_id IS NOT NULL);
UPDATE user SET user_id = (@max_id := @max_id + 1) WHERE user_id IS NULL;
ALTER TABLE user MODIFY user_id int NOT NULL AUTO_INCREMENT;
"

# 5. Fix activity_log (if broken)
docker compose exec mysql mysql -u root -pveristoretools3 veristoretools3 -e "
SET @max_id = (SELECT COALESCE(MAX(act_log_id), 0) FROM activity_log WHERE act_log_id IS NOT NULL);
UPDATE activity_log SET act_log_id = (@max_id := @max_id + 1) WHERE act_log_id IS NULL;
ALTER TABLE activity_log MODIFY act_log_id int NOT NULL AUTO_INCREMENT;
"

# 6. Fix verification_report (if broken)
docker compose exec mysql mysql -u root -pveristoretools3 veristoretools3 -e "
SET @max_id = (SELECT COALESCE(MAX(vfi_rpt_id), 0) FROM verification_report WHERE vfi_rpt_id IS NOT NULL);
UPDATE verification_report SET vfi_rpt_id = (@max_id := @max_id + 1) WHERE vfi_rpt_id IS NULL;
ALTER TABLE verification_report MODIFY vfi_rpt_id int NOT NULL AUTO_INCREMENT;
"

# 7. Fix technician (if broken)
docker compose exec mysql mysql -u root -pveristoretools3 veristoretools3 -e "
SET @max_id = (SELECT COALESCE(MAX(tech_id), 0) FROM technician WHERE tech_id IS NOT NULL);
UPDATE technician SET tech_id = (@max_id := @max_id + 1) WHERE tech_id IS NULL;
ALTER TABLE technician MODIFY tech_id int NOT NULL AUTO_INCREMENT;
"

# 8. Fix app_activation (if broken)
docker compose exec mysql mysql -u root -pveristoretools3 veristoretools3 -e "
SET @max_id = (SELECT COALESCE(MAX(app_act_id), 0) FROM app_activation WHERE app_act_id IS NOT NULL);
UPDATE app_activation SET app_act_id = (@max_id := @max_id + 1) WHERE app_act_id IS NULL;
ALTER TABLE app_activation MODIFY app_act_id int NOT NULL AUTO_INCREMENT;
"

# 9. Fix app_credential (if broken)
docker compose exec mysql mysql -u root -pveristoretools3 veristoretools3 -e "
SET @max_id = (SELECT COALESCE(MAX(app_cred_id), 0) FROM app_credential WHERE app_cred_id IS NOT NULL);
UPDATE app_credential SET app_cred_id = (@max_id := @max_id + 1) WHERE app_cred_id IS NULL;
ALTER TABLE app_credential MODIFY app_cred_id int NOT NULL AUTO_INCREMENT;
"

# 10. Fix template_parameter (if broken)
docker compose exec mysql mysql -u root -pveristoretools3 veristoretools3 -e "
SET @max_id = (SELECT COALESCE(MAX(tparam_id), 0) FROM template_parameter WHERE tparam_id IS NOT NULL);
UPDATE template_parameter SET tparam_id = (@max_id := @max_id + 1) WHERE tparam_id IS NULL;
ALTER TABLE template_parameter MODIFY tparam_id int NOT NULL AUTO_INCREMENT;
"

# 11. Fix tms_login (if broken)
docker compose exec mysql mysql -u root -pveristoretools3 veristoretools3 -e "
SET @max_id = (SELECT COALESCE(MAX(tms_login_id), 0) FROM tms_login WHERE tms_login_id IS NOT NULL);
UPDATE tms_login SET tms_login_id = (@max_id := @max_id + 1) WHERE tms_login_id IS NULL;
ALTER TABLE tms_login MODIFY tms_login_id int NOT NULL AUTO_INCREMENT;
"

# 12. Fix export (if broken)
docker compose exec mysql mysql -u root -pveristoretools3 veristoretools3 -e "
SET @max_id = (SELECT COALESCE(MAX(exp_id), 0) FROM export WHERE exp_id IS NOT NULL);
UPDATE export SET exp_id = (@max_id := @max_id + 1) WHERE exp_id IS NULL;
ALTER TABLE export MODIFY exp_id int NOT NULL AUTO_INCREMENT;
"

# 13. Fix tid_note (if broken)
docker compose exec mysql mysql -u root -pveristoretools3 veristoretools3 -e "
SET @max_id = (SELECT COALESCE(MAX(tid_note_id), 0) FROM tid_note WHERE tid_note_id IS NOT NULL);
UPDATE tid_note SET tid_note_id = (@max_id := @max_id + 1) WHERE tid_note_id IS NULL;
ALTER TABLE tid_note MODIFY tid_note_id int NOT NULL AUTO_INCREMENT;
"

# 14. Fix faq (if broken)
docker compose exec mysql mysql -u root -pveristoretools3 veristoretools3 -e "
SET @max_id = (SELECT COALESCE(MAX(faq_id), 0) FROM faq WHERE faq_id IS NOT NULL);
UPDATE faq SET faq_id = (@max_id := @max_id + 1) WHERE faq_id IS NULL;
ALTER TABLE faq MODIFY faq_id int NOT NULL AUTO_INCREMENT;
"

# 15. Fix import (if broken)
docker compose exec mysql mysql -u root -pveristoretools3 veristoretools3 -e "
SET @max_id = (SELECT COALESCE(MAX(imp_id), 0) FROM \`import\` WHERE imp_id IS NOT NULL);
UPDATE \`import\` SET imp_id = (@max_id := @max_id + 1) WHERE imp_id IS NULL;
ALTER TABLE \`import\` MODIFY imp_id int NOT NULL AUTO_INCREMENT;
"

# 16. Verify all fixes
docker compose exec mysql mysql -u root -pveristoretools3 veristoretools3 -e "
SELECT TABLE_NAME, COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, EXTRA
FROM INFORMATION_SCHEMA.COLUMNS
WHERE TABLE_SCHEMA = 'veristoretools3'
AND COLUMN_KEY IN ('PRI','UNI')
AND COLUMN_TYPE = 'int'
ORDER BY TABLE_NAME;
"
```

## On Local (Bare Metal MySQL)

Same commands but without `docker compose exec mysql` prefix:

```bash
mysql -u root veristoretools3 -e "..."
```

## Notes

- These fixes are safe to run multiple times — if the table is already correct, the ALTER will be a no-op or fail harmlessly
- The `SET @max_id` + `UPDATE` pattern assigns sequential IDs to any NULL rows before restoring AUTO_INCREMENT
- Run step 1 first to see which tables actually need fixing
- Run step 16 last to verify — all `int` PRI/UNI columns should show `IS_NULLABLE=NO` and `EXTRA=auto_increment`
