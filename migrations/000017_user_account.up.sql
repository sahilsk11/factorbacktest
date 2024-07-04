create table user_account(
  user_account_id uuid default uuid_generate_v4() primary key,
  first_name text not null,
  last_name text not null,
  email text not null unique,
  created_at timestamp with time zone not null default now(),
  updated_at timestamp with time zone not null default now()
);

alter table api_request
add column user_account_id uuid references user_account(user_account_id);