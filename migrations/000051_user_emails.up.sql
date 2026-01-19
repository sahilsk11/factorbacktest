create type email_type as enum (
  'SAVED_STRATEGY_SUMMARY'
);

create type email_frequency as enum (
  'DAILY',
  'WEEKLY',
  'MONTHLY',
  'OFF'
);

create table email_preference(
  email_preference_id uuid primary key default uuid_generate_v4(),
  user_account_id uuid not null references user_account(user_account_id),
  email_type email_type not null,
  frequency email_frequency not null,
  created_at timestamp with time zone not null default now(),
  updated_at timestamp with time zone not null default now(),
  constraint unique_user_email_type unique(user_account_id, email_type)
);
