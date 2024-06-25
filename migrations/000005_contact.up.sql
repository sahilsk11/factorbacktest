CREATE TABLE contact_message (
  message_id uuid DEFAULT uuid_generate_v4() PRIMARY KEY,
  user_id uuid,
  reply_email text,
  message_content text not null,
  created_at timestamp with time zone not null
)