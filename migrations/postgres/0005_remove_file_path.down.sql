-- Restore file_path column (will be empty for database-stored files)
ALTER TABLE files ADD COLUMN file_path TEXT;
