-- Add blob column to store encrypted file data directly in database
ALTER TABLE files ADD COLUMN file_data BLOB;
