alter table user_account
alter column first_name drop not null;

alter table user_account
alter column last_name drop not null;

alter table user_account
alter column email drop not null;

alter table user_account
add column phone_number text unique;

create type user_account_provider_type as enum (
  'SUPABASE',
  'GOOGLE',
  'MANUAL'
);

alter table user_account
add column provider user_account_provider_type not null default 'GOOGLE';

alter table user_account
alter column provider drop default;

alter table user_account
add column provider_id text;

alter table user_account
add constraint unique_provider_id unique(provider, provider_id);
