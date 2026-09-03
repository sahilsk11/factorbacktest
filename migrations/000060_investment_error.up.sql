ALTER TABLE investment
ADD COLUMN error_at timestamp with time zone,
ADD COLUMN error_reason text;
