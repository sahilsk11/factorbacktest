CREATE TABLE interest_rate (
  interest_rate_id uuid default uuid_generate_v4() primary key,
  date date not null,
  duration_months int not null,
  interest_rate decimal not null
);

CREATE INDEX interest_rate_date on interest_rate(date);