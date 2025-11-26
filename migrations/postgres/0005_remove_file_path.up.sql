-- Remove the file_path column as files are now stored in the database
ALTER TABLE files DROP COLUMN file_path;
