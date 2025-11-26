-- Remove blob column
ALTER TABLE files DROP COLUMN IF EXISTS file_data;

-- Restore file_path NOT NULL constraint
ALTER TABLE files ALTER COLUMN file_path SET NOT NULL;
