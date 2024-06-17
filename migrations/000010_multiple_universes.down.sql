drop table asset_universe_ticker;

drop table asset_universe;

drop type asset_universe_name;

alter table ticker
rename ticker_id to universe_id;

alter table ticker
rename to universe;

-- dirty, sorry
alter table universe
add column serial not null;