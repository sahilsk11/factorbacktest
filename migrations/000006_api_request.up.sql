CREATE TABLE api_request(
  request_id uuid default uuid_generate_v4() primary key,
  user_id uuid,
  ip_address text,
  method text not null,
  route text not null,
  request_body text,
  start_ts timestamp with time zone not null,
  duration_ms bigint,
  status_code int,
  response_body text
);
