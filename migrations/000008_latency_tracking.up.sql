CREATE TABLE latency_tracking(
  latency_tracking_id uuid default uuid_generate_v4() primary key,
  processing_times jsonb not null,
  total_processing_ms bigint not null,
  request_id uuid references api_request(request_id)
);

alter table user_strategy add column request_id uuid references api_request(request_id);