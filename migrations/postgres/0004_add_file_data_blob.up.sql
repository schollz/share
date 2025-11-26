-- Add blob column to store encrypted file data directly in database
ALTER TABLE files ADD COLUMN file_data BYTEA;

-- Make file_path nullable since we'll be storing data in the database
ALTER TABLE files ALTER COLUMN file_path DROP NOT NULL;
