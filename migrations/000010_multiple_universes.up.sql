ALTER TABLE universe DROP CONSTRAINT universe_pkey;
ALTER TABLE universe
add column universe_id uuid default uuid_generate_v4() primary key;
alter table universe drop column id;

alter table universe
rename to ticker;

alter table ticker
rename universe_id to ticker_id;

create type asset_universe_name as enum ('SPY_TOP_80');

create table asset_universe(
  asset_universe_id uuid default uuid_generate_v4() primary key,
  asset_universe_name asset_universe_name not null
);

create table asset_universe_ticker(
  asset_universe_ticker uuid default uuid_generate_v4() primary key,
  ticker_id uuid references ticker(ticker_id) not null,
  asset_universe_id uuid references asset_universe(asset_universe_id) not null
);
